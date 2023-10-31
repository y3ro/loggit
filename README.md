# loggit

Automate the generation of changelogs by using specific Git commit trailers.
`loggit` allows you to log important changes in your commits and have them collected later into a changelog, so you don't forget--or are too lazy to do it, like me.

## Installation

Assuming you have Go installed:

```
go install github.com/y3ro/loggit@latest
```

You should have `$HOME/go/bin` in your `PATH`.

## Usage

You can use `loggit` to populate a version changelog and also branch changelogs.

### Version changelog

Collect log entries starting from the last "bump-version" commit.

Quick example:

0. Call `loggit` from your Git repository's  `commit-msg` hook, passing the first argument to the hook, like so:
```
loggit $1
```

1. You have finished a feature or squashed a bug and want to commit the changes, so you add the following to your commit's body:
```
[...]

log: [FEATURE] Added amazing new feature

[...]
```
you can replace `log:` with your preferred key in the configuration file.

Alternatively, you can use the subject of the commit message by using a special log line:
```
[FEATURE] Added amazing feature

[...]

log: %s

[...]
```

2. In a later commit, you bump the version of your software and write the corresponding commit message's title:
```
Bump version 1.3.0
```
again, you can configure the format of this special commit. See below.

3. Check your changelog to see the added entries and also that a new tag was created, unless you specified otherwise.

### Branch changelog

You can use the `-branch` flag to collect all the log entries for the current branch.
Normally, you would use this more directly:
```
loggit -branch
```
The resulting changelog file will have the branch name prefixed to its name.

This is useful to summarize the work in a particular branch for a merge/pull request.

### Configuration

You can configure `loggit` using a `loggit.json` file which can live in the root of your repository or in `$HOME/.config/`.
This is also the order of precedence to choose between the two, if both are available.
You can also specify you preferred file anywhere in the file system by using the `-config` parameter.

Example contents of this file, which are also the default values for each setting:

```
{
    "BumpVersionMsg": "Bump version",
    "VersionRegexpStr": "\\d+\\.\\d+\\.\\d+",
    "LogGitTrailer: "log:",
    "UseCommitTitleMsg: "%s",
    "ChangelogRelativePath: "CHANGELOG.md",
    "VersionHeader: "# Version ",
    "MasterBranchName: "master",
    "ConfigFileName: "loggit.json",
    "AlsoTag": true
}
```
