package shared

import "github.com/timwehrle/asana/internal/api/asana"

func FetchAllProjects(
	client *asana.Client,
	workspace *asana.Workspace,
	limit int,
) ([]*asana.Project, error) {
	initialCapacity := 100
	if limit > 0 {
		initialCapacity = limit
	}

	projects := make([]*asana.Project, 0, initialCapacity)
	options := &asana.Options{
		Limit:  limit,
		Fields: []string{
			"name",
			"archived",
			"color",
			"default_view",
			"due_on",
			"start_on",
			"notes",
			"owner",
			"owner.name",
			"team",
			"team.name",
			"public",
			"created_at",
			"modified_at",
		},
	}

	for {
		batch, nextPage, err := workspace.Projects(client, options)
		if err != nil {
			return nil, err
		}

		projects = append(projects, batch...)

		if limit > 0 && len(projects) >= limit {
			projects = projects[:limit]
			break
		}

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return projects, nil
}
