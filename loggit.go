package main

import (
	"bufio"
	"encoding/json"
	"errors"
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
  defaultUseCommitTitleMsg = "%s"
  defaultChangelogRelativePath = "CHANGELOG.md"
  defaultVersionHeader = "# Version "
  defaultMasterBranchName = "master"
  configFileName = "loggit.json"
)

var (
  config Config
  defaultAlsoTag = true
)

type Config struct {
  BumpVersionMsg string
  VersionRegexp *regexp.Regexp // TODO: fix
  LogGitTrailer string
  UseCommitTitleMsg string
  ChangelogRelativePath string
  VersionHeader string
  MasterBranchName string
  AlsoTag *bool
}

func getConfigDir() string {
	var homePath string
	if runtime.GOOS == "windows" {
		homePath = "HOMEPATH"
	} else {
		homePath = "HOME"
	}

	return filepath.Join(os.Getenv(homePath), ".config")
}

// TODO: reduce complexity
func readConfig(configPath string) {
  var configFile *os.File
  var err error
  if len(configPath) == 0 {
    cmd := exec.Command("git", "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    if err == nil {
      repoRoot := strings.TrimSpace(string(out))
      configPath = filepath.Join(repoRoot, configFileName) 
      configFile, err = os.Open(configPath)
    }

    if err != nil {
      configDir := getConfigDir()
      err := os.MkdirAll(configDir, os.ModePerm)
      if err != nil {
	log.Fatalf("Error mkdir'ing in readConfig: %s\n", err)
      }

      configPath = filepath.Join(configDir, configFileName)
      configFile, err = os.Open(configPath)
    }
  }

  if err == nil {
    defer configFile.Close()

    configBytes, err := io.ReadAll(configFile)
    if err != nil {
      log.Fatalf("Error reading config file in readConfig: %s\n", err)
    }

    err = json.Unmarshal(configBytes, &config)
    if err != nil {
      log.Fatalf("Error unmarshalling in readConfig: %s\n", err)
    }
  }

  if config.BumpVersionMsg == "" {
    config.BumpVersionMsg = defaultBumpVersionMsg
  }
  if config.LogGitTrailer == "" {
    config.LogGitTrailer = defaultLogGitTrailer
  }
  if config.UseCommitTitleMsg == "" {
    config.UseCommitTitleMsg = defaultUseCommitTitleMsg
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
  if config.MasterBranchName == "" {
    config.MasterBranchName = defaultMasterBranchName
  }
  if config.AlsoTag == nil {
    config.AlsoTag = &defaultAlsoTag
  }
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
  var exitError *exec.ExitError
  if errors.As(err, &exitError) {
    log.Fatalf("Could not read the previous bump-commit hash: %s\n", string(exitError.Stderr))
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
  var exitError *exec.ExitError
  if errors.As(err, &exitError) {
    log.Fatalf("Could not get the current Git branch: %s\n", string(exitError.Stderr))
  }
  return strings.TrimSpace(string(output))
}

func getFirstBranchCommitHash(branchName string) string {
  interval := config.MasterBranchName + "~.." + branchName
  formatArg := "--pretty=format:%H"
  cmd := exec.Command("git", "log", interval, formatArg)
  out, err := cmd.Output()
  var exitError *exec.ExitError
  if errors.As(err, &exitError) {
    log.Fatalf("Could not read the previous bump-commit hash: %s\n", string(exitError.Stderr))
  }

  outStr := string(out)
  outLines := strings.Split(outStr, "\n") 
  nLines := len(outLines)
  if nLines == 0 || outLines[nLines - 1] == "" {
    log.Fatalln("Could not read the first commit hash of the current branch")
  }

  return outLines[nLines - 1]
}

// TODO: reduce complexity
func collectLogMsgs(prevCommitHash string) []string {
  lowerLimit := ""
  if len(prevCommitHash) > 0 {
    lowerLimit = prevCommitHash + ".."
  }
  commitsInterval := lowerLimit + "HEAD"
  grepArg := "--grep=" + config.LogGitTrailer
  formatSubjectArg := "--pretty=format:%s"
  formatBodyArg := "--pretty=format:%b"

  cmd := exec.Command("git", "log", commitsInterval, grepArg, formatBodyArg)
  bodyOut, err := cmd.Output()
  var exitError *exec.ExitError
  if errors.As(err, &exitError) {
    log.Fatalf("Failed to collect log messages (bodies): %s\n", string(exitError.Stderr))
  }

  cmd = exec.Command("git", "log", commitsInterval, grepArg, formatSubjectArg)
  subjectOut, err := cmd.Output()
  if errors.As(err, &exitError) {
    log.Fatalf("Failed to collect log messages (subjects): %s\n", string(exitError.Stderr))
  }

  gitTrailerLen := len(config.LogGitTrailer)
  outStr := strings.TrimSpace(string(bodyOut))
  outLineAllBodies := strings.Split(outStr, "\n")
  var outLineBodies []string
  for i := 0; i < len(outLineAllBodies); i++ {
    body := outLineAllBodies[i]
    if len(body) > 0 {
      outLineBodies = append(outLineBodies, body)
    }
  }

  outStr = strings.TrimSpace(string(subjectOut))
  outLineSubjects := strings.Split(outStr, "\n")

  if len(outLineBodies) != len(outLineSubjects) {
    log.Fatalln("Different number of commit bodies and subjects")
  }

  var logMsgs []string
  for i := 0; i < len(outLineBodies); i++ {
    bodyMsg := outLineBodies[i]
    subjectMsg := outLineSubjects[i]
    if strings.HasPrefix(bodyMsg, config.LogGitTrailer) {
      logMsg := strings.TrimSpace(bodyMsg[gitTrailerLen:])
      if logMsg == config.UseCommitTitleMsg {
	logMsgs = append(logMsgs, subjectMsg)
      } else {
	logMsgs = append(logMsgs, logMsg)
      }
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
  var exitError *exec.ExitError
  if errors.As(err, &exitError) {
    log.Fatalf("Could not create a new tag: %s\n", string(exitError.Stderr))
  }
  if len(out) > 0 {
    log.Fatal(string(out))
  }
}

func AppendToChangelog(commitMsgPath string, alsoTag bool) {
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

  if alsoTag {
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
  configPathPtr := flag.String("config", "", "Path to the configuration file")
  flag.Parse()

  if len(os.Args) == 1 {
    log.Fatal("Please provide the commit message file or specify branch mode with `-branch`")
  }

  readConfig(*configPathPtr)

  if *branchModePtr {
    WriteBranchChangelog()
    return
  }
  
  AppendToChangelog(os.Args[1], *config.AlsoTag)

}

func main() {
  parseCliArgsAndRun()
}
