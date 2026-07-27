# Creating a Task

## Steps

1. **Parse the request** for: task name, assignee, project, section, due date, description, followers.

2. **Decide the assignee.** If the user named nobody, do not default to `me` — see "Assignee precedence" in SKILL.md. Checking the destination section's existing tasks is usually what settles it, and it's one command:
   ```bash
   asana projects tasks "Project Name" --sections --json \
     | jq -r '.[] | select(.section == "Section Name") | .tasks[] | "\(.name) :: \(.assignee.name // "unassigned")"'
   ```

3. **Decide how the description is formatted.** A bulleted list, a hyperlink, bold text, or headings means `--markdown-notes` — `-m` would leave the markup literal. See "Descriptions" below.

4. **Look things up only if you actually need to.** Every flag below takes a plain name, and the CLI resolves it (exact, then partial, case-insensitive). Skip these calls when the user gave you an exact name:

   | You need a lookup when... | Command |
   |---|---|
   | The project name is a guess, or `-p` failed to resolve | `asana projects list -q "Project Name"` |
   | You don't know the section names | `asana projects sections "Project Name"` |
   | The assignee is ambiguous | `asana users list` |
   | You need a numeric project GID (e.g. for `tasks search --project`) | `asana projects list -q "Name" --json` |

   Given an exact project name, `-p "Outgoing Tasks"` resolves on its own — a `projects list -q` call first is a wasted round trip.

5. **Rehearse with `--dry-run`.** It resolves assignee, project, section and the generated HTML for real, prints the exact request body, and creates nothing. This is the cheapest step in the list and it catches name mismatches, unavailable flags, and mis-converted markup before anything is written.

6. **Create** — same command, minus `--dry-run`:

```bash
asana tasks create --non-interactive \
  -n "Task name" \
  -a "Assignee" \
  -p "Project" \
  -s "Section" \
  -d "YYYY-MM-DD" \
  -m "Plain-text description" \
  -f "Follower1,Follower2"
```

7. **Verify the output** — confirm the success message includes every expected field (name, assignee, due date, followers, URL). If one is missing, investigate.

## What is actually required

**Only `--name`.** Asana accepts a task with no assignee and a task with no project, and so does this CLI:

```bash
# Nobody owns it yet — e.g. the target section's tasks are all unassigned
asana tasks create -n "Task name" --unassigned -p "Outgoing Tasks" -s "Untitled section"

# --assignee "" does the same thing
asana tasks create -n "Task name" -a "" -p "Outgoing Tasks"

# Workspace-level: no project, no section
asana tasks create -n "Call the bank" -a me --no-project
```

*Omitting* `--assignee` or `--project` remains an error — that guard catches the script that forgot one, rather than silently producing an ownerless or unfilable task. The error names the flag that gets you through.

Do not invent a sentinel value. `-a "none"` is **dangerous**, not merely wrong: name matching is partial and case-insensitive, so it can resolve to a real person whose name contains "none".

## Non-interactive mode

`--non-interactive` is in the example on purpose. It is **not** strictly required — the CLI also skips prompts when stdin is not a terminal (always the case from a tool call) or when `--name`, `--assignee` and `--project` are all supplied. But pass it anyway: it makes the intent explicit and does not depend on how the calling shell wired stdin.

With prompts off, required values must come from flags. Optional values — due date, description — are left unset, not prompted for.

## Descriptions

Three mutually exclusive forms. Pick one.

```bash
# Plain text — no formatting at all
-m "Just some text"

# Markdown — the default for anything with structure
--markdown-notes "Two things:

- The **build** is green again
- Details are [in slack](https://example.slack.com/archives/C1/p2)"

# Asana-flavored HTML — exact control, or @-mentions
--html-notes '<body>See <a href="https://example.com">the docs</a></body>'
```

Both rich-text flags also take `@/path/to/file` or `-` (stdin). **For anything longer than a couple of lines, write a file and pass `@path`** — it avoids shell quoting and newline problems entirely:

```bash
# write /tmp/notes.md first, then
asana tasks create --non-interactive -n "Task" -a "Chris" -p "Outgoing Tasks" \
  --markdown-notes @/tmp/notes.md
```

Markdown coverage, and the html_notes element allowlist, are documented in SKILL.md under "Rich text descriptions". The short version: no tables, no images, no `<p>`, no `<br>`, no `<h3>`; bare URLs stay unlinked.

## Guard rails

- If creation fails, check `asana auth status` first
- If a name doesn't match, use the lookup commands in step 4 to discover the correct value
- If section is not found, run `asana projects sections "Project"` and suggest alternatives
- **Before concluding the CLI cannot do something, run `asana tasks create --help`.** Guessing at flag values is not research — a previous agent guessed twice, reported the capability missing, and switched to the MCP connector for a task the CLI could have done
- **Never switch to the MCP connector because the CLI refused.** Say what wall you hit. See "Two transports reach this workspace" in SKILL.md for what a silent switch costs you
- `Error: <p> is not allowed in Asana html notes...` means your `--html-notes` used an element Asana rejects — caught locally, before any request, so nothing was created. Fix the markup, or switch to `--markdown-notes` and let the CLI generate valid markup
- After creation, read the output carefully — don't claim success unless the output confirms it. The assignee line is always present (`Assignee: unassigned` when there is none), and rich-text descriptions print `Description: rich text (markdown, N chars)`
