---
name: asana-delete-task
description: Permanently delete an Asana task by ID
argument-hint: <task-id>
allowed-tools: Bash(asana *)
---

Delete an Asana task: $ARGUMENTS

## Auth check

- Auth status: !`asana auth status 2>&1 | head -1`

## Steps

1. Extract the task ID from $0
2. Confirm the task ID with the user before deleting — this action is permanent and cannot be undone
3. Run: `asana tasks delete <task-id>`
4. Report success or failure

## Guard rails

- Always confirm with the user before deleting
- If task not found, inform the user the ID may be wrong
