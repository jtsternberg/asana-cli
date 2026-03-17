---
name: troubleshooting-asana
description: Diagnoses and resolves Asana CLI issues. Use when `asana` commands fail or Asana authentication errors occur.
allowed-tools: Bash(asana *), Bash(which asana), Bash(security find-generic-password *)
user-invocable: false
---

# Troubleshooting the Asana CLI

## Diagnostic Steps

When an `asana` command fails, follow this order:

### 1. Check authentication

```bash
asana auth status
```

If this fails, re-authenticate:

```bash
asana auth login
```

### 2. Check the binary

```bash
which asana
asana --version
```

Ensure you're running the fork with non-interactive support (version should show `dev` or include `--project` flag in `asana tasks create --help`).

### 3. Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `unknown flag: --name` | Running upstream v1.2.0 (no flag support) | Install the fork from `~/Code/asana-cli`: `cd ~/Code/asana-cli && go build -o /usr/local/bin/asana ./cmd/asana` |
| `could not prompt: EOF` | Interactive prompt in non-TTY context | Use flags to skip prompts (`-n`, `-a`, `-p`) |
| `The result is too large` | API pagination issue | Use commands that paginate properly (e.g., `projects sections` instead of raw API) |
| `section "X" not found in project` | Section name doesn't exist in that project | Run `asana projects sections "Project Name"` to see available sections |
| `assignee "X" not found` | Name doesn't match any workspace user | Run `asana users list` to see available users; try partial name match |
| `followers: Cannot write this property` | Using followers in update request body | Followers must be added via `AddFollowers` endpoint (handled in fork) |
| `task "X" not found` | Wrong task ID | Get the task ID from the Asana URL or from `asana tasks list` |

### 4. Rebuild from source

If the binary is outdated or broken:

```bash
cd ~/Code/asana-cli
go build -o /usr/local/bin/asana ./cmd/asana
asana --version
```

Requires Go installed (`brew install go`).

### 5. Keychain issues

The CLI stores tokens in the system keychain. If authentication fails after a successful login:

```bash
# Check if the token is stored
security find-generic-password -s "asana" -w 2>&1
```

If the token is missing, re-run `asana auth login`.
