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
asana tasks view
```

### List your tasks

```bash
asana tasks list
```

### Search tasks

```bash
asana tasks search
```

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

All name-based flags (assignee, project, section, followers) support:
1. **Exact match** (case-insensitive)
2. **Partial/contains match** (case-insensitive)
3. **ID match** (Asana GID)

For example, `-a "Chris"` will match "Chris Christoff" if no exact "Chris" exists.

## Common Patterns

### Create a task and add collaborators

```bash
asana tasks create \
  -n "Review PR #42" \
  -a me \
  -p "Engineering" \
  -f "Alice,Bob" \
  -d tomorrow
```

### Batch update: complete multiple tasks

```bash
for id in 123 456 789; do
  asana tasks update "$id" --complete
done
```

### Discover sections before creating a task

```bash
asana projects sections "My Project"
# Then use the section name in create:
asana tasks create -n "..." -a me -p "My Project" -s "Sprint 5"
```
