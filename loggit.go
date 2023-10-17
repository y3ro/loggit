package main

import (
	"encoding/json"
  "os/exec"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
  defaultBumpVersionMsg = "Bump version"
  defaultVersionRegexStr = "\\d+\\.\\d+\\.\\d+"
  defaultLogGitTrailer = "log:"
  defaultChangelogRelativePath = "CHANGELOG.md"
  defaultVersionHeader = "## Version "
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

    return string(out)
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
  if (len(outLines) == 0) { // TODO: no parenthesis
    return firstCommitHash()
  }

  return outLines[0]
}

// TODO: check if trimming is necessary
func collectLogMsgs(prevCommitHash string) []string {
  commitsInterval := prevCommitHash + "..HEAD"
  grepArg := "--grep=" + config.LogGitTrailer
  formatArg := "--pretty=format:%b"

  cmd := exec.Command("git", "log", commitsInterval, grepArg, formatArg)
  out, err := cmd.Output()
  if err != nil {
    log.Fatalln("Failed to collect log messages")
  }

  outStr := string(out)
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
