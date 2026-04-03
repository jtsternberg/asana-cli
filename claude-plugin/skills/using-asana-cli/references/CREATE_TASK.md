# Creating a Task

## Steps

1. Parse the request for: task name, assignee, project, section, due date, description, and followers
2. If project is named, search for it: `asana projects list -q "Project Name"`
   - Only fall back to `asana projects list -l 20` if no name was given and you need to show options
3. If section is unknown, discover options: `asana projects sections "Project Name"`
4. If assignee is unclear, discover options: `asana users list`
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

6. **Verify the output** — confirm the success message includes all expected fields (name, assignee, due date, followers, URL). If a field is missing, investigate.

## Guard rails

- If creation fails, check `asana auth status` first
- If a name doesn't match, use list commands to discover the correct value
- If section is not found, run `asana projects sections "Project"` and suggest alternatives
- After creation, read the output carefully — don't claim success unless the output confirms it
