## proji

`proji` is a git simplification layer for young people learning software development who need a way to interact with a remote. It bakes in a number of conventions intended to help beginners fetch a remote containing multiple project assignments and check work in when they're ready to have it reviewed.

## Technology

Language: golang
Libraries: 
  - go-github
  - spf13-cobra

## Tooling

- golangci-lint
- gosec
- govulncheck
- Makefile
- GitHub CI Actions
- Release Please

All public surface areas of modules should be tested with behavior style tests (mocking external depedencies like the github API where possible).

Local make should print all available actions. A pre-commit make action should run all tests and checks that would run in CI so there are no surprises when pushing to the remote.

## Design

This tool is aimed at beginners and should use conventions rather than configuration or complex command lines to:
 - prompt a user for a github token or create one via an OIDC login to github (if possible) and store it in their personal home directory under `~/.proji/creds.yml`
 - fork an instructor's repo
 - clone a remote to the local machine
 - fetch and ff-only merge where possible
 - or rebase on top of remote changes when ff-only merge is not possible
 - commit and push work in the current working directory with an optional message from the learner (prompt them for it) with a default set to the date and time on their local machine + the subfolder name

## Use Cases

### Forking the original repository

`proji copy username/repository`

This should create the fork for the user, clone it locally, then change directory to the repository. It should output:

 * that it completed the clone to their github user and print the user/repo as part of the message
 * that it copied their version of the repository to their machine and print the pwd so they know where it was copied
 * any issues that prevented it from successfully completing those tasks

### Fetching from the upstream clone

`proji check upstream`

If an instructor pushes an update out to the original, proji needs a command to check for this and take one of three possible actions after fetching from the remote's upstream:

 - if the changes can be merged with ff-only, perform that action and report back
 - if the changes cannot be merged, perform a rebase instead
 - if there are conflicts, cancel the rebase and ask them if they want to perform a merge that could overwrite changes
 - output the result of the operation

### Fetching from the remote

`proji checkout`

Fetches from the remote and conditionally:

 - if the changes can be merged with ff-only, perform that action and report back
 - if the changes cannot be merged, perform a rebase instead
 - if there are conflicts, cancel the rebase and ask them if they want to perform a merge that could overwrite changes
 - output the result of the operation

### Pushing to the remote

`proji checkin`

Commits and pushes the changes relative to the working directory. It should ask them if they want to provide a message, if they hit enter, it should default to the working directory name (not full path) and append the date and time. Otherwise, it should use their message as the commit message entry after the directory name (not full path).
