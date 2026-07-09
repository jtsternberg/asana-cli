# Troubleshooting the Asana CLI

Follow this order when an `asana` command fails:

## 1. Check authentication

```bash
asana auth status
```

If this fails, re-authenticate:

```bash
asana auth login
```

## 2. Check the binary

```bash
which asana
asana --version
```

Ensure you're running the fork with non-interactive support (version should show `dev` or include `--project` flag in `asana tasks create --help`).

## 3. Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `unknown flag: --name` | Running upstream v1.2.0 (no flag support) | Install the fork: `asana upgrade --yes` |
| `could not prompt: EOF` | Interactive prompt in non-TTY context | Use flags to skip prompts (`-n`, `-a`, `-p`) |
| `The result is too large` | Unbounded project enumeration in a large workspace (fixed in-CLI for `projects tasks`/`projects list` as of the name-resolution fix). If seen on an older build: `asana upgrade --yes` | Resolve projects by name or ID directly (`projects tasks "Name"`, `projects list -q "Name"`) — these use the typeahead API and never enumerate the whole workspace |
| `section "X" not found in project` | Section name doesn't exist | Run `asana projects sections "Project Name"` to see available sections |
| `assignee "X" not found` | Name doesn't match any workspace user | Run `asana users list` to see available users; try partial name match |
| `followers: Cannot write this property` | Using followers in update request body | Followers must be added via `AddFollowers` endpoint (handled in fork) |
| `task "X" not found` | Wrong task ID | Get the task ID from the Asana URL or from `asana tasks list` |

## 4. Rebuild from source

If the binary is outdated or broken:

```bash
cd ~/Code/asana-cli
go build -o /usr/local/bin/asana ./cmd/asana
asana --version
```

Or use the upgrade command:

```bash
asana upgrade --yes
```

## 5. Keychain issues

The CLI stores tokens in the system keychain. If authentication fails after a successful login:

```bash
security find-generic-password -s "asana" -w 2>&1
```

If the token is missing, re-run `asana auth login`.
