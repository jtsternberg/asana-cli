---
allowed-tools: Bash(asana *)
argument-hint: <task-id>
description: Permanently delete an Asana task
---

Delete Asana task: $ARGUMENTS

1. Confirm the task ID with the user before deleting (this action is permanent and cannot be undone)
2. Run: `asana tasks delete <task-id>`
3. Report success or failure

If task not found, inform the user the ID may be wrong.
