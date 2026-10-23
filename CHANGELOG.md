# Changelog

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
