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
| `proji init` | Sets up git on this computer: identity, default branch name, and an SSH key registered with GitHub. Runs automatically the first time any command below needs git and can't find it, but you can also run it directly any time (e.g. to fix something, or set up a second machine). |
| `proji copy owner/repository` | Forks `owner/repository` to your GitHub account, clones your fork locally, and configures the original as an `upstream` remote. |
| `proji create name` | Creates a new **public** GitHub repository named `name`, a matching local folder with a placeholder `README.md`, and pushes the initial commit with `origin` pointing at it. |
| `proji add` | Turns the **current folder** into a new public GitHub repository: creates the repo on GitHub (named after the folder), initializes git locally if needed, commits everything with the message `init: initializing`, and pushes. |
| `proji check upstream` | Fetches from `upstream` (the instructor's original repository) and brings any new changes into your work: fast-forward if possible, otherwise rebase, otherwise (with your confirmation) merge. |
| `proji checkout` | Same as `check upstream`, but against `origin` (your own fork) — useful if you work from more than one machine. |
| `proji checkin` | Commits and pushes your changes. Prompts for a commit message; press Enter to use a default of `<directory name>: <date and time>`. |
| `proji upgrade` | Checks for a newer proji release and installs it over the current binary — no need to re-run the install script to check. |

Run `proji <command> --help` for details on any command.

### Idempotent by design

`proji init`, `proji create`, and `proji add` are all safe to run more than once. If a run fails partway through — a network blip, a missing prerequisite, anything — just run the same command again: every step checks whether it's already done and skips it (with a message saying so) instead of erroring or redoing it. Nothing is ever silently overwritten (an existing `README.md`, git identity, or SSH key is left alone, not replaced).

## First run: setting up git

Every command above needs git. If proji can't find `git` on your machine, it asks whether you'd like it installed automatically:

- **macOS**: uses Homebrew if it's installed; otherwise opens the Xcode Command Line Tools installer (a GUI flow — finish that, then re-run your command).
- **Debian, Ubuntu, Raspbian (and derivatives)**: runs `sudo apt-get update && sudo apt-get install -y git`.
- Anywhere else, proji doesn't know how to install git for you and points you to https://git-scm.com/downloads.

Once git is available, proji also configures it — setting the global default branch name to `main`, your git identity, an SSH key, and registering that key with GitHub. Each of these is checked individually and skipped (with a message) if it's already set, so this is safe however many times it runs.

You don't have to wait for this to happen automatically: run `proji init` any time to (re-)do this setup directly, e.g. after installing git yourself, on a second machine, or to fix something without starting over. Specifically, it:

1. Sets the global default branch name to `main`, unless it's already set that way.
2. Asks for your name and email and sets them with `git config --global`, unless they're already set.
3. Generates a passphrase-less ECDSA-256 SSH key at `~/.ssh/id_ecdsa`, unless you already have one there.
4. Registers your SSH key with your GitHub account automatically (via the GitHub API), unless it's already registered. If registration isn't possible — you're not signed in, or the API call fails — it prints the public key and a link (https://github.com/settings/ssh/new) with instructions to add it yourself.

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
- **The SSH key proji generates and registers isn't currently used by proji itself.** `copy`, `create`, `add`, `checkin`, and `checkout` all push over HTTPS using your token, not SSH. The key is set up because it's a standard part of getting git working on a fresh machine (e.g. for tools outside proji, or a future SSH-based remote mode), not because proji's own git operations depend on it.
- **`proji add` names the new GitHub repository after your current folder.** There's no argument to override it — rename the folder first if you want a different name.
- **`proji upgrade` supports `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`** — the same platforms proji ships release binaries for.
