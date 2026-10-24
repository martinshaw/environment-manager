# Changelog

## Unreleased

## 1.0.0 - 2026-10-24

- `envm upgrade` installs the latest GitHub release over the current binary after checking `checksums.txt`. `envm upgrade --check` reports whether a newer release exists.
- First-run setup stores each project's existing `.env` as `Local Development` instead of asking for a name.
- The recovery key is copied to the clipboard and shown on one line.
- Pending findings advance as soon as they are handled, and `c` adds `.env` to `.gitignore`.
- `use`, `update`, `new`, `revert`, `diff`, and the TUI project list follow the directory `pwd` prints.
- A monorepo directory with its own `.env`, such as `packagevault/packages/web`, is its own project. Dependency directories are skipped.
- Commenting a line no longer stores a version immediately. `u` updates the open env file, `a` saves a new env file, and `R` discards the comment changes.
- `s` selects the focused env file. On a version, `s` stores that snapshot as the next version and selects it.
- Each version is shown with the local date and time it was stored, next to its number.

## 0.1.0 - 2026-10-23

Initial release of envm, a multi-platform tool for versioned `.env` files.

- Bubble Tea TUI to search projects, stored env files, and versions, then view or edit the chosen snapshot.
- Passphrase-encrypted snapshots in `~/.envm`, with a one-time recovery key.
- Commands that follow the current directory: `use`, `update`, `new`, `revert`, and `diff`.
- Version numbers (`v1`, `v2`, `v3`, …) and unified diffs between snapshots.
- Copy the open snapshot as a Teams markdown fence, plain text, or a diff.
- Dotenv lint, and comment or uncomment a line as a new version.
- Login watcher for manual `.env` edits and for `.env` missing from `.gitignore`, with update, new file, and revert actions.
- Release binaries for Linux, macOS, and Windows on amd64 and arm64.
