# Move Asana Task

Move an Asana task to a different project and/or section.

## Usage

/asana-cli:asana-move-task <task-id> [destination]

## Arguments

- `task-id` (required) - The Asana task ID or URL
- `destination` (optional) - Natural language description of where to move it (e.g., "to Outgoing Tasks in the Tom section")

## Instructions

1. Extract the task ID from the argument (if a URL, parse the ID from it)
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

## When to Use

Use `tasks move` instead of deleting and recreating a task when:
- A task needs to be in a different project
- A task was created in the wrong project
- A task needs to be in a different section of a different project

Using move preserves task history, comments, followers, and attachments.

## Error Handling

- If task not found, ask the user to verify the ID
- If project not found, run `asana projects list` and suggest closest match
- If section not found, run `asana projects sections "Project"` and suggest alternatives
