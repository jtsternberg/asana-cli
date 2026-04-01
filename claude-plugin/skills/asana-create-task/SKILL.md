---
name: asana-create-task
description: Create a new Asana task with full metadata (name, assignee, project, section, due date, followers)
argument-hint: [task description in natural language]
allowed-tools: Bash(asana *)
---

Create a new Asana task based on: $ARGUMENTS

## Auth check

- Auth status: !`asana auth status 2>&1 | head -1`

## Steps

1. Parse the request for: task name, assignee, project, section, due date, description, and followers
2. If project is unknown, discover options: `asana projects list -l 20`
3. If section is unknown, discover options: `asana projects sections "Project Name"`
4. If assignee is unclear, discover options: `asana users list`
5. Create the task with all available flags:

```bash
asana tasks create \
  -n "Task name" \
  -a "Assignee" \
  -p "Project" \
  -s "Section" \
  -d "YYYY-MM-DD" \
  -m "Description" \
  -f "Follower1,Follower2"
```

6. **Verify the output** — confirm the success message includes all expected fields (name, assignee, due date, followers, URL). If a field is missing, investigate.

## Date handling

- For "due today" → use `--due today` (NEVER pre-resolve to a date string)
- For "due tomorrow" → use `--due tomorrow`
- For other relative dates → compute `YYYY-MM-DD` yourself
- The output shows resolved dates with keywords: `Due: Apr 1, 2026 (today)`

## Followers / CC

- "CC someone" / "loop in someone" / "add to task" → use `--followers` or `--cc`
- `--cc` is a hidden alias for `--followers` — both work

## Guard rails

- If creation fails, check `asana auth status` first
- If a name doesn't match, use list commands to discover the correct value
- If section is not found, run `asana projects sections "Project"` and suggest alternatives
- After creation, read the output carefully — don't claim success unless the output confirms it
