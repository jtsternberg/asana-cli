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

Provide `--name`, `--assignee`, and `--project` to skip all prompts. When all three are provided, non-interactive mode is auto-detected. Without flags, falls back to interactive prompts.

```!
asana tasks create --help
```

### Update a task (non-interactive)

Pass a task ID as the first argument to use flags. Without a task ID, falls back to interactive mode.

```!
asana tasks update --help
```

### Delete a task

```!
asana tasks delete --help
```

### View a task

```!
asana tasks view --help
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
- **Tasks:** `list`, `search`, `view`
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
| "find the outgoing project" / "which project is X in?" | `asana projects list -q "outgoing"` | Uses typeahead API — no 100-project ceiling |
| "mark it done" / "complete this" | `--complete` | Update command only |
| "move it to Project X" | `asana tasks move <task-id>` | Don't delete and recreate |

**Critical rule:** For `--due today` and `--due tomorrow`, ALWAYS pass the keyword literally. The CLI resolves it using `time.Now()` on the local machine, which is more reliable than the agent computing a date from session context (which may be stale or in a different timezone).

## Post-Mutation Verification

After ANY create, update, or delete operation, you MUST verify the result:

1. **Read the CLI output carefully** — it confirms what was actually set (name, assignee, due date, followers, URL)
2. **Check for missing fields** — if you requested a due date but the output doesn't show one, the operation failed silently
3. **Due date keyword confirmation** — when you pass `--due today`, the output shows the resolved date with the keyword in parentheses, e.g. `Due: Apr 1, 2026 (today)`. Verify this matches your intent.
4. **Never claim success based on vibes** — if the output doesn't confirm a field was set, it wasn't. Check the receipts.

If something looks wrong, run `asana tasks view <task-id>` to get the full task state.
