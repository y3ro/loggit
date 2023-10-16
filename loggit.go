package main

import (
  "fmt"
  "io"
  "encoding/json"
  "os"
  "path/filepath"
  "regexp"
  "runtime"
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
