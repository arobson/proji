# proji

`proji` is a git simplification layer for people learning software development. It bakes in a small set of conventions so you can fetch an instructor's assignment repository and check your work in when it's ready to be reviewed, without having to learn git's full command surface first.

## Installation

**One-liner (macOS and Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/arobson/proji/main/install.sh | bash
```

The script detects your platform, downloads the correct binary from the latest release, verifies its checksum, installs it to `/usr/local/bin`, and warns you if that directory is not in your `PATH`. Every release (cut by [Release Please](https://github.com/googleapis/release-please) from conventional commits on `main`) automatically builds and publishes binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`, along with `checksums.txt`.

Every release binary also carries a [build provenance attestation](https://github.com/arobson/proji/attestations), verifiable independently of the checksum:

```bash
gh attestation verify dist/proji-<os>-<arch> -R arobson/proji
```

**From source:**

```bash
git clone https://github.com/arobson/proji
cd proji
make install       # go install ./cmd/proji → $GOPATH/bin/proji
```

Or without cloning:

```sh
go install github.com/arobson/proji/cmd/proji@latest
```

## Commands

| Command | What it does |
|---|---|
| `proji copy owner/repository` | Forks `owner/repository` to your GitHub account, clones your fork locally, and configures the original as an `upstream` remote. |
| `proji create name` | Creates a new **public** GitHub repository named `name`, a matching local folder with a placeholder `README.md`, and pushes the initial commit with `origin` pointing at it. |
| `proji check upstream` | Fetches from `upstream` (the instructor's original repository) and brings any new changes into your work: fast-forward if possible, otherwise rebase, otherwise (with your confirmation) merge. |
| `proji checkout` | Same as `check upstream`, but against `origin` (your own fork) — useful if you work from more than one machine. |
| `proji checkin` | Commits and pushes your changes. Prompts for a commit message; press Enter to use a default of `<directory name>: <date and time>`. |

Run `proji <command> --help` for details on any command.

## First run: setting up git

Every command above needs git. If proji can't find `git` on your machine, it asks whether you'd like it installed automatically:

- **macOS**: uses Homebrew if it's installed; otherwise opens the Xcode Command Line Tools installer (a GUI flow — finish that, then re-run your command).
- **Debian, Ubuntu, Raspbian (and derivatives)**: runs `sudo apt-get update && sudo apt-get install -y git`.
- Anywhere else, proji doesn't know how to install git for you and points you to https://git-scm.com/downloads.

Once git is available, proji also:

1. Generates a passphrase-less ECDSA-256 SSH key at `~/.ssh/id_ecdsa` if you don't already have one there.
2. Asks for your name and email and sets them with `git config --global`.
3. Registers your new SSH key with your GitHub account automatically (via the GitHub API). If that isn't possible — you're not signed in, or the API call fails — it prints the public key and a link (https://github.com/settings/ssh/new) with instructions to add it yourself.

This only happens once, the first time you run proji on a machine without git.

## Signing in to GitHub

The first time proji needs to talk to GitHub (typically on `proji copy`), it signs you in with **GitHub OAuth Device Flow**: it prints a URL and a short code, you open the URL, enter the code, and approve access — no token to copy and paste. The resulting token is stored in `~/.proji/creds.yml`.

proji ships with its own registered GitHub OAuth App (the client ID is baked into the binary — client IDs are public identifiers, not secrets, and Device Flow never involves a client secret at all, per [RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)). Maintainers of a fork who want to use their own OAuth App instead can change `githubOAuthClientID` in `internal/cli/root.go`; that app just needs **Device Flow enabled** in its settings (the Authorization callback URL GitHub requires you to fill in is unused by Device Flow — any placeholder like `http://127.0.0.1/callback` works).

If Device Flow ever can't be used, proji falls back to asking you to paste a personal access token directly (it prints a link to create one with the right scope).

## Development

```sh
make help         # list all available targets
make pre-commit    # everything CI runs: fmt, vet, lint, gosec, govulncheck, tests, build
```

Unit tests are fast and mock all external dependencies (the `git` binary, the GitHub API, the terminal). A small integration suite (`make test-integration`) exercises the real `git` binary against temporary local repositories to validate assumptions about its exit codes and output.

## Known limitations

- **`proji copy` can't change your shell's directory.** A subprocess can't do that for its parent shell, so `copy` prints a `cd <path>` line for you to run.
- **`check upstream` / `checkout` fetch the remote branch with the same name as your current local branch.** This matches the fork workflow proji sets up and avoids guessing at a remote's default branch, but it means renaming your local branch away from the instructor's branch name will break these commands.
- **Your fork's `origin` remote URL embeds your GitHub token** (`https://...@github.com/...`) so `git push` works without a system credential helper being configured. This means the token is visible in `.git/config` inside your project directory. A future version could replace this with a dedicated git credential helper.
- **The SSH key proji generates and registers isn't currently used by proji itself.** `copy`, `create`, `checkin`, and `checkout` all push over HTTPS using your token, not SSH. The key is set up because it's a standard part of getting git working on a fresh machine (e.g. for tools outside proji, or a future SSH-based remote mode), not because proji's own git operations depend on it.
