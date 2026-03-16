---
allowed-tools: Bash(asana *)
argument-hint: <task-id> [changes in natural language]
description: Update an existing Asana task by ID
---

Update Asana task based on: $ARGUMENTS

1. Extract the task ID from the first argument (if a URL, parse the ID from it)
2. Parse requested changes: name, due date, assignee, followers, completion, description
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

If task not found, ask the user to verify the ID. If user/assignee not found, run `asana users list` and suggest closest match.
