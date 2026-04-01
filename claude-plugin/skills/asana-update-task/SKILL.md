---
name: asana-update-task
description: Update an existing Asana task by ID (name, due date, assignee, followers, completion, description)
argument-hint: <task-id> [changes in natural language]
allowed-tools: Bash(asana *)
---

Update an Asana task based on: $ARGUMENTS

## Auth check

- Auth status: !`asana auth status 2>&1 | head -1`

## Steps

1. Extract the task ID from $0 (if a URL, parse the numeric ID from it)
2. Parse requested changes from remaining arguments: name, due date, assignee, followers, completion, description
3. Run the update:

```bash
asana tasks update <task-id> \
  [-n "New name"] \
  [-d "YYYY-MM-DD"] \
  [-a "Assignee"] \
  [-f "Follower1,Follower2"] \
  [-m "Description"] \
  [--complete]
```

4. **Verify the output** — confirm the success message includes all expected changes. If a field is missing, investigate.

## Date handling

- For "due today" → use `--due today` (NEVER pre-resolve to a date string)
- For "due tomorrow" → use `--due tomorrow`
- The output shows resolved dates with keywords: `Due: Apr 1, 2026 (today)`

## Followers / CC

- "CC someone" / "add someone to the task" → use `--followers` or `--cc`

## Guard rails

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest the closest match
- After updating, read the output carefully — don't claim success unless the output confirms it
