# Moving a Task

Always use `tasks move` instead of deleting and recreating — move preserves task history, comments, followers, and attachments.

## Steps

1. Extract the task ID (if a URL, parse the numeric ID from it)
2. Parse the destination: project name, section name
3. If project is named, search for it: `asana projects list -q "Project Name"`
   - Only fall back to `asana projects list -l 20` if no name was given
4. If section is unknown, discover options: `asana projects sections "Project Name"`
5. Run the move:

```bash
asana tasks move <task-id> \
  -p "Project Name" \
  [-s "Section Name"] \
  [--keep]
```

6. Report the result to the user

## Guard rails

- If task not found, ask the user to verify the ID
- If project/section not found, use list commands to suggest alternatives
