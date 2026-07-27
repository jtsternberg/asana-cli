---
name: asana-task-manager
description: Manages Asana tasks, Asana projects, and Asana workspace users via the `asana` CLI. Use when the user mentions Asana, asks to create/update/move/delete tasks, search projects, or manage workspace users.
argument-hint: '[create|update|move|delete|search|list] [natural language description]'
allowed-tools: 'Bash(asana *), Bash(which asana), Bash(security find-generic-password *)'
---

# Asana CLI

Manage Asana tasks, Asana projects, and Asana workspace members from the command line using the `asana` CLI. All commands support non-interactive mode for scripting and agent use.

This skill only applies when the user is working with Asana specifically.

## Operation-specific workflows

**You MUST read the corresponding reference file(s) before performing any operation.** These contain the exact steps, guard rails, and gotchas for each action. Do not skip this step.

| Operation | Reference | Read it BEFORE you... |
|-----------|-----------|----------------------|
| Create | `references/CREATE_TASK.md` | ...run `asana tasks create` |
| Update | `references/UPDATE_TASK.md` | ...run `asana tasks update` |
| Move | `references/MOVE_TASK.md` | ...run `asana tasks move` |
| Delete | `references/DELETE_TASK.md` | ...run `asana tasks delete` |
| Troubleshoot | `references/TROUBLESHOOTING.md` | ...tell the user something is broken |

## Prerequisites

Verify authentication before running commands:

```bash
asana auth status
```

If not authenticated, run `asana auth login` and follow the prompts.

## Task Management

### Create a task (non-interactive)

Prompts are skipped when **any** of these holds:

1. **stdin is not a terminal** — which is always true when you run the CLI from a tool call
2. `--non-interactive` was passed
3. `--name`, `--assignee` and `--project` were all supplied

Required values must then come from flags (a missing one is an error naming the flag). Optional ones — due date, description — are simply left unset rather than prompted for.

**Pass `--non-interactive` anyway.** It costs nothing, states the intent, and does not depend on how the calling shell happened to wire stdin. Every example in `references/CREATE_TASK.md` includes it.

```!
asana tasks create --help
```

### Update a task (non-interactive)

Pass a task ID as the first argument to use flags. A task ID is **required** whenever prompts are unavailable (no terminal on stdin, or `--non-interactive`) — without one the command fails with a message saying so, rather than hanging or erroring on EOF.

```!
asana tasks update --help
```

### Rich text descriptions — links, lists, bold

`--description`/`-m` sets the **plain text** `notes` field. Markdown in it stays literal: `**bold**` renders as asterisks and `[text](url)` renders as brackets. There are real tasks in this workspace that look like that; don't add more.

For anything with a hyperlink, a bulleted list, or emphasis, use one of these instead. All three description forms are mutually exclusive.

| Flag | Input | Use when |
|------|-------|----------|
| `--markdown-notes` | Markdown | **Default choice for rich text.** It's what you already write. |
| `--html-notes` | Asana-flavored HTML | You need exact control, or an `@`-mention. |
| `--description` / `-m` | Plain text | The description genuinely has no formatting. |

Both rich-text flags accept the value three ways, because HTML and multi-line Markdown are painful as shell arguments:

```bash
# Inline
asana tasks create -n "Task" -a "Chris" -p "Outgoing Tasks" --non-interactive \
  --markdown-notes "Two things:

- The **build** is green again
- Details are [in slack](https://example.slack.com/archives/C1/p2)"

# From a file — best for anything long; write the file, then pass @path
asana tasks create -n "Task" -a "Chris" -p "Outgoing Tasks" --non-interactive \
  --markdown-notes @/tmp/notes.md

# From stdin
generate-notes | asana tasks update 1234567890 --markdown-notes -
```

#### Markdown → Asana conversion

Supported: `#`/`##` headings, `-`/`*`/`+` and `1.` lists (including nesting), `**bold**`, `*italic*`, `` `code` ``, `~~strike~~`, `[text](url)`, `<https://autolink>`, `> blockquotes`, fenced code blocks, and `---`.

Not supported, because Asana has no element for them: **tables**, images, footnotes, reference-style links. `###` and deeper are demoted to `<h2>`. Raw HTML in Markdown input is escaped, not passed through. A **bare URL stays plain text** — wrap it in `<…>` or use `[text](url)` if you want it clickable.

A single newline stays a line break and a blank line separates blocks, matching how Asana's own editor stores descriptions.

#### Asana's html_notes rules (only relevant for `--html-notes`)

The value must be well-formed **XML** with a single `<body>` root. Allowed elements, and nothing else:

`body` `strong` `em` `u` `s` `code` `ol` `ul` `li` `a` `blockquote` `pre` `h1` `h2` `hr` `img`

Only `<a>` and `<img>` may carry attributes. Note what is **absent**: no `<p>`, no `<br>`, no `<div>`, no `<h3>`, no `<span>`. Any of them is a 400 from Asana.

The CLI checks all of this locally before sending anything, and names the offending element:

```
Error: <p> is not allowed in Asana html notes; allowed elements are: a blockquote body code em h1 h2 hr img li ol pre s strong u ul
```

It also repairs what is unambiguous: a bare fragment is wrapped in `<body>`, `<hr>` becomes `<hr/>` (XML needs the slash), and a stray `&` becomes `&amp;`. Anything else you must fix yourself.

`<a data-asana-gid="123"/>` expands to an @-mention of user/task/project 123.

### Delete a task

```!
asana tasks delete --help
```

### View a task

```!
asana tasks view --help
```

### View task comments

`asana tasks view` returns only the task object — it does **not** include comments. To read the comments (the discussion thread) on a task, use `asana tasks comments <task-id>`. It fetches the task's stories, filters to comment-type stories, and prints each comment's author, timestamp, and text. Supports `--json`.

```!
asana tasks comments --help
```

```bash
# Read the comments on a task
asana tasks comments 1234567890

# Machine-readable: author + text + created_at per comment
asana tasks comments 1234567890 --json | jq '.[] | {author, created_at, text}'
```

### List vs Search

Use **`tasks list`** for a quick view of tasks assigned to a user. Use **`tasks search`** for anything more flexible — filtering by creator, tags, blocked status, date ranges, or keyword.

```bash
# "My tasks" (assigned to me) — use list
asana tasks list

# "Tasks I created" — use search
asana tasks search --creator me

# "Tasks assigned to me about X" — use search
asana tasks search --assignee me --query "X"
```

### List tasks

Lists tasks assigned to a user (defaults to `me`). Cannot filter by creator — use `search` for that.

```!
asana tasks list --help
```

### Search tasks

Flexible search across all tasks in the workspace. **Note:** `--assignee` has no default — omit it to search across all assignees.

```!
asana tasks search --help
```

## Structured Output

Most commands support `--json` for machine-readable output:
- **Tasks:** `list`, `search`, `view`, `comments`
- **Projects:** `list`, `sections`, `tasks`
- **Users:** `list`
- **Teams:** `list`
- **Tags:** `list`
- **Workspaces:** `list`
- **Time:** `status`, `create`

JSON output includes all available fields from the API (assignee, completion status, custom fields, dates, etc.). Pipe to `jq` for filtering:

```bash
# Get all task IDs from search results
asana tasks search --query "deploy" --json | jq '.[].id'

# Get task names and assignees
asana tasks list --json | jq '.[] | {id, name, assignee: .assignee.name}'

# Find incomplete tasks
asana tasks list --json | jq '.[] | select(.completed == false)'

# Get tasks with specific custom field values
asana tasks view <task-id> --json | jq '.custom_fields[] | {name, display_value}'

# Filter tasks by name pattern (case-insensitive)
asana tasks list --json | jq '.[] | select(.name | test("keyword"; "i"))'

# Find a user by email
asana users list --json | jq '.[] | select(.email | test("tom"; "i"))'
```

Text output also includes rich data: task list/search show assignee, due date, projects, and completion status alongside the task name and ID.

## Project Management

### List projects

**Important:** The workspace may have hundreds of projects. Without `--search`/`-q`, only the first 100 are returned. Always use `--search` when looking for a specific project by name.

```!
asana projects list --help
```

### List sections in a project

```!
asana projects sections --help
```

### Create a section in a project

```!
asana projects sections create --help
```

### List tasks in a project

```!
asana projects tasks --help
```

Accepts a project **name** or **numeric ID**. Both resolve efficiently — name lookups use the typeahead API (no project-count ceiling), so this works even in workspaces with thousands of projects.

### List tasks in a specific SECTION of a project

**Sections are not assignees.** A project can have a section *named after a person* (e.g. a "Tom" section) that is completely independent of who its tasks are assigned to. "Tasks in the Tom section" and "tasks assigned to Tom" are **different questions with different answers** — do not substitute one for the other.

To get the tasks that belong to a section, use `--sections` (groups tasks by their real section membership), then filter by section name:

```bash
# Tasks that live in the "Tom" section (section membership — NOT assignee)
asana projects tasks "Outgoing Tasks" --sections --json \
  | jq '.[] | select(.section == "Tom") | .tasks'

# Count them
asana projects tasks "Outgoing Tasks" --sections --json \
  | jq '.[] | select(.section == "Tom") | .tasks | length'
```

Contrast with assignee filtering, which ignores sections entirely:

```bash
# Tasks ASSIGNED to Tom anywhere in the project (may span many sections,
# and may miss Tom-section tasks assigned to someone else or unassigned)
asana tasks search --project <project-id> --json \
  | jq '.[] | select(.assignee.id == "<tom-gid>")'
```

**Caveat — `--sections` uses a board-view endpoint.** Section-scoped task listing relies on Asana's `sections/{id}/tasks` endpoint, which is populated for **board-layout** projects. On a list-layout project a section may return no tasks even when the Asana web UI shows some. If `--sections` yields empty sections that you know are non-empty, say so explicitly rather than silently falling back to an assignee filter — see "Answering read queries honestly" below.

## Users

### List workspace users

```!
asana users list --help
```

## Teams

### List teams

```!
asana teams list --help
```

## Tags

### List tags

```!
asana tags list --help
```

## Workspaces

### List workspaces

```!
asana workspaces list --help
```

## Time Tracking

### Log time on a task

```!
asana time create --help
```

### View time entries

```!
asana time status --help
```

## Name Matching

Name flags support exact, partial, and ID matching (case-insensitive).

## Translation Layer

When the user describes an action in natural language, translate it to the correct CLI flags:

| User says | CLI equivalent | Notes |
|-----------|---------------|-------|
| "CC Chris on this" / "add Chris to the task" / "loop in Chris" | `--followers "Chris"` or `--cc "Chris"` | `--cc` is a hidden alias for `--followers` |
| "due today" / "this is due today" | `--due today` | **NEVER** pre-resolve to a date string — pass the literal keyword |
| "due tomorrow" | `--due tomorrow` | Same rule: pass the keyword, not a computed date |
| "due next Friday" | `--due 2026-04-03` | CLI only supports `today`, `tomorrow`, or `YYYY-MM-DD` — you must compute this one |
| "assign to me" / "I'll take this" | `--assignee me` | |
| "assign to Chris" | `--assignee "Chris"` | Name matching works on create, update, AND search |
| "find Tom's tasks" / "search Tom's stuff" | `--assignee "Tom"` | Search resolves names to IDs automatically |
| "tasks in Tom's section" / "the Tom column" / "what's in the Tom section" | `projects tasks <project> --sections` + filter by section name | A **section** named after a person is NOT the same as its assignee. See "List tasks in a specific SECTION". Do not translate this to `--assignee`. |
| "find the outgoing project" / "which project is X in?" | `asana projects list -q "outgoing"` | Uses typeahead API — no 100-project ceiling |
| "mark it done" / "complete this" | `--complete` | Update command only |
| "move it to Project X" | `asana tasks move <task-id>` | Don't delete and recreate |
| description has a list, a link, or bold text | `--markdown-notes "..."` | **Not** `-m`. `-m` is plain text and leaves `**bold**` literal. |
| "link the words X to this URL" | `--markdown-notes "... [X](url) ..."` | Anchoring a link on specific words needs rich text |
| "@-mention Chris in the description" | `--html-notes '<body>... <a data-asana-gid="GID"/> ...</body>'` | Get the GID from `asana users list --json` |

**Critical rule:** For `--due today` and `--due tomorrow`, ALWAYS pass the keyword literally. The CLI resolves it using `time.Now()` on the local machine, which is more reliable than the agent computing a date from session context (which may be stale or in a different timezone).

## Answering read queries honestly

Reads deserve the same rigor as writes. When you search, filter, or count tasks:

1. **Answer the question that was asked.** If you were asked for a *section's* tasks, return section membership — not a proxy like "tasks assigned to the person the section is named after." If the two happen to differ, that difference is the point.
2. **Never silently swap the query.** If the intended approach fails (an endpoint errors, returns empty, or hits a limit), STOP and say what failed. Do not quietly substitute a different filter and present its results as if they answered the original question. A wrong-but-confident count is worse than "I couldn't get that directly — here's what I tried."
3. **Don't present a proxy count as the real count.** "24 tasks in the Tom section" and "26 tasks assigned to Tom" are different numbers answering different questions. Label which one you actually computed.
4. **Watch for silent truncation.** `--limit N` caps totals; if a result set is exactly N, more may exist. State when a count could be truncated.
5. **Large workspace? Resolve by name or ID directly** (`projects tasks`, `projects list -q`) — these use the typeahead API and won't hit the "result is too large" 400 that unbounded enumeration triggers.

## Post-Mutation Verification

After ANY create, update, or delete operation, you MUST verify the result:

1. **Read the CLI output carefully** — it confirms what was actually set (name, assignee, due date, followers, URL)
2. **Check for missing fields** — if you requested a due date but the output doesn't show one, the operation failed silently
3. **Due date keyword confirmation** — when you pass `--due today`, the output shows the resolved date with the keyword in parentheses, e.g. `Due: Apr 1, 2026 (today)`. Verify this matches your intent.
4. **Rich text confirmation** — when you pass `--markdown-notes` or `--html-notes`, the output includes a `Description: rich text (markdown, 163 chars)` line. No such line means the description was not set as rich text.
5. **Never claim success based on vibes** — if the output doesn't confirm a field was set, it wasn't. Check the receipts.

If something looks wrong, run `asana tasks view <task-id>` to get the full task state.
