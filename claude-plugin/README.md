# Asana CLI Plugin for Claude Code

Manage Asana tasks, projects, and users directly from Claude Code conversations.

## Prerequisites

The `asana` CLI must be installed and authenticated:

```bash
asana auth login       # One-time setup
asana auth status      # Verify it's working
```

## Installation

```bash
# From the marketplace
/plugin marketplace add jtsternberg/asana-cli
/plugin install asana-cli

# Or from a local clone
claude plugins add /path/to/asana-cli
```

## What's Included

### Skills

| Skill | Description |
|-------|-------------|
| `using-asana-cli` | Full command reference for all `asana` CLI operations |
| `troubleshooting-asana` | Diagnoses auth failures, API errors, and CLI issues |

### Commands

| Command | Description |
|---------|-------------|
| `/asana-create-task` | Create a new task with full metadata |
| `/asana-update-task` | Update an existing task by ID |
| `/asana-delete-task` | Permanently delete a task |
| `/asana-move-task` | Move a task to a different project/section |

### Agent

| Agent | Description |
|-------|-------------|
| `asana-task-manager` | Autonomous agent for end-to-end task management |

## Usage Examples

```
/asana-create-task Buy birthday cake for the team, assign to me in the Fun Committee project
/asana-update-task 1234567890 change due date to tomorrow and assign to Tom
/asana-move-task 1234567890 to Outgoing Tasks in the Tom section
/asana-delete-task 1234567890
```

## Permissions

The plugin pre-approves `asana` CLI commands so you don't get prompted for each one. See `settings.json` for the full allow list.
