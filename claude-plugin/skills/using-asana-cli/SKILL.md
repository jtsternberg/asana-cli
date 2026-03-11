---
name: using-asana-cli
description: Manages Asana tasks, Asana projects, and Asana workspace users via the `asana` CLI. Use when the user explicitly mentions Asana or uses `asana` commands.
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

### List your tasks

```bash
asana tasks list [--sort due_on|created_at] [--limit 20] [--user me] [--json]
```

### Search tasks

```bash
asana tasks search --query "search term" [--assignee me] [--sort-by due_date] [--due-on 2026-04-01] [--limit 10] [--json]
```

## Structured Output

All task commands (`list`, `search`, `view`) support `--json` for machine-readable output. Use this for scripting and piping results between commands:

```bash
# Get task IDs from search results
asana tasks search --query "deploy" --json

# View a specific task as JSON
asana tasks view <task-id> --json
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
