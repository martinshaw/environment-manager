# envm

envm keeps named, versioned copies of the `.env` files in your local projects. Point it at one or more parent folders, such as a Laravel Herd directory and a general projects directory. Each child directory becomes a project. You can store several env files per project (`Staging`, `Production`, a teammate's copy) and switch which one is written to `.env`.

Snapshots are encrypted with a passphrase you choose. A background watcher notices when you edit `.env` by hand, and when `.gitignore` does not cover `.env`.

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [Concepts](#concepts)
- [Commands](#commands)
- [TUI](#tui)
- [Daemon](#daemon)
- [Storage and encryption](#storage-and-encryption)
- [Gitignore checks](#gitignore-checks)
- [Development](#development)
- [License](#license)

## Install

Release archives are built for Linux, macOS, and Windows, each on `amd64` and `arm64`. Download the archive for your machine from the [releases](https://github.com/martinshaw/environment-manager/releases) page, unpack it, and put `envm` (or `envm.exe`) on your `PATH`.

```bash
# macOS Apple silicon
curl -sL https://github.com/martinshaw/environment-manager/releases/download/v1.0.0/envm_1.0.0_darwin_arm64.tar.gz | tar -xz
sudo install envm /usr/local/bin/envm

# Linux x86_64
curl -sL https://github.com/martinshaw/environment-manager/releases/download/v1.0.0/envm_1.0.0_linux_amd64.tar.gz | tar -xz
sudo install envm /usr/local/bin/envm
```

On Windows, download `envm_1.0.0_windows_amd64.zip` or `envm_1.0.0_windows_arm64.zip` and extract `envm.exe`.

Checksums for every archive are in `checksums.txt` on the same release.

After envm is on your `PATH`, later releases replace that binary:

```bash
envm upgrade          # download, verify, and install the latest release
envm upgrade --check  # report whether a newer release exists
```

`envm upgrade` is not `envm update`. Update stores a new snapshot of the project's `.env`. Upgrade replaces the envm program from GitHub.

The Go module path is `martinshaw.co/environment-manager`. `go install` works after `https://martinshaw.co/environment-manager` serves these tags:

```html
<meta name="go-import" content="martinshaw.co/environment-manager git https://github.com/martinshaw/environment-manager">
<meta name="go-source" content="martinshaw.co/environment-manager https://github.com/martinshaw/environment-manager https://github.com/martinshaw/environment-manager/tree/master{/dir} https://github.com/martinshaw/environment-manager/blob/master{/dir}/{file}#L{line}">
```

```bash
go install martinshaw.co/environment-manager/cmd/envm@v1.0.0
```

Until that page exists, install a release binary. Building from this repository also works:

```bash
go build -o envm ./cmd/envm
```

Requires Go 1.24.2 or newer.

## Quick start

Run `envm` and create a passphrase of at least 10 characters. The recovery key is copied to the clipboard and shown once, on a single line. Store it somewhere safe before continuing. A forgotten passphrase cannot be undone without that key.

Add a root directory. Its child folders become projects, named after the directory. For example, `/Users/martin/Herd/Todo App` becomes the project `Todo App`.

During that first setup, envm stores each project's existing `.env` as an env file named `Local Development` and selects it. Later, `a` still asks you to name a new env file.

From that project directory:

```bash
envm new "Staging Todo App"   # store the current .env as v1 and select it
envm new "Production App"      # store another copy
envm use "Staging Todo App"   # write that snapshot back to .env
# edit .env
envm update                      # store v2 when the file differs from the latest snapshot
envm revert                      # put the latest snapshot back into .env
envm diff                        # unified diff from the snapshot to .env
```

`ENVM_HOME` overrides the data directory. The default is `~/.envm`.

## Concepts

**Roots** are directories you choose. Each non-hidden child directory is a project. Hidden names (they start with `.`) are skipped. A root itself is not a project. Inside a project, a directory that contains its own `.env` is a project too. `~/Developer/packagevault/packages/web/.env` is managed separately from the `packagevault` repository root. `node_modules`, `vendor`, `dist`, `build`, and other dependency or build directories are skipped.

**Projects** start with the directory name. A nested `.env` starts with its path from the root, such as `packagevault/packages/web`. You can rename a project and point it at a different path. Hiding a project removes it from the TUI list and leaves the directory on disk. Removing a project from envm also leaves the directory and its `.env` in place.

**Env files** are named snapshots for one project, such as `Bob's Todo App` or `Staging Todo App`. Names are unique per project.

**Versions** are `v1`, `v2`, `v3`, and so on, shown with the local date and time they were stored, such as `v1  24 Oct 05:18`. Each version is a full copy of the file, stored encrypted. Diffs are calculated when you view or copy them. Bytes are kept as they were: envm does not reformat quotes or add a trailing newline on its own.

**Selected** means one env file is the one `.env` should contain. The on-disk file should match that env file's latest version. `envm use` writes a snapshot and selects it. `envm new` selects the file it just stored.

Commands that read or write `.env` start from the directory the terminal is in, which is what `pwd` prints. They match a project whose path is that directory, or the nearest parent that is a known project. Inside a monorepo, a terminal sitting in `packages/web` selects that package, not the repository root. They do not treat a configured root as a project.

## Commands

- `envm` opens the TUI.
- `envm use <name>` writes that env file's latest version to `./.env` and selects it.
- `envm update` stores a new version of the selected env file when `./.env` differs from the latest snapshot.
- `envm new <name>` stores `./.env` as a new env file at `v1` and selects it.
- `envm revert` writes the selected latest version back over `./.env`.
- `envm diff` prints a unified diff from the selected latest version to `./.env`.
- `envm drift` opens the TUI on pending watcher findings. If there are none, it prints `No pending changes.`
- `envm roots add <path>` adds a root and scans it.
- `envm roots remove <path>` removes a root. Projects already recorded are kept.
- `envm roots list` lists roots.
- `envm daemon run` runs the watcher in the foreground.
- `envm daemon install` registers a login watcher and starts it.
- `envm daemon uninstall` stops it and removes the login service.
- `envm daemon start`, `envm daemon stop`, and `envm daemon status` control the watcher.
- `envm upgrade` downloads the latest GitHub release for this machine, checks `checksums.txt`, and replaces the current binary. `--check` only reports whether a newer release exists.
- `envm completion bash|zsh|fish|powershell` prints a shell completion script.
- `envm version` prints the version. `--version` does the same.

`use`, `update`, `new`, `revert`, and `diff` ask for the passphrase in the terminal. `roots` and `daemon` do not, because they do not read secret values.

## TUI

The sidebar has three lists: projects, env files, and versions. The main pane shows the open snapshot. With no search, the project for the directory the terminal is in (`pwd`) is listed first, then the selected env file, then the latest version. A search still prefers that row when the fuzzy score ties.

`tab` moves between the lists and the file. On a terminal narrower than 100 columns, `tab` switches between the sidebar and the file. `s` on an env file makes it the selected file and writes its latest version to `.env`. `s` on a version does the same with that snapshot: when it is older than the latest, envm stores those bytes as the next version first, so v1 can become the current contents without reordering history.

| Key | Action |
| --- | --- |
| `j` `k` or arrows | Move in the focused list, or move the line cursor in the file |
| `/` | Filter the focused list. In the file, search its contents |
| `n` `N` | Next and previous file-search match, while a search is active |
| `enter` | Move to the next sidebar section |
| `e` | Edit the open snapshot. `ctrl+s` saves a new version. `esc` cancels |
| `c` | Comment or uncomment the current line. Changes wait until `u` updates the open env file, `a` saves a new env file, or `R` discards them |
| `x` | Delete the focused project, env file, or version, after confirmation |
| `r` | Rename the focused project or env file |
| `p` | Edit the focused project's path |
| `H` | Hide the focused project. `g` then enter shows it again |
| `d` | Diff the open version against the previous one |
| `D` | Diff the selected latest version against the on-disk `.env` |
| `l` | Lint dotenv syntax |
| `y` then `m`, `t`, or `d` | Copy a markdown fence, plain text, or a diff |
| `s` | Select the focused env file, or make the focused version current by storing it as the next version |
| `u` | When `.env` differs from the selected env file, store it as the next version. Otherwise it does nothing |
| `a` | Store `.env` as a new named env file and select it |
| `R` | Revert `.env` to the selected latest version |
| `o` | Open pending watcher findings |
| `g` | Roots and hidden projects |
| `?` | All keys |
| `q` | Quit. `ctrl+c` quits from any screen |

Saving an edit creates the next version when the bytes changed. Commenting lines does not. After one or more comment changes, `u` stores them as the next version of the open env file, `a` stores them as a new env file, and `R` discards them. If that env file is the selected one, saving or `u` updates `.env` too so the disk matches the new snapshot. `a` selects the new file and writes it to `.env`.

Lint reports illegal or empty keys, a missing `=`, unterminated quotes, duplicate keys, and unquoted values that contain spaces. Blank lines, `#` comments, an optional `export` prefix, and spaces around `=` are accepted.

The markdown copy uses a `env` fence. The fence is longer than three backticks when the file itself contains a backtick run that would close a shorter fence. That is the form to paste into Microsoft Teams.

Deleting a version, env file, or project removes it from envm only. The project directory and the on-disk `.env` stay.

## Daemon

`envm daemon install` registers a watcher that starts at login:

- macOS: `~/Library/LaunchAgents/co.martinshaw.envm.plist`
- Linux: `~/.config/systemd/user/envm.service` (enabled with `systemctl --user`)
- Windows: a logon scheduled task named `envm`

The service runs `envm daemon run` using the binary path from install time. `envm upgrade` replaces that binary in place and restarts a running watcher. If you move the binary to a different path, run `envm daemon install` again.

The watcher uses filesystem events on each root and each project directory, so an editor's atomic save is seen. It also scans about every 30 seconds. It records a finding only when the state changes, so the same edit is not announced again.

- `.env` differs from the selected latest snapshot: notification offers update, save as a new file, or revert.
- `.env` exists and nothing is selected yet: notification offers to save it as a new file. First-run setup does this itself, as `Local Development`.
- `.env` is not ignored, or it is tracked by git: notification. Inside a git repository this reads the ignore rules, including `.git/info/exclude` and the global git excludes file, and the index when `.env` is already tracked. Otherwise envm reads `.gitignore` files above the project.

The daemon does not receive the passphrase or the file contents. It compares a SHA-256 of `.env` with the hash stored next to each snapshot. Opening `envm`, or running `envm drift`, unlocks the vault and asks what to do. `u` updates, `a` saves a new file, `R` reverts, `c` adds `.env` to the project's `.gitignore` (creating the file when needed), and `x` dismisses the finding until the file or the ignore status changes again. If git already tracks `.env`, the rule is still added, and envm says that it will not take effect until the file is untracked.

On Linux, the notification's default action launches `envm drift` when `notify-send` supports actions. Elsewhere, open `envm` or run `envm drift`. The pending queue is `~/.envm/pending.json`.

While the TUI is open and the daemon is not running, the TUI watches for that session. Findings show up in the same prompt. Writes made by envm itself match the stored hash, so they do not raise a new finding.

If no login service is installed, `envm daemon start` runs the watcher in the background. `envm daemon stop` stops it.

## Storage and encryption

Everything lives in `~/.envm` (mode `0700`). Files are mode `0600`.

| Path | Contents |
| --- | --- |
| `config.json` | Root directories. Not secret. |
| `projects.json` | Project names, paths, env file names, version numbers, timestamps, and SHA-256 hashes. Names are not encrypted. File contents are not in this file. |
| `key.age` | Data key wrapped with your passphrase (`age` scrypt). |
| `recovery.age` | The same data key wrapped for the one-time recovery identity. The identity is not stored. |
| `content/<id>` | AES-GCM snapshot. The nonce is the start of the file. |
| `pending.json` | Watcher findings. Project names and paths only. |
| `state.json` | Hashes and ignore states already announced. |
| `daemon.log` | Watcher log. |
| `daemon.pid`, `daemon.port`, `daemon.token` | Present while the daemon is running. The token is mode `0600` and is only accepted from localhost. |

The passphrase is required when envm reads or writes snapshot bytes. A forgotten passphrase cannot be undone without the recovery key. On the unlock screen, `ctrl+r` switches to the recovery key. There is no other reset.

The SHA-256 values are stored so the locked watcher can see that `.env` changed. They are not the file contents. Someone who can already read `~/.envm` can test whether a file they have matches a snapshot.

A flock file serializes writers. Two envm processes can run; they will not interleave a save.

## Gitignore checks

envm looks at `.env` in the project directory, not at `.env.example`. A project with no `.env` is not asked to add a gitignore rule.

In a git repository envm reads the ignore rules and the index itself. It does not run `git`. A tracked `.env` stays in the repository until you untrack it; adding a gitignore rule does not remove it from history. envm will not untrack the file for you.

Outside git, envm reads `.gitignore` files in the project directory and up to three parents. The last matching pattern wins, including a later `!.env` negation.

Dismissing the notice leaves it dismissed until the ignore result changes.

## Development

```bash
go test ./...
go build -o envm ./cmd/envm
```

Tests cover dotenv lint and comment toggling, encrypt and decrypt, version bumps, current-directory project resolution, diffs and markdown fences, gitignore coverage, watcher findings, the pending queue, daemon startup, notifications, the command line, self-upgrade, and a TUI browse pass.

Pushing a tag `vX.Y.Z` runs GoReleaser. The GitHub release body is `CHANGELOG.md`, and the artifacts are the six platform archives plus `checksums.txt`. Update the changelog before tagging.

Module path: `martinshaw.co/environment-manager`.

Default branch: `master`.

## License

Copyright (C) 2026 Martin Shaw.

envm is free software under the GNU General Public License version 3 only. See [LICENSE](LICENSE).
