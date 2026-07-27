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

> **Do not run `asana upgrade` if the version has a `-g<sha>` suffix** (e.g. `v3.3.2-10-g117d35c`) or reads `dev`. That means a locally built binary, usually newer than any release. `asana upgrade` downloads the latest *release* and overwrites it, so you would silently move **backwards** and lose whatever you were testing. Verify a suspected-stale binary with `asana tasks create --help` instead — if the flag you need is listed, the binary is fine.

## 3. Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `unknown flag: --name` | Running upstream v1.2.0 (no flag support) | Install the fork: `asana upgrade --yes` |
| `could not prompt: EOF` / `failed to read due date: could not prompt: EOF` | An interactive prompt in a non-TTY context. Fixed: prompts are now skipped whenever stdin is not a terminal, and optional prompts resolve to blank rather than failing. Seeing this means an **old build** | `asana upgrade --yes`, or rebuild from source (§4). Meanwhile, add `--non-interactive` |
| `a task ID is required when not running interactively` | `asana tasks update` with no task ID and no terminal to pick one from | Find the ID with `asana tasks search` / `asana tasks list --json`, then pass it as the first argument |
| `<p> is not allowed in Asana html notes` (or `<br>`, `<div>`, `<h3>`, ...) | `--html-notes` used an element outside Asana's allowlist. Caught locally — **nothing was created or changed** | Remove the element, or switch to `--markdown-notes` and let the CLI generate valid markup |
| `html notes are not well-formed XML` | Usually an unclosed tag, or `<hr>`/`<img>` without the XML slash in a spot the auto-repair missed | Close every tag; self-close voids (`<hr/>`). Or use `--markdown-notes` |
| `if any flags in the group [description html-notes markdown-notes] are set none of the others can be` | More than one description form was passed | A task has one description — pick `-m`, `--markdown-notes`, or `--html-notes` |
| Description shows literal `**bold**` or `[text](url)` in Asana | Markdown was passed to `-m`, which is the plain-text field | Re-run with `--markdown-notes` instead |
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
