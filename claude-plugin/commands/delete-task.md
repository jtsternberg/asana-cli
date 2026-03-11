# Delete Asana Task

Permanently delete an Asana task.

## Usage

/asana-cli:delete-task <task-id>

## Arguments

- `task-id` (required) - The Asana task ID

## Instructions

1. Confirm the task ID with the user before deleting
2. Run: `asana tasks delete <task-id>`
3. Report success or failure

## Error Handling

- If task not found, inform the user the ID may be wrong
- This action is permanent and cannot be undone
