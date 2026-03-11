# Update Asana Task

Update an existing Asana task by its Asana task ID.

## Usage

/asana-cli:update-task <task-id> [updates]

## Arguments

- `task-id` (required) - The Asana task ID or URL
- `updates` (optional) - Natural language description of changes

## Instructions

1. Extract the task ID from the argument (if a URL, parse the ID from it)
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

## Error Handling

- If task not found, ask the user to verify the ID
- If user/assignee not found, run `asana users list` and suggest closest match
