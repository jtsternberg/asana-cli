# Create Asana Task

Create a new Asana task with full metadata.

## Usage

/asana-cli:create-task [task description]

## Arguments

- `task description` (optional) - Natural language description of the task to create

## Instructions

1. Parse the user's request for: task name, assignee, project, section, due date, description, and followers
2. If project is unknown, list available projects: `asana projects list -l 20`
3. If section is unknown, list sections: `asana projects sections "Project Name"`
4. If assignee is unclear, list users: `asana users list`
5. Create the task with all available flags:

```bash
asana tasks create \
  -n "Task name" \
  -a "Assignee" \
  -p "Project" \
  -s "Section" \
  -d "YYYY-MM-DD" \
  -m "Description" \
  -f "Follower1,Follower2"
```

6. Report the created task URL back to the user

## Error Handling

- If creation fails, check `asana auth status` first
- If a name doesn't match, use list commands to discover the correct value
- If section is not found, run `asana projects sections "Project"` and suggest alternatives
