---
name: asana-move-task
description: Move an Asana task to a different project and/or section (preserves history, comments, attachments)
argument-hint: <task-id> [destination project/section]
allowed-tools: Bash(asana *)
---

Move an Asana task based on: $ARGUMENTS

## Auth check

- Auth status: !`asana auth status 2>&1 | head -1`

## Steps

1. Extract the task ID from $0 (if a URL, parse the numeric ID from it)
2. Parse the destination from remaining arguments: project name, section name
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

## Important

Always use `tasks move` instead of deleting and recreating a task — move preserves task history, comments, followers, and attachments.

## Guard rails

- If task not found, ask the user to verify the ID
- If project/section not found, use list commands to suggest alternatives
