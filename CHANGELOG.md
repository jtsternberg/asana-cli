# Changelog

## [3.1.0] - 2026-04-08

### Added

- **All commands now display full API data** — every CLI command (`tasks view/list/search`, `projects list/tasks/sections`, `users list`, `teams list`, `tags list`, `workspaces list`, `time status/create`) now shows all available fields from the Asana API in both text and JSON output
- **`--json` flag on 6 more commands** — `teams list`, `tags list`, `workspaces list`, `projects sections`, `time status`, and `time create` now support structured JSON output
- **Assignee in task view** — `tasks view` was fetching the assignee from the API but never displaying it. Now shown in both text and JSON output
- **Rich task list/search output** — `tasks list` and `tasks search` now show assignee, completion status, projects, tags, custom fields, dependencies, and more (previously only showed name and due date)
- **User email in users list** — `users list` now displays email addresses in both text and JSON output
- **Custom fields in task output** — task commands display custom field names and display values
- **Dependencies/dependents in task view** — task view now shows blocking and blocked-by relationships
- **~60 new tests** across all updated commands

### Changed

- **Skill doc uses live `--help` output** — replaced static flag tables with ` ```! ` shell execution blocks that inline the actual `--help` text at skill load time. Docs can never go stale.
- **Task view no longer double-fetches** — `displayDetails` was redundantly calling `task.Fetch()` after the caller already fetched. Removed the extra API call.
- **Projects list requests more API fields** — `opt_fields` expanded to include owner, team, dates, notes, and more

### Fixed

- **`tasks view --json` returned `"assignee": null` for assigned tasks** — the Task struct had the assignee populated but the JSON output struct never included it

## [3.0.0] - 2026-04-03

### BREAKING

- **Consolidated 6 Asana skills into 1** — `asana-create-task`, `asana-update-task`, `asana-move-task`, `asana-delete-task`, and `troubleshooting-asana` are removed. All functionality is now in `asana-task-manager` (formerly `using-asana-cli`) with operation-specific reference files. Agents referencing the old skill names will need to update.

### Changed

- **Operation-specific workflows moved to reference files** — `references/CREATE_TASK.md`, `UPDATE_TASK.md`, `MOVE_TASK.md`, `DELETE_TASK.md`, `TROUBLESHOOTING.md` contain step-by-step instructions. The main skill routes to the right reference based on the operation.
- **Reference file reading is mandatory** — the skill now uses imperative language requiring agents to read the relevant reference before performing any operation

### Fixed

- **Task skills now use `--search`/`-q` for project discovery** — previously used `asana projects list -l 20` which missed projects beyond the first 20

## [2.5.0] - 2026-04-02

### Added

- **`--search`/`-q` flag on `projects list`** — searches projects by name using the Asana typeahead API, bypassing the 100-project pagination ceiling entirely. `asana projects list -q "outgoing"` finds it instantly.
- **`--json` flag on `users list`** — structured output with user IDs and names for programmatic use
- **Name resolution on `tasks search --assignee` and `--creator`** — pass names instead of IDs. `asana tasks search --assignee "Tom McFarlin"` now works (previously required a numeric user ID).
- **Typeahead API support** — new `Workspace.Typeahead()` and `Workspace.SearchProjects()` methods in the API client

### Changed

- **TDD rule added to CLAUDE.md** — tests first, code second

## [2.4.0] - 2026-04-01

### Added

- **`--cc` flag on `tasks create` and `tasks update`** — hidden alias for `--followers`, because agents and humans naturally reach for "CC" when adding collaborators. Now it Just Works instead of failing silently.
- **Due date keyword echo in task output** — when using `--due today` or `--due tomorrow`, the success output now shows the resolved date with the keyword in parentheses (e.g., `Due: Apr 1, 2026 (today)`), making it easy to verify date resolution.
- **Translation Layer in agent skills** — new section mapping natural language intent to correct CLI flags (e.g., "CC Chris" → `--followers`, "due today" → `--due today` literal). Prevents agents from hallucinating flags or pre-resolving date keywords.
- **Post-Mutation Verification guidance in agent skills** — agents are now instructed to read CLI output after create/update operations and verify all expected fields are present. No more claiming success based on vibes.

### Fixed

- **hooks.json uses record instead of array** — corrected the hooks configuration format

### Changed

- **`golang.org/x/sync` promoted to direct dependency** — was indirect, now explicit
- **publish-release moved from command to skill** — relocated from `.claude/commands/` to `.claude/skills/` for proper slash command discovery

## [2.3.0] - 2026-03-17

### Added

- **Non-interactive `projects tasks`** — accepts an optional positional argument for project name or ID, with exact and fuzzy matching (matching `projects sections` pattern). Falls back to interactive prompter when omitted.
- **`--json` flag on `projects tasks`** — machine-readable output with task IDs, including section-grouped JSON for `--sections` mode
- **`--json` flag on `projects list`** — structured output for scripting workflows
- **`--project`/`-p` filter on `tasks search`** — scope search results to specific project IDs via the Asana `projects.any` API parameter
- **`--limit` flag on `projects tasks`** — cap total tasks returned across all pages/sections
- **Task IDs in `projects tasks` output** — shown in both human-readable and JSON modes
- **Concurrent section-task fetching** — `projects tasks --sections` now fetches up to 5 sections in parallel using `errgroup`, reducing wall-clock time by ~4-5x on projects with many sections
- **Rate-limit retry with backoff** — concurrent fetches automatically retry on 429 responses (up to 3 attempts) using the `Retry-After` header or exponential backoff

### Fixed

- **Pagination error on large projects** — `projects tasks` and `projects sections` now set proper page-size limits, preventing "result too large" 400 errors from the Asana API
- **`RetryAfter` header parsing** — fixed inverted nil check in `errors.go` that caused the parsed Retry-After value to never be stored
- **Negative `--limit` validation** — `projects tasks` now rejects negative limit values, matching `projects list` behavior
- **Server-side limit in `tasks search`** — `--limit` is now passed to the Asana API to avoid over-fetching
- **JSON field name in jq examples** — corrected `gid` to `id` in documentation examples

### Changed

- **Limit comparison normalized** — standardized `>=` comparison for limit checks across `tasks search` and `projects tasks`

## [2.2.0] - 2026-03-16

### Added

- **`tasks move` command** — move tasks between projects and sections without delete/recreate. Supports `--project`, `--section`, and `--keep` flags with both interactive and non-interactive modes.
- **`/asana-move-task` slash command** in the Claude Code plugin

### Changed

- **Claude plugin commands migrated to skills format** — all plugin commands now use `skills/<name>/SKILL.md` with proper YAML frontmatter, `$ARGUMENTS` placeholders, `allowed-tools`, and dynamic auth context injection per Claude Code best practices
- **Background skills** (`using-asana-cli`, `troubleshooting-asana`) now use `user-invocable: false` so they auto-load without cluttering the `/` menu

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
