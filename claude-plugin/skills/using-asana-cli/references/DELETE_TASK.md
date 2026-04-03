# Deleting a Task

**This action is permanent and cannot be undone.**

## Steps

1. Extract the task ID
2. **Confirm with the user before deleting**
3. Run: `asana tasks delete <task-id>`
4. Report success or failure

## Guard rails

- Always confirm with the user before deleting
- If task not found, inform the user the ID may be wrong
