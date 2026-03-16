# Changelog

## [Unreleased]

### Added

- **`tasks move` command** — move tasks between projects and sections without delete/recreate. Supports `--project`, `--section`, and `--keep` flags with both interactive and non-interactive modes.

## [2.1.0] - 2026-03-12

### Added

- **`upgrade` command** — self-update the CLI with `asana upgrade`. Detects git-source vs pre-built binary install method, downloads latest release with SHA256 checksum verification, hardened tar extraction, and atomic binary replacement. Supports `--yes` for non-interactive use.

## [2.0.0] - 2026-03-12

First release as a maintained fork of [timwehrle/asana](https://github.com/timwehrle/asana).

### Added

- **Non-interactive CLI mode** — `tasks create`, `tasks update`, and `tasks view` all work without prompts when flags/args are provided
- **`--json` flag** on `tasks list`, `tasks search`, and `tasks view` for machine-readable structured output
- **Task IDs** shown in `list` and `search` text output for scripting workflows
- **`--limit` flag** on `tasks search`, consistent with `tasks list`
- **`tasks delete` command** — delete a task by ID
- **`projects sections` command** — list sections in a project
- **`Task.AddFollowers` API method** — uses `/tasks/{id}/addFollowers` endpoint
- **Fuzzy name matching** — assignee, project, section, and follower flags support case-insensitive exact, partial/contains, and GID matching
- **Claude Code plugin** — skills (`using-asana-cli`, `troubleshooting-asana`), commands (`/asana-create-task`, `/asana-update-task`, `/asana-delete-task`), and autonomous `asana-task-manager` agent

### Changed

- **`--assignee` on search** no longer defaults to `me` — omit to search all assignees, pass `--assignee me` explicitly
- **`--creator-any` renamed to `--creator`** for natural flag naming
- README overhauled for fork identity with features overview, quick start, and streamlined sections

### Fixed

- Extract `getOrPromptDueDate` helper to fix undefined reference in tests
