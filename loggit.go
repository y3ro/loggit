package main

import (
  "bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
  defaultBumpVersionMsg = "Bump version"
  defaultVersionRegexStr = "\\d+\\.\\d+\\.\\d+"
  defaultLogGitTrailer = "log:" // TODO: add space
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

func newVersion(commitMsgPath string) (string, error) {
  commitMsgFile, err := os.Open(commitMsgPath)
  if err != nil {
    log.Fatalln("Could not open the commit message file")
  }
 
	commitMsgBytes, err := io.ReadAll(commitMsgFile)
	if err != nil {
    log.Fatalln("Could not read the commit message")
	}

  commitMsg := string(commitMsgBytes)
  if (!strings.HasPrefix(commitMsg, config.BumpVersionMsg)) {
    return "", fmt.Errorf("No new version in this commit")
  }

  versionMatch := config.VersionRegexp.Find(commitMsgBytes)  
  if (len(versionMatch) == 0) {
    return "", fmt.Errorf("Invalid format for new version in this commit") // TODO: this should be fatal
  }

  return string(versionMatch), nil
}

func firstCommitHash() string {
    cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
    out, err := cmd.Output()
    if err != nil {
      log.Fatalln("Falling back to first commit, there was an error")
    }

    return strings.TrimSpace(string(out))
}

// TODO: these "getters" should use a verb to signal possible errors
func prevBumpCommitHash() string {
  grepArg := "--grep=" + config.BumpVersionMsg
  formatArg := "--pretty=format:%H"

  cmd := exec.Command("git", "log", grepArg, "-n", "1", formatArg)
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Could not read the previous bump-commit hash")
  }

  outStr := string(out)
  outLines := strings.Split(outStr, "\n")
  if len(outLines) == 1 && outLines[0] == "" { // TODO: no parenthesis
    return firstCommitHash()
  }

  return outLines[0]
}

func collectLogMsgs(prevCommitHash string) []string {
  commitsInterval := prevCommitHash + "..HEAD"
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

func versionLogHeader(version string) string {
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
    _, err = tempLogFile.WriteString(scanner.Text())
    if err != nil {
      log.Fatal(err)
    }
  }

  if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }
  tempLogFile.Sync()
}

func AppendToChangelog(commitMsgPath string) {
  newVersion, err := newVersion(commitMsgPath)
  if err != nil {
    fmt.Println(err)
    os.Exit(0)
  }
  newVersionHeader := versionLogHeader(newVersion)

  prevHash := prevBumpCommitHash()
  logMsgs := collectLogMsgs(prevHash)

  tempFile, err := os.CreateTemp("", "loggit-")
  if err != nil {
    log.Fatalln("Could not create a temporary file")
  }
  defer tempFile.Close()

  writeTempLogFile(tempFile, newVersionHeader, logMsgs)

  err = os.Rename(tempFile.Name(), config.ChangelogRelativePath)
  if err != nil {
    log.Fatal(err)
  }
}

func main() {
  err := readConfig()
  if err != nil {
    log.Fatal(err)
  }

  if len(os.Args) == 1 {
    log.Fatal("Please provide the commit message file")
  }
  AppendToChangelog(os.Args[1])
}
