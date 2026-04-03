# Updating a Task

## Steps

1. Extract the task ID (if a URL, parse the numeric ID from it)
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

4. **Verify the output** — confirm the success message includes all expected changes. If a field is missing, investigate.

## Guard rails

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest the closest match
- After updating, read the output carefully — don't claim success unless the output confirms it
