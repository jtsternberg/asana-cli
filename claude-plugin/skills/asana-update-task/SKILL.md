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

4. Report the result to the user

## Guard rails

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest the closest match
