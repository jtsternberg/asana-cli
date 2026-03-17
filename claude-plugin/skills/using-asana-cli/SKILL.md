---
name: using-asana-cli
description: Manages Asana tasks, Asana projects, and Asana workspace users via the `asana` CLI. Use when the user explicitly mentions Asana or uses `asana` commands.
allowed-tools: Bash(asana *)
user-invocable: false
---

# Asana CLI

Manage Asana tasks, Asana projects, and Asana workspace members from the command line using the `asana` CLI. All commands support non-interactive mode for scripting and agent use.

This skill only applies when the user is working with Asana specifically.

## Prerequisites

Verify authentication before running commands:

```bash
asana auth status
```

If not authenticated, run `asana auth login` and follow the prompts.

## Task Management

### Create a task (non-interactive)

Provide `--name`, `--assignee`, and `--project` to skip all prompts:

```bash
asana tasks create \
  -n "Task name" \
  -a "Assignee Name" \
  -p "Project Name" \
  -s "Section Name" \
  -d "2026-04-01" \
  -m "Task description" \
  -f "Follower One,Follower Two"
```

**Flags:**
| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--name` | `-n` | Yes* | Task name |
| `--assignee` | `-a` | Yes* | Assignee name, ID, or `me` |
| `--project` | `-p` | Yes* | Project name or ID |
| `--section` | `-s` | No | Section name or ID (defaults to first section) |
| `--due` | `-d` | No | Due date: `YYYY-MM-DD`, `today`, `tomorrow` |
| `--description` | `-m` | No | Task description |
| `--followers` | `-f` | No | Comma-separated follower names or IDs |
| `--non-interactive` | | No | Explicitly prevent prompts; errors on missing required fields |

*Required in non-interactive mode. When all three are provided, non-interactive mode is auto-detected.

Without flags, the command falls back to interactive prompts.

### Update a task (non-interactive)

Pass a task ID as the first argument to use flags:

```bash
asana tasks update <task-id> \
  -n "New name" \
  -d "2026-04-01" \
  -a "New Assignee" \
  -f "Follower Name" \
  --complete
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | New task name |
| `--description` | `-m` | New description |
| `--due` | `-d` | New due date |
| `--assignee` | `-a` | New assignee name or `me` |
| `--followers` | `-f` | Comma-separated follower names to add |
| `--complete` | | Mark task as completed |
| `--non-interactive` | | Explicitly prevent prompts |

Without a task ID, falls back to interactive mode.

### Delete a task

```bash
asana tasks delete <task-id>
```

### View a task

```bash
asana tasks view <task-id>
```

Without a task ID, falls back to interactive selection.

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

```bash
asana tasks list [--sort due|due-desc|asc|desc|created-at] [--limit 20] [--user me] [--json]
```

### Search tasks

Flexible search across all tasks in the workspace.

```bash
# Tasks assigned to me
asana tasks search --assignee me

# Tasks I created (regardless of assignee)
asana tasks search --creator me

# Keyword search with limit
asana tasks search --query "deploy" --limit 5

# Blocked tasks due this week
asana tasks search --is-blocked --due-on-after 2026-03-09 --due-on-before 2026-03-13
```

**Search flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--query` | `-q` | Full-text search on task names and descriptions |
| `--assignee` | `-a` | Comma-separated assignee IDs or `me`. Omit to search all |
| `--creator` | | Comma-separated creator IDs or `me` |
| `--limit` | `-l` | Limit number of results |
| `--sort-by` | | Sort by: `due_date`, `created_at`, `completed_at`, `likes`, `modified_at` (default: `modified_at`) |
| `--sort-asc` | | Sort ascending (default is descending) |
| `--due-on` | | Tasks due on exact date (`YYYY-MM-DD`) |
| `--due-on-before` | | Tasks due before date |
| `--due-on-after` | | Tasks due after date |
| `--is-blocked` | | Only tasks with incomplete dependencies |
| `--tags-all` | | Comma-separated tag IDs to filter by |
| `--type` | | Resource subtype: `default_task`, `milestone` (default: `default_task`) |
| `--exclude-assignee` | | Comma-separated user IDs to exclude |
| `--exclude-creator` | | Comma-separated creator IDs to exclude |
| `--json` | | Output as JSON |

**Note:** `--assignee` has no default — omit it to search across all assignees.

## Structured Output

All task commands (`list`, `search`, `view`) support `--json` for machine-readable output. Pipe the output to `jq` for filtering and transformation:

```bash
# Get all task IDs from search results
asana tasks search --query "deploy" --json | jq '.[].gid'

# Get task names and IDs
asana tasks list --json | jq '.[] | {gid, name}'

# Filter tasks by name pattern (case-insensitive)
asana tasks list --json | jq '.[] | select(.name | test("keyword"; "i"))'

# Extract a single field from a specific task
asana tasks view <task-id> --json | jq '.name'
```

Task IDs are also shown in the default text output of `list` and `search` (e.g., `(ID: 1234567890)`).

## Project Management

### List projects

```bash
asana projects list -l 20        # Limit to 20
asana projects list -f           # Favorites only
asana projects list -s asc       # Sort ascending
```

### List sections in a project

```bash
asana projects sections "Project Name"
```

### List tasks in a project

```bash
asana projects tasks             # Interactive project selection
asana projects tasks --sections  # Group by section
```

## Users

### List workspace users

```bash
asana users list
```

## Name Matching

Name flags support exact, partial, and ID matching (case-insensitive).

## Verification

After creating or updating a task, verify by checking the returned output or running:

```bash
asana tasks view <task-id>
```
