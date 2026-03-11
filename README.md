<div align="center">
 <img width="300" height="300" src="https://github.com/user-attachments/assets/12bda636-36af-4d55-837d-f51fbe836ef1" alt="Asana Gopher" />

_Image from <a href="https://gopherize.me/">https://gopherize.me/</a>_

</div>

# Asana CLI

A command-line interface to manage your Asana tasks and projects directly from your terminal.

<div>
    <a href="https://pkg.go.dev/github.com/timwehrle/asana">
        <img src="https://pkg.go.dev/badge/github.com/timwehrle/asana.svg" alt="Go Reference">
    </a>
    <a href="https://github.com/timwehrle/asana/blob/main/LICENSE">
        <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
    </a>
   <a href="https://github.com/timwehrle/asana/actions/workflows/go.yml">
      <img src="https://github.com/timwehrle/asana/actions/workflows/go.yml/badge.svg" alt="Go Pipeline">
   </a>
   <a href="https://goreportcard.com/report/github.com/timwehrle/asana">
      <img src="https://goreportcard.com/badge/github.com/timwehrle/asana" alt="Go Report Card">
   </a>
</div>

# Installation

## Pre-built binaries

Download the latest binary for your platform from the [releases page](https://github.com/timwehrle/asana/releases).

## From Source

```shell
go install github.com/timwehrle/asana/cmd/asana@latest
```

## Bash Installation

```shell
curl -sSL https://raw.githubusercontent.com/timwehrle/asana/main/scripts/install.sh | bash
```

## Homebrew Installation

```shell
brew tap timwehrle/asana
brew install --formula asana
```

## Having troubles with keyrings on WSL2?

If you're running into issues with keyring access on WSL2, there's a simple workaround!
You can find a detailed explanation here: [https://github.com/XeroAPI/xoauth/issues/25#issuecomment-2364599936](https://github.com/XeroAPI/xoauth/issues/25#issuecomment-2364599936)

To make development smoother, we've also provided a setup script.
It installs the necessary packages and configures the GNOME keyring automatically. You probably have to do this every time you start your WSL2 environment.

```shell
chmod +x scripts/setup-wsl-keyring.sh
./scripts/setup-wsl-keyring.sh
```

After running the script, keyring functionality should be available in your WSL2 environment.

# Getting started

## Authentication

1. Get your Personal Access Token from Asana (Settings > Apps > Developer Apps)
2. Run the login command:
   ```shell
   asana auth login
   ```
3. Follow the prompts to paste your token and select your default workspace.

To check the current status of your authentication and the Asana API:

```shell
asana auth status
```

## Configuration

Set or get your default workspace:

```shell
asana config set default-workspace
asana config set dw

asana config get default-workspace
asana config get dw
```

## Task Management

### Create a task

All commands support both interactive and non-interactive modes. Provide flags to skip prompts:

```shell
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

### Update a task

Pass a task ID to use flags, or omit for interactive mode:

```shell
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

### View, list, search, and delete tasks

```shell
asana tasks view <task-id>         # View task details (or omit ID for interactive)
asana tasks list                   # List all your tasks
asana tasks list --sort due-desc   # Sort tasks by descending due date
asana tasks search --assignee me   # Search tasks with filters
asana tasks delete <task-id>       # Delete a task by ID
```

### Name matching

All name-based flags (assignee, project, section, followers) support case-insensitive exact matching, partial/contains matching, and Asana GID matching. For example, `-a "Chris"` will match "Chris Christoff" if no exact "Chris" exists.

## Time Tracking

```shell
asana time create -m 23 --date 2025-01-06  # Log time
asana time status                           # Check time entries
asana time delete                           # Delete a time entry
```

## Projects

```shell
asana projects list                    # List all projects
asana projects list -l 25 --sort desc  # List with options
asana projects sections "Project Name" # List sections in a project
asana projects tasks                   # List tasks in a project
asana projects tasks --sections        # Group by section
```

## Teams, Users, and Tags

```shell
asana teams list                       # List all teams
asana users list                       # List all users
asana tags list                        # List all tags
asana tags list --favorite             # List favorite tags
```

For more usage:

```shell
asana help # Show all available commands
```

# Security

To keep your Asana credentials safe, this CLI uses your system's keyring for secure token storage.
This ensures your Personal Access Token is never written to disk in plain text. The keyring integration
works across major platforms (macOS, Linux, and Windows), and includes WSL2 support with a setup script provided.

## How to improve Token Security

While keyrings are a secure option, here are some additional best practices you can consider:

- **Token Rotation**: Regularly rotate your token and avoid long-lived secrets.
- **Environment Isolation**: Avoid running this CLI in shared or untrusted environments.
- **Two-Factor Authentication (2FA)**: Enable 2FA on your Asana account to enhance account-level security.

We are also trying to implement a feature that will remind you to rotate your token every 90 (or so) days.

# Contributing

If something feels off, you see an opportunity to improve performance, or think some
functionality is missing, we’d love to hear from you! Please review our [contributing docs][contributing] for
detailed instructions on how to provide feedback or submit a pull request. Thank you!

# License

This project is licensed under the MIT License. See the [LICENSE][license] file for details.

[contributing]: ./.github/CONTRIBUTING.md
[license]: ./LICENSE
