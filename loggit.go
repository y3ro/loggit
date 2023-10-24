package main

import (
  "bufio"
  "encoding/json"
  "flag"
  "fmt"
  "io"
  "log"
  "math/rand"
  "os"
  "os/exec"
  "path/filepath"
  "regexp"
  "runtime"
  "strconv"
  "strings"
  "time"
)

const (
  defaultBumpVersionMsg = "Bump version"
  defaultVersionRegexStr = "\\d+\\.\\d+\\.\\d+"
  defaultLogGitTrailer = "log:"
  defaultChangelogRelativePath = "CHANGELOG.md"
  defaultVersionHeader = "# Version "
  configFileName = "loggit.json"
)

var (
  config Config
)

type Config struct {
  BumpVersionMsg string
  VersionRegexp *regexp.Regexp
  LogGitTrailer string
  ChangelogRelativePath string
  VersionHeader string
}

func getHomePath() string {
	var homePath string
	if runtime.GOOS == "windows" {
		homePath = "HOMEPATH"
	} else {
		homePath = "HOME"
	}

	return filepath.Join(os.Getenv(homePath), ".config")
}

func readConfig() error {
	configDir := getHomePath()
	err := os.MkdirAll(configDir, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("Error mkdir'ing in readConfig: %w", err)
		return nil
	}

	configFilePath := filepath.Join(configDir, configFileName)
	configFile, err := os.Open(configFilePath)
	if err == nil {
    defer configFile.Close()

    configBytes, err := io.ReadAll(configFile)
    if err != nil {
      err = fmt.Errorf("Error reading config file in readConfig: %w", err)
      return err
    }

    err = json.Unmarshal(configBytes, &config)
    if err != nil {
      err = fmt.Errorf("Error unmarshalling in readConfig: %w", err)
      return err
    }
	}
 
	if config.BumpVersionMsg == "" {
    config.BumpVersionMsg = defaultBumpVersionMsg
	}
	if config.LogGitTrailer == "" {
    config.LogGitTrailer = defaultLogGitTrailer
	}
	if config.VersionHeader == "" {
    config.VersionHeader = defaultVersionHeader
	}
	if config.VersionRegexp == nil {
    config.VersionRegexp = regexp.MustCompile(defaultVersionRegexStr)
	}
	if config.ChangelogRelativePath == "" {
    config.ChangelogRelativePath = defaultChangelogRelativePath
	}

	return nil
}

func getNewVersion(commitMsgPath string) (string, error) {
  commitMsgFile, err := os.Open(commitMsgPath)
  if err != nil {
    log.Fatalln("Could not open the commit message file")
  }
 
	commitMsgBytes, err := io.ReadAll(commitMsgFile)
	if err != nil {
    log.Fatalln("Could not read the commit message")
	}

  commitMsg := string(commitMsgBytes)
  if !strings.HasPrefix(commitMsg, config.BumpVersionMsg) {
    return "", fmt.Errorf("No new version in this commit")
  }

  versionMatch := config.VersionRegexp.Find(commitMsgBytes)  
  if len(versionMatch) == 0 {
    log.Fatalln("Invalid format for new version in this commit")
  }

  return string(versionMatch), nil
}

func getPrevBumpCommitHash() string {
  grepArg := "--grep=" + config.BumpVersionMsg
  formatArg := "--pretty=format:%H"

  cmd := exec.Command("git", "log", grepArg, "-n", "1", formatArg)
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Could not read the previous bump-commit hash")
  }

  outStr := string(out)
  outLines := strings.Split(outStr, "\n")
  if len(outLines) == 1 && outLines[0] == "" {
    return ""
  }

  return outLines[0]
}

func getCurrentGitBranch() string {
  cmd := exec.Command("git", "branch", "--show-current")
  output, err := cmd.Output()
  if err != nil {
    log.Fatalln("Could not get the current Git branch")
  }
  return strings.TrimSpace(string(output))
}

func getFirstBranchCommitHash(branchName string) string {
  interval := "master.." + branchName
  formatArg := "--pretty=format:%H"
  cmd := exec.Command("git", "log", interval, formatArg, "|", "tail", "-1")
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Could not read the previous bump-commit hash")
  }

  return strings.TrimSpace(string(out))
}

func collectLogMsgs(prevCommitHash string) []string {
  lowerLimit := ""
  if len(prevCommitHash) > 0 {
    lowerLimit = prevCommitHash + ".."
  }
  commitsInterval := lowerLimit + "HEAD"
  grepArg := "--grep=" + config.LogGitTrailer
  formatArg := "--pretty=format:%b"

  cmd := exec.Command("git", "log", commitsInterval, grepArg, formatArg)
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Failed to collect log messages")
  }

  outStr := strings.TrimSpace(string(out))
  outLines := strings.Split(outStr, "\n")
  gitTrailerLen := len(config.LogGitTrailer)

  var logMsgs []string
  for i := 0; i < len(outLines); i++ {
    line := outLines[i]
    if strings.HasPrefix(line, config.LogGitTrailer) {
      logMsgs = append(logMsgs, line[gitTrailerLen:])
    }
  }

  return logMsgs
}

func getVersionLogHeader(version string) string {
  today := time.Now().Format("2006-01-02")
  return config.VersionHeader + version + " - " + today
}

func writeTempLogFile(tempLogFile *os.File, newVersionHeader string, 
    newLogLines []string) {
  logFile, err := os.Open(config.ChangelogRelativePath)
  if err != nil {
    logFile, err = os.Create(config.ChangelogRelativePath)
    if err != nil {
      log.Fatalln("Could not open nor create the changelog file")
    }
  }
  defer logFile.Close()

  _, err = tempLogFile.WriteString(newVersionHeader + "\n")
  if err != nil {
    log.Fatal(err)
  }

  for i := 0; i < len(newLogLines); i++ {
    _, err = tempLogFile.WriteString("* " + newLogLines[i] + "\n")
    if err != nil {
      log.Fatal(err)
    }
  }

  _, err = tempLogFile.WriteString("\n")
  if err != nil {
    log.Fatal(err)
  }

  scanner := bufio.NewScanner(logFile)
  for scanner.Scan() {
    _, err = tempLogFile.WriteString(scanner.Text() + "\n")
    if err != nil {
      log.Fatal(err)
    }
  }

  if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }
  tempLogFile.Sync()
}

func CreateNewVersionGitTag(newVersion string) {
  cmd := exec.Command("git", "tag", newVersion)
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Could not create a new tag")
  }
  if len(out) > 0 {
    log.Fatal(string(out))
  }
}

func AppendToChangelog(commitMsgPath string, createTag bool) {
  newVersion, err := getNewVersion(commitMsgPath)
  if err != nil {
    fmt.Println(err)
    os.Exit(0)
  }
  newVersionHeader := getVersionLogHeader(newVersion)

  prevHash := getPrevBumpCommitHash()
  logMsgs := collectLogMsgs(prevHash)

  randNumber := strconv.Itoa(rand.Int())
  tempFile, err := os.Create("loggit-" + randNumber)
  if err != nil {
    log.Fatalln("Could not create a temporary file")
  }
  defer tempFile.Close()

  writeTempLogFile(tempFile, newVersionHeader, logMsgs)

  err = os.Rename(tempFile.Name(), config.ChangelogRelativePath)
  if err != nil {
    log.Fatal(err)
  }

  if createTag {
    CreateNewVersionGitTag(newVersion)
  }
}

func WriteBranchChangelog() {
  currentBranch := getCurrentGitBranch()
  prevHash := getFirstBranchCommitHash(currentBranch)
  logMsgs := collectLogMsgs(prevHash)

  _, logFileName := filepath.Split(config.ChangelogRelativePath)
  if len(logFileName) == 0 {
    _, logFileName = filepath.Split(defaultChangelogRelativePath)
  }

  branchLogFile, err := os.Create(currentBranch + "-" + logFileName)
  if err != nil {
    log.Fatalln("Could not create branch changelog")
  }
  defer branchLogFile.Close()

  for i := 0; i < len(logMsgs); i++ {
    line := "* " + logMsgs[i] + "\n"
    fmt.Print(line)
    _, err = branchLogFile.WriteString(line)
    if err != nil {
      log.Fatal(err)
    }
  }
}

func parseCliArgsAndRun() {
  branchModePtr := flag.Bool("branch", false, "Use all commits from the current branch")
  avoidCreatingTagPtr := flag.Bool("no-tag", false, "Do not create a tag on bump version")
  flag.Parse()

  if len(os.Args) == 1 {
    log.Fatal("Please provide the commit message file or specify branch mode with `-branch`")
  }

  if *branchModePtr {
    WriteBranchChangelog()
    return
  }
  
  AppendToChangelog(os.Args[1], !*avoidCreatingTagPtr)

}

func main() {
  err := readConfig()
  if err != nil {
    log.Fatal(err)
  }

  parseCliArgsAndRun()
}
