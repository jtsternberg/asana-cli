# Creating a Task

## Steps

1. Parse the request for: task name, assignee, project, section, due date, description, and followers
2. If project is named, search for it: `asana projects list -q "Project Name"`
   - Only fall back to `asana projects list -l 20` if no name was given and you need to show options
3. If section is unknown, discover options: `asana projects sections "Project Name"`
4. If assignee is unclear, discover options: `asana users list`
5. **Decide how the description is formatted before you build the command.** If it contains a bulleted list, a hyperlink, bold text, or headings, it needs `--markdown-notes` — `-m` would leave the markup literal. See "Descriptions" below.
6. Create the task:

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

7. **Verify the output** — confirm the success message includes all expected fields (name, assignee, due date, followers, URL). If a field is missing, investigate.

Unsure a name will match, or want to see what your Markdown became? Add `--dry-run` first. It resolves assignee, project and section for real and prints the exact request body without creating anything, then re-run without the flag.

## Non-interactive mode

`--non-interactive` is in the example on purpose. It is **not** strictly required — the CLI also skips prompts when stdin is not a terminal (always the case from a tool call) or when `--name`, `--assignee` and `--project` are all supplied. But pass it anyway: it makes the intent explicit and does not depend on how the calling shell wired stdin.

With prompts off, required values must come from flags. A missing one is a clear error (`--assignee is required in non-interactive mode`). Optional values — due date, description — are left unset, not prompted for.

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
- If a name doesn't match, use list commands to discover the correct value
- If section is not found, run `asana projects sections "Project"` and suggest alternatives
- `Error: <p> is not allowed in Asana html notes...` means your `--html-notes` used an element Asana rejects — this is caught locally, before any request, so nothing was created. Fix the markup, or switch to `--markdown-notes` and let the CLI generate valid markup.
- After creation, read the output carefully — don't claim success unless the output confirms it. Rich-text descriptions print a `Description: rich text (markdown, N chars)` line; no line means no rich text was sent.
