# Updating a Task

## Steps

1. Extract the task ID (if a URL, parse the numeric ID from it)
2. Parse requested changes: name, due date, assignee, followers, completion, description
3. **Decide how a new description is formatted before you build the command.** A list, hyperlink, or bold text needs `--markdown-notes`; `-m` would leave the markup literal. See "Descriptions" below.
4. Run the update:

```bash
asana tasks update <task-id> \
  [-n "New name"] \
  [-d "YYYY-MM-DD"] \
  [-a "Assignee"] \
  [-f "Follower1,Follower2"] \
  [-m "Plain-text description"] \
  [--complete]
```

5. **Verify the output** — confirm the success message lists all expected changes. If one is missing, investigate.

Add `--dry-run` to print the exact request body and change nothing — useful for checking a resolved assignee or the HTML your Markdown produced before overwriting a description.

## The task ID is required

There is no flag-driven update without a task ID. When prompts are unavailable — stdin not a terminal (always the case from a tool call), or `--non-interactive` — omitting the ID fails with:

```
Error: a task ID is required when not running interactively: asana tasks update <task-id> [flags]
```

Find the ID with `asana tasks search` or `asana tasks list --json` first.

## Descriptions

Three mutually exclusive forms; a rich-text one **replaces** the whole description.

```bash
# Plain text
asana tasks update 1234567890 -m "Just some text"

# Markdown — the default for anything with structure
asana tasks update 1234567890 --markdown-notes "Now with a [link](https://example.com) and:

- a bullet
- another"

# From a file, best for anything long
asana tasks update 1234567890 --markdown-notes @/tmp/notes.md

# From stdin
generate-notes | asana tasks update 1234567890 --markdown-notes -

# Asana-flavored HTML, incl. @-mentions
asana tasks update 1234567890 --html-notes '<body>ping <a data-asana-gid="580196049969505"/></body>'
```

Markdown coverage and the html_notes element allowlist are in SKILL.md under "Rich text descriptions". No tables, no images, no `<p>`, no `<br>`, no `<h3>`; bare URLs stay unlinked.

**Read the existing description first if you mean to amend rather than replace it** — `asana tasks view <task-id> --json | jq -r .notes`. There is no append mode.

## Known gap: update cannot clear an assignee

`tasks create` can make an unassigned task (`--unassigned`), but **`tasks update` cannot remove an existing assignee.** `-a ""` is treated as "no assignee change" and you get `Error: no updates specified`. Reassigning to a different person works fine; only clearing does not.

If you're asked to unassign a task, **say that the CLI can't do it** rather than quietly reaching for the MCP connector — see "Two transports reach this workspace" in SKILL.md. Deliberately using the connector for this, and saying so, is fine; switching silently is the failure mode.

## Guard rails

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest the closest match
- `Error: <p> is not allowed in Asana html notes...` is caught locally, before any request — nothing was changed. Fix the markup or switch to `--markdown-notes`.
- `Error: no updates specified` means none of the change flags were set
- After updating, read the output carefully — don't claim success unless the output confirms it. Rich-text descriptions print a `Description: rich text (markdown, N chars)` line.
