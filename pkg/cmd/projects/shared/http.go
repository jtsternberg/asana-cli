package shared

import "github.com/jtsternberg/asana-cli/internal/api/asana"

const maxPageSize = 100

// projectFields is the opt_fields list behind every project listing. Both the
// enumerate-everything path and the typeahead path (`projects list -q`) use it,
// so the two render the same columns instead of the query path silently dropping
// the owner/team column.
var projectFields = []string{
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
}

// ProjectFields returns the canonical project opt_fields list. The returned
// slice is a copy.
func ProjectFields() []string {
	out := make([]string, len(projectFields))
	copy(out, projectFields)
	return out
}

func FetchAllProjects(
	client *asana.Client,
	workspace *asana.Workspace,
	limit int,
) ([]*asana.Project, error) {
	initialCapacity := maxPageSize
	if limit > 0 {
		initialCapacity = limit
	}

	// Always request a bounded page. The Asana API rejects an unbounded
	// projects request with "400: The result is too large", so page size is
	// fixed at 100 (the API max) regardless of the caller's total cap; `limit`
	// only truncates the accumulated results below.
	pageSize := maxPageSize
	if limit > 0 && limit < maxPageSize {
		pageSize = limit
	}

	projects := make([]*asana.Project, 0, initialCapacity)
	options := &asana.Options{
		Limit:  pageSize,
		Fields: ProjectFields(),
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
