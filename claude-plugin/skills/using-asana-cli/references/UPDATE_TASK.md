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

To *empty* a field rather than set it, see "Emptying a field" below — `--unassigned`, `--no-due`, `--no-description`, `--incomplete`, `--remove-followers`.

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

## Emptying a field, not just setting it

Anything `tasks update` can set, it can now also clear. Omitting a flag means "leave this alone"; passing it explicitly empty, or using its `--no-*` form, means "empty this".

| To do this | Use | Also spelled |
|---|---|---|
| Unassign | `--unassigned` | `-a ""` |
| Remove the due date | `--no-due` | `-d ""` |
| Empty the description | `--no-description` | `-m ""` |
| Reopen a completed task | `--incomplete` | — |
| Unfollow someone | `--remove-followers "Name"` | — |

```bash
asana tasks update 1234567890 --unassigned
asana tasks update 1234567890 --no-due --no-description
asana tasks update 1234567890 --incomplete
asana tasks update 1234567890 --remove-followers "Tom McFarlin"
```

Setting and clearing the same field in one command is rejected (`--due today --no-due`). `--dry-run` prints a `Would change: assignee cleared` line under the payload, so you can confirm the intent without decoding `"assignee": null` yourself.

Two things worth knowing:

- **A task name cannot be emptied.** Asana requires one, so `-n ""` is not a clear — it's a no-op. That's a constraint, not a gap.
- **Followers use their own endpoints**, one per direction, which is why removal is `--remove-followers` rather than an empty `--followers`.

## What `tasks update` still cannot change at all

These are absent from the command entirely — not "can set but can't clear", but simply not wired up. Each needs its own design, so don't go looking for a flag:

| Field | Why it isn't here |
|---|---|
| Start date (`start_on`) | Asana requires a due date to be present in the same request when setting or clearing it — needs paired handling |
| Due *time* (`due_at`) | Only whole-day `due_on` is exposed; a timestamp is a separate field with its own conflicts |
| Parent task | Uses `setParent`, a different endpoint |
| Removing a task from a project | `tasks move` relocates a task; there is no "remove from project and leave it nowhere" |
| Custom fields | Clearing depends on the field's type (enum vs text vs number) |
| Tags | Not exposed on any task command |

If you're asked for one of these, **say that the CLI can't do it** rather than quietly reaching for the MCP connector — see "Two transports reach this workspace" in SKILL.md. Deliberately using the connector, and saying so, is fine; switching silently is the failure mode.

## Guard rails

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest the closest match
- `Error: <p> is not allowed in Asana html notes...` is caught locally, before any request — nothing was changed. Fix the markup or switch to `--markdown-notes`.
- `Error: no updates specified` means none of the change flags were set. Clearing flags count as changes, so this no longer appears for `--unassigned`, `--no-due`, `--no-description` or `--incomplete` — if you see it with one of those, the binary predates them (`asana --version` should show `v3.3.2-18-…` or later)
- After updating, read the output carefully — don't claim success unless the output confirms it. Rich-text descriptions print a `Description: rich text (markdown, N chars)` line.
