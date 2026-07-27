# <img src=".github/assets/Asana-avatar.png" alt="Asana" height="36" /> Asana CLI

A command-line interface for managing Asana tasks, projects, time tracking, and more — with both interactive and non-interactive (scriptable) modes.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/jtsternberg/asana-cli)](https://goreportcard.com/report/github.com/jtsternberg/asana-cli)

> [!NOTE]
> This is a maintained fork of [timwehrle/asana](https://github.com/timwehrle/asana) (archived upstream, May 2026) with extended features: non-interactive CLI support, `--json` output, `tasks delete`, `projects sections` (list + create), fuzzy name matching, self-update command, and a Claude Code plugin.

## Features

- **Interactive and non-interactive modes** — use interactively with prompts, or pass flags for scripting and CI/CD
- **Structured JSON output** — `--json` on `create`, `list`, `search`, and `view`, all in one identical task shape, for piping into `jq` or other tools
- **Fuzzy name matching** — assignee, project, section, and follower flags match by partial name, exact name, or Asana GID
- **Task CRUD** — create, view, update, delete, list, and search tasks
- **Project management** — list projects, sections, and tasks (optionally grouped by section)
- **Time tracking** — log time, check status, delete entries
- **Teams, users, and tags** — list and filter workspace members and tags
- **Self-update** — `asana upgrade` detects your install method and updates in-place with checksum verification
- **Secure credential storage** — system keyring integration (macOS, Linux, Windows, WSL2), with an `ASANA_TOKEN` environment override for headless and containerized runs
- **Claude Code plugin** — AI-assisted task management with skills, commands, and an autonomous agent

## Installation

### Pre-built Binaries

Download the latest binary for your platform from the [releases page](https://github.com/jtsternberg/asana-cli/releases).

### Script Install

```bash
curl -sSL https://raw.githubusercontent.com/jtsternberg/asana-cli/main/scripts/install.sh | bash
```

### From Source

```bash
git clone https://github.com/jtsternberg/asana-cli.git
cd asana-cli
go install ./cmd/asana
```

<details>
<summary>WSL2 keyring setup</summary>

If you run into keyring issues on WSL2, see [this workaround](https://github.com/XeroAPI/xoauth/issues/25#issuecomment-2364599936). A setup script is also provided:

```bash
chmod +x scripts/setup-wsl-keyring.sh
./scripts/setup-wsl-keyring.sh
```

</details>

## Quick Start

```bash
asana auth login                       # Authenticate with your Personal Access Token
asana tasks list                       # List your tasks
asana tasks create -n "Ship it" -a me -p "My Project"  # Create a task (no prompts)
asana tasks search --query "deploy"    # Search across tasks
asana tasks view <task-id> --json      # View task details as JSON
asana upgrade                          # Update to the latest version
```

## Authentication

1. Get a Personal Access Token from Asana (Settings > Apps > Developer Apps)
2. Run `asana auth login` and follow the prompts
3. Check status with `asana auth status`

## Configuration

```bash
asana config set default-workspace     # Set default workspace
asana config get dw                    # Get current workspace (dw is shorthand)
```

## Task Management

### Create

All commands support both interactive and non-interactive modes. Provide flags to skip prompts:

```bash
asana tasks create \
  -n "Task name" \
  -a "Assignee Name" \
  -p "Project Name" \
  -s "Section Name" \
  -d "2025-04-01" \
  -m "Task description" \
  -f "Follower One,Follower Two"
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--name` | `-n` | Yes* | Task name |
| `--assignee` | `-a` | Yes* | Assignee name, ID, or `me` |
| `--project` | `-p` | Yes* | Project name or ID |
| `--section` | `-s` | No | Section name or ID (defaults to first section) |
| `--due` | `-d` | No | Due date: `YYYY-MM-DD`, `today`, `tomorrow` |
| `--description` | `-m` | No | Task description |
| `--followers` | `-f` | No | Comma-separated follower names or IDs |
| `--non-interactive` | | No | Explicitly prevent prompts; errors on missing fields |

\*Required in non-interactive mode. When all three are provided, non-interactive mode is auto-detected.

Without flags, the command falls back to interactive prompts.

### Update

Pass a task ID to use flags, or omit for interactive mode:

```bash
asana tasks update <task-id> \
  -n "New name" \
  -d "2025-04-01" \
  -a "New Assignee" \
  -f "Follower Name" \
  --complete
```

| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | New task name |
| `--description` | `-m` | New description |
| `--due` | `-d` | New due date |
| `--assignee` | `-a` | New assignee name or `me` |
| `--followers` | `-f` | Comma-separated follower names to add |
| `--complete` | | Mark task as completed |
| `--non-interactive` | | Explicitly prevent prompts |

### View, List, Search, and Delete

```bash
asana tasks view <task-id>              # View task details (or omit ID for interactive)
asana tasks comments <task-id>          # Read a task's comments (author + text + time)
asana tasks list                        # List all your tasks
asana tasks list --sort due-desc        # Sort by descending due date
asana tasks search --assignee me        # Search your assigned tasks
asana tasks search --query "deploy" -l 5  # Search with limit
asana tasks search --creator me         # Search tasks you created
asana tasks delete <task-id>            # Delete a task by ID
```

Task IDs are shown in `list` and `search` output for easy use with other commands.

### JSON Output

All task commands (`create`, `list`, `search`, `view`, `comments`) support `--json` for machine-readable output:

```bash
asana tasks create -n "Ship it" -a me -p "Outgoing Tasks" --json  # The created task
asana tasks list --json                 # JSON array of tasks
asana tasks search --query "bug" --json # Search results as JSON
asana tasks view <task-id> --json       # Full task details as JSON
asana tasks comments <task-id> --json   # Task comments as JSON
```

`create`, `list`, `search` and `view` all emit the same task shape, so one parser
handles all of them — including creating a task and re-fetching it later:

```bash
id=$(asana tasks create -n "Ship it" -a me -p "Outgoing Tasks" --json | jq -r .id)
asana tasks view "$id" --json
```

The identifier key is `id`, not Asana's `gid`. `html_notes` carries the rich-text
form of the description. Adding `--json` to `--dry-run` prints just the request
payload, with no prose around it.

A task that has been deleted in Asana makes `asana tasks view <id>` exit 1 with
`404: task: Not a recognized ID` on stderr and nothing on stdout, which is how a
script can detect deletions after the fact.

### Name Matching

All name-based flags (assignee, project, section, followers) support case-insensitive exact matching, partial/contains matching, and Asana GID matching. For example, `-a "Chris"` will match "Chris Christoff" if no exact "Chris" exists.

## Time Tracking

```bash
asana time create -m 23 --date 2025-01-06  # Log time
asana time status                           # Check time entries
asana time delete                           # Delete a time entry
```

## Projects

```bash
asana projects list                    # List all projects
asana projects list -l 25 --sort desc  # List with options
asana projects sections "Project Name" # List sections in a project
asana projects sections create "Project Name" "Section Name"  # Create a new section
asana projects tasks                   # List tasks in a project
asana projects tasks --sections        # Group by section
```

## Teams, Users, and Tags

```bash
asana teams list                       # List all teams
asana users list                       # List all users
asana tags list                        # List all tags
asana tags list --favorite             # List favorite tags
```

Run `asana help` for all available commands.

## Upgrading

```bash
asana upgrade          # Auto-detects install method, confirms, upgrades
asana upgrade --yes    # Skip confirmation (CI-friendly)
```

The command detects whether you installed from a git clone or a pre-built binary and updates accordingly. Binary downloads are verified against SHA256 checksums.

## Claude Code Plugin

This repo includes a [Claude Code](https://claude.com/claude-code) plugin for AI-assisted Asana task management.

### Installation

In Claude Code:
```
/plugin marketplace add jtsternberg/asana-cli
/plugin install asana-cli
```

Or point to your local copy:
```
claude plugins add /path/to/asana-cli
```

### What's Included

- **Slash commands**: `/asana-create-task`, `/asana-update-task`, `/asana-delete-task`, `/asana-move-task`
- **Background skills**: `using-asana-cli` (command reference), `troubleshooting-asana` (error diagnosis)
- **Agent**: `asana-task-manager` for autonomous task management

The `asana` CLI must be installed and authenticated (`asana auth login`) before using the plugin.

## Security

Credentials are stored in your system's keyring — your Personal Access Token is never written to disk in plain text. Keyring integration works across macOS, Linux, Windows, and WSL2.

### Environment override

`ASANA_TOKEN` (or its alias `ASANA_PAT`) takes precedence over the keyring, and
when it is set the keyring is not consulted at all:

```bash
ASANA_TOKEN=1/1234:abcd asana tasks list
```

This is how to run the CLI where a keyring is absent or locked — containers, CI,
and unattended jobs on a headless box under cron or a systemd timer. `asana auth
status` reports which source the token came from, and `auth login`/`auth logout`
warn when an override is in play, since it outranks anything they store or remove.

The token is not the whole story yet: a default workspace still lives only in
`~/.config/asana-cli/config.yaml`, and only `asana auth login` writes that file.
On a machine that has never logged in, the override gets you as far as
`asana --version` and `asana --help`, but a command that talks to Asana still
needs that file — either copy one from a machine you have logged in on, or write
the workspace block by hand:

```yaml
workspace:
  id: "1234567890"
  name: "your-workspace"
```

Supplying the workspace from the environment instead is tracked as `asana-cli-19k`.

## License

MIT
