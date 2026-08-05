# Changelog

## [3.6.0] - 2026-08-05

### Added

- **`asana projects sections delete <project> <section>`** — there was no way to remove a section, so a workflow that moved every task out of one could not finish in the CLI. An agent doing exactly that decoded the CLI's own keyring token and called `DELETE /sections/{gid}` directly, seven times. Deleting a section does **not** delete its tasks, they move to the project's default section, but the command still refuses a non-empty section by default and names the task count; `--force` overrides, `--yes` skips the prompt for scripted use.

- **`asana projects sections move <project> <section>`** — new sections are always appended to the bottom of a project, and reaching the top previously meant dragging in the web UI. Takes `--first`, `--last`, `--before <section>` or `--after <section>`. Already-in-position is a no-op rather than a request. Contrary to Asana's documentation, this works on list-layout projects, not just board-layout ones.

- **`asana users list -q/--query <text>`** — filters by name or email and shows IDs, which is how you answer "which David?" when a name comes back ambiguous. Shows IDs implicitly, since picking one is the point.

- **`asana tasks search --exclude-assignee` and `--exclude-creator` now accept names.** They previously passed whatever you gave them straight to an API that only accepts GIDs.

### Changed

- **Names now resolve strictly. A reference matching more than one object is an error, not a guess.** This is the headline change and it can break a script or agent that relied on the old behaviour.

  Every project, section and user reference used to try an exact name match and then return the *first* substring match. Against this repository's reference workspace — 1203 projects, 241 people, 22 duplicated first names — `--project Rocks` matched 211 projects and silently created or moved the task into whichever one sorted first; `--assignee David` picked one of five Davids. A wrong assignee or a task in the wrong project is invisible until somebody notices it in the wrong queue, which is precisely what makes the silent version worse than a failure.

  An ambiguous reference now errors and lists the candidates with their IDs (capped at ten, with a count of what was elided):

  ```
  $ asana tasks create -n "Ship it" -p Rocks -a David --non-interactive
  Error: --project: project "Rocks" is ambiguous — 100 projects match:
    Q1 2026 Rocks - SB (ID: 1212478655912226)
    2023 Q4 Rocks (ID: 1205784903414470)
    ...
    …and 90 more
  Re-run with the full project name or its ID (`asana projects list -q "Rocks"` lists the matches).
  ```

  An exact name still beats any number of partial matches, so `-p Lindris` resolves even alongside `Lindris Previous Rocks`. Resolution order is `me` → numeric ID → exact name or email → unique partial name. Email addresses are accepted for users, being the one guaranteed-unique handle. **If a command starts failing where it used to succeed, it was probably resolving to the wrong thing before** — narrow it with `asana projects list -q <text>`, `asana users list -q <text>`, or `asana projects sections <project>`.

- **`me` is reserved in every flag that takes a user**, not just `--assignee`. `--followers me` and `--cc me` used to fall through to substring matching and resolve to real people whose names contain "me" — Angie Meeker, Tom Mendez. This was fixed in `tasks create` alone; it applies to `tasks update --followers`/`--remove-followers` and `tasks search` now too.

- **A named project resolves through the typeahead API instead of enumerating the workspace.** `asana projects sections`, `tasks create --project` and `tasks move --project` each listed all 1203 projects to match one name: 5.5s for a section listing, and exposure to `400: The result is too large` on the unbounded first page. Interactive selection still enumerates, because a picker needs the list.

### Fixed

- **`projects tasks` reported completed tasks as incomplete, and showed no assignee or due date.** `/projects/{gid}/tasks` and `/sections/{gid}/tasks` return compact records — GID, name, resource type — and the listing asked for nothing more while rendering an assignee, a due date and a completion status. Every task therefore arrived with a nil assignee (printed `-`), no due date, and a nil `completed`, which renders as `Incomplete`. Anyone reading the listing to decide what was already done was told something false.

- **`teams list` printed every description as empty** — 17 of 61 teams in the reference workspace have one. Same compact-record cause; the JSON output's `organization` was always `null` too.

- **`tags list` printed every colour as `-`.** Same cause.

- **`workspaces list` labelled organizations as "Workspace".** `is_organization` was never requested.

- **`projects list -q` silently dropped the owner/team column** that the unfiltered listing showed, because the typeahead path requested no fields. The same data rendered as two different-looking answers depending on how you asked.

- **`asana projects sections move` could never have worked.** Its API path was built without a leading slash, which panics before any request is made, and the request body carried the project GID that is already in the path, which Asana rejects with `400: Duplicate field: project` — a trap, since Asana's own documentation lists `project` among the body fields.

- **`tags list` pagination could not paginate.** It built request options, advanced the offset on each pass, and then issued a call that ignored them — so it re-read page one. Masked only because Asana currently returns all 939 tags in a single page. `tag.Tasks` and `project.Sections` were likewise single unbounded calls, and every one of these now pages with a bounded page size.

- **`tasks create` fetched the workspace user list twice and `tasks update` up to three times.** One resolver per run now.

## [3.5.0] - 2026-07-27

### Added

- **`ASANA_WORKSPACE` environment override** — supplies the default workspace as a GID, taking precedence over the config file. With `ASANA_TOKEN`, this is enough to run on a machine that has never run `auth login`: no keyring, no config file, nothing interactive. That closes the gap v3.4.0 shipped with, where the token override removed the keyring dependency but not the dependency on the one command that writes a config file. `auth status` reports the source of the workspace as well as the token.

### Fixed

- **A missing config file is no longer an error** — `Config.Load` treated it as an authentication failure and told the user to run `auth login`, which is exactly what an unattended run cannot do. Load now reads what is there; whether the result is usable is `RequireWorkspace`'s judgement, and its error names both the interactive and the environment route.

- **Recording the build version can no longer fail a command** — every invocation wrote `build:` into the config file and aborted if the write failed. Nothing reads that value back, and a machine configured purely from the environment has no file to write, so a container with a read-only filesystem could never satisfy it.

- **`tasks list` and `workspaces list` no longer print `Tasks for :`** — the username comes from the config file, so an environment-configured machine has none. The heading now omits the subject instead of interpolating an empty one.

- **`auth status` no longer reports "No default workspace configured" when one is in force** — it read `cfg.Workspace` directly and so could not see the environment override.

- **`asana --version` printed a link that 404s, to the wrong project** — the banner pointed at `timwehrle/asana`, the archived repository this one forked from, and dropped the `v` from the tag. It now points at this repository's release for the running version.

- **`tasks create --json` flag help rendered as though the flag took an argument** — Cobra reads backticked text in a usage string as the value placeholder name.

### Changed

- **The Go module is now `github.com/jtsternberg/asana-cli`** — it was still `github.com/timwehrle/asana`, so every import path and both sets of build ldflags named a project that does not ship this code. The reason to leave it alone would have been upstream merge conflicts on every import block; upstream was archived in May 2026 and this repository has never merged from it, so that cost is gone. No effect on anyone installing from a release; `go install github.com/timwehrle/asana/cmd/asana@latest` no longer resolves to this project.

- **Security and conduct reports no longer go to the upstream author** — `SECURITY.md` sent vulnerability reports to the archived upstream maintainer's personal email. It now uses this repository's GitHub private vulnerability reporting, and says explicitly not to send reports upstream. `CODE_OF_CONDUCT.md` named the same address for enforcement.

## [3.4.0] - 2026-07-27

### Added

- **Rich-text task descriptions** — `tasks create` and `tasks update` accept `--markdown-notes` (Markdown, converted for you) and `--html-notes` (hand-written Asana-flavored HTML) alongside plain `--description`. Both produce a description with working links, lists and emphasis instead of a wall of plain text. `--html-notes` is validated locally against Asana's element whitelist before any request is made, so a rejected value costs no round trip, and `<a data-asana-gid="123"/>` becomes an @-mention. Either flag accepts `@file` to read a file or `-` to read stdin.

- **`--dry-run` on `tasks create` and `tasks update`** — resolves assignee, project and section, prints the exact request payload, and sends nothing. This is the only way to inspect the `html_notes` that `--markdown-notes` generated without creating a task.

- **`tasks create --json`** — prints the created task instead of a human summary, so a script no longer has to scrape the `URL:` line to learn the ID of the task it just made. Combined with `--dry-run` it prints just the request payload, with no prose around it.

- **One canonical JSON task shape across `create`, `list`, `search` and `view`** — a new `pkg/taskjson` replaces the four hand-rolled shapes these commands used, so one parser handles all of them, including creating a task and re-fetching it later. `html_notes` is now included, carrying the rich-text form of the description. The identifier key is `id`, not Asana's `gid`, as before.

- **Unassigned and project-less task creation** — `tasks create --unassigned` (or `--assignee ""`) creates a task with no assignee, and `--no-project` creates a workspace-level task with no project or section. Both are explicit on purpose: merely omitting `--assignee` or `--project` remains an error, so a script that forgot one does not quietly produce an ownerless or unfiled task.

- **Emptying fields with `tasks update`** — `--unassigned`, `--no-due` and `--no-description` clear a field rather than set it, and `--incomplete` reopens a completed task. Every field on the update request carries `omitempty` so that an update touches only what was asked for, which had the side effect that no field could be emptied at all.

- **`tasks update --remove-followers`** — unfollow users, the counterpart to `--followers`.

- **`ASANA_TOKEN` environment override** — `ASANA_TOKEN`, or its alias `ASANA_PAT`, takes precedence over the system keyring, and when it is set the keyring is not consulted at all. An unattended job on a headless box no longer depends on a D-Bus Secret Service being present and unlocked. `auth status` reports which source the token came from, and `auth login`, `auth update` and `auth logout` warn when an override is in play, since it outranks anything they store or remove.

### Fixed

- **`asana --version` and `asana --help` no longer require a config file** — on a machine that had never run `auth login`, every invocation died with `No configuration file found`, including those two. Version and help are pure local output; an install script, a container health check and "what have I got installed" all reach for them first, and advice to run `asana upgrade` is useless if you cannot read your current version. Bare `asana` and `completion` were failing the same way.

- **Non-interactive mode no longer stops at optional prompts** — `tasks create` and `tasks update` consulted the `--non-interactive` flag directly rather than the resolved non-interactive state, so a fully-flagged run still stopped to ask for a due date or a description. Prompts are now skipped whenever stdin is not a terminal, which is what makes scripted and agent invocations work without passing the flag at all.

- **`tasks view` reported `num_subtasks: 0` and no dependencies for every task** — `Task.Fetch` requested no fields, so Asana returned only its default set and every opt-in field arrived empty. `num_subtasks`, `dependencies` and `dependents` are now requested and populate in text output as well as JSON.

- **A failed `auth login` no longer leaves a config with no credential behind it** — the config was written before the token, so a keyring failure left a config on disk claiming a login that never completed: `auth status` reported a user and a workspace while every command failed to authenticate. The token is now stored first and rolled back if the config write fails.

- **Local builds report the commit they were built from** — a build from source was labelled with the last release tag, so a locally-built binary was indistinguishable from the release it was built after.

### Changed

- **`list --json` and `search --json` emit more fields** — routing them through the canonical shape means they now include `html_notes`, `memberships`, `dependencies`, `dependents`, `followers`, `workspace`, `created_at`, `modified_at`, `completed_at`, `liked` and `num_likes`, and `completed` and `assignee` are always present rather than omitted when empty. No key was removed or changed type, so existing parsers keep working.

- **JSON output no longer HTML-escapes** — `<` and `>` come through as themselves, which is what makes an `html_notes` value readable. Semantically identical JSON.

- **Agent skill docs** — rich-text notes and honest non-interactive behavior, assignee precedence and the unassigned flags, the `CREATE_TASK` step order and its missing guard rails, the transport boundary and what switching costs, a warning against `asana upgrade` over a local build, and the `ASANA_TOKEN` override including the config-file gap it does not yet close.

### Known limitations

- **`ASANA_TOKEN` alone cannot bootstrap a fresh machine** (`asana-cli-19k`, fixed after this release) — the override removes the keyring dependency but not the interactive-login dependency. A default workspace still lives only in `~/.config/asana-cli/config.yaml` and only `auth login` writes that file, so on a machine that has never logged in the override gets you as far as `--version` and `--help`, but a command that talks to Asana still needs that file copied or hand-written. Supplying the workspace from the environment is not yet possible.

## [3.3.2] - 2026-07-09

### Fixed

- **`projects tasks` and `projects list` no longer 400 in large workspaces** — resolving a project by name previously enumerated *every* project in the workspace, and because `Options.Limit` is `omitempty`, a `limit=0` "no cap" dropped the limit param entirely and sent an unbounded request that Asana rejected with `400: The result is too large`. `FetchAllProjects` now always requests a bounded page (≤100) and treats `limit` purely as a total cap; `projects tasks <name>` resolves via the typeahead API (numeric IDs fetched directly by gid), the same ceiling-free path `projects list -q` already used. Full enumeration is now used only for interactive selection.

### Changed

- **Agent skill docs** — added a section-membership workflow with an explicit section-vs-assignee disambiguation (a section *named* after a person is not the same as tasks *assigned to* that person), the board-view caveat for `--sections`, and an "Answering read queries honestly" discipline against silently substituting a proxy query when the intended one fails.

## [3.3.1] - 2026-06-23

### Fixed

- **Missing default workspace no longer crashes commands** — commands that need a workspace now show a friendly setup error when the config has no default workspace instead of panicking on a nil `cfg.Workspace`.

### Changed

- **Release docs** — captured GoReleaser upload and annotated-tag gotchas in the repo-local publish release skill.

## [3.3.0] - 2026-06-19

### Added

- **`tasks comments` subcommand** — read a task's comments from the CLI: `asana tasks comments <task-id>`. Fetches the task's stories, filters to comment-type stories, and prints each comment's author, timestamp, and text. Supports `--json` for scripting and agent use. (`asana tasks view` returns only the task object and never included comments.)

### Changed

- **Claude plugin** — updated `argument-hint` for consistency across commands
- **Docs** — cited PR #13 and beads-2z7 in the v4 safety & workflow design spec

## [3.2.0] - 2026-05-03

### Added

- **`projects sections create` subcommand** — create a new section in a project from the CLI: `asana projects sections create "Project Name" "Section Name"` (supports `--json` for scripting and agent use)

### Changed

- **CI** — keep Claude plugin marketplace and `plugin.json` versions in lockstep
- **Docs** — added v4 safety & workflow design spec; widened v4 retrofit scope to include `auth login --workspace`

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
