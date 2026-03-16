---
allowed-tools: Bash(asana *)
argument-hint: <task-id> [destination project/section]
description: Move an Asana task to a different project and/or section
---

Move Asana task based on: $ARGUMENTS

1. Extract the task ID from the first argument (if a URL, parse the ID from it)
2. Parse the destination: project name, section name
3. If project is unknown, list available projects: `asana projects list -l 20`
4. If section is unknown, list sections: `asana projects sections "Project Name"`
5. Run the move:

```bash
asana tasks move <task-id> \
  -p "Project Name" \
  [-s "Section Name"] \
  [--keep]
```

6. Report the result to the user

Always use `tasks move` instead of deleting and recreating — it preserves task history, comments, followers, and attachments.

If task not found, ask the user to verify the ID. If project/section not found, use list commands to suggest alternatives.
