package shared

import (
	"fmt"
	"strings"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// IsGID reports whether s looks like an Asana gid (all digits).
//
// This is intentionally greedy: an all-digit input is always treated as a gid,
// never resolved by name. An object literally named after a bare integer (e.g.
// "2024") is therefore unreachable by that name — an accepted trade-off, since
// such names are vanishingly rare and inherently ambiguous with gids anyway.
func IsGID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ResolveProject resolves a project by gid or name without enumerating the
// whole workspace. Numeric input is fetched directly as a gid; everything else
// goes through the workspace typeahead API (the same endpoint `projects list -q`
// uses), which has no project-count ceiling.
//
// Enumerating instead — the way this used to work — costs several seconds in a
// workspace with thousands of projects and can 400 outright with "The result is
// too large".
func ResolveProject(
	client *asana.Client,
	ws *asana.Workspace,
	nameOrID string,
) (*asana.Project, error) {
	if IsGID(nameOrID) {
		project := &asana.Project{}
		project.ID = nameOrID
		if err := project.Fetch(client); err != nil {
			return nil, fmt.Errorf("project %q not found: %w", nameOrID, err)
		}
		return project, nil
	}

	projects, err := ws.SearchProjects(client, nameOrID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to search projects: %w", err)
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("project %q not found in workspace", nameOrID)
	}

	return FindProject(projects, nameOrID)
}

// FindProject picks the best match for name out of an already-fetched list:
// exact (case-insensitive) name or gid first, then the first substring match.
func FindProject(projects []*asana.Project, name string) (*asana.Project, error) {
	nameLower := strings.ToLower(name)

	for _, p := range projects {
		if strings.ToLower(p.Name) == nameLower || p.ID == name {
			return p, nil
		}
	}

	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), nameLower) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("project %q not found in workspace", name)
}

// FetchAllSections pages through every section in a project.
func FetchAllSections(client *asana.Client, project *asana.Project) ([]*asana.Section, error) {
	sections := make([]*asana.Section, 0, 20)
	options := &asana.Options{Limit: maxPageSize}

	for {
		batch, nextPage, err := project.Sections(client, options)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch sections: %w", err)
		}

		sections = append(sections, batch...)

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	return sections, nil
}

// ResolveSection finds exactly one section in project by gid or name.
//
// Unlike project resolution, an ambiguous name is an error rather than a
// first-match-wins guess. Section names in a real project are routinely
// prefixed variants of each other ("Q3 2026 Rocks - Ben", "Q3 2026 Rocks -
// Alyssa", ...), and the callers of this function delete and reorder things.
// Silently acting on whichever one happened to come back first is not a
// tolerable failure mode, so ambiguity surfaces as an error listing every
// candidate and its gid — enough for the caller to retry unambiguously.
func ResolveSection(
	client *asana.Client,
	project *asana.Project,
	nameOrID string,
) (*asana.Section, error) {
	// Validate before spending a request: an empty name can never resolve.
	if strings.TrimSpace(nameOrID) == "" {
		return nil, fmt.Errorf("section name cannot be empty")
	}

	sections, err := FetchAllSections(client, project)
	if err != nil {
		return nil, err
	}

	return FindSection(sections, project.Name, nameOrID)
}

// FindSection picks exactly one section out of an already-fetched list, with the
// same no-guessing contract as ResolveSection. Callers that need the ordered
// section list anyway (reordering, for instance) use this to avoid a second
// fetch.
func FindSection(
	sections []*asana.Section,
	projectName string,
	nameOrID string,
) (*asana.Section, error) {
	if strings.TrimSpace(nameOrID) == "" {
		return nil, fmt.Errorf("section name cannot be empty")
	}

	if IsGID(nameOrID) {
		for _, s := range sections {
			if s.ID == nameOrID {
				return s, nil
			}
		}
		return nil, fmt.Errorf("section %q not found in project %q", nameOrID, projectName)
	}

	nameLower := strings.ToLower(nameOrID)

	var exact, partial []*asana.Section
	for _, s := range sections {
		sectionLower := strings.ToLower(s.Name)
		switch {
		case sectionLower == nameLower:
			exact = append(exact, s)
		case strings.Contains(sectionLower, nameLower):
			partial = append(partial, s)
		}
	}

	// An exact match beats any number of partial ones: "Done" should resolve to
	// the section named "Done" even alongside "Done - Q3".
	candidates := exact
	if len(candidates) == 0 {
		candidates = partial
	}

	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("section %q not found in project %q", nameOrID, projectName)
	case 1:
		return candidates[0], nil
	default:
		return nil, ambiguousSectionError(nameOrID, projectName, candidates)
	}
}

func ambiguousSectionError(query, projectName string, candidates []*asana.Section) error {
	var b strings.Builder
	fmt.Fprintf(&b, "section %q is ambiguous in project %q — %d sections match:\n",
		query, projectName, len(candidates))
	for _, s := range candidates {
		fmt.Fprintf(&b, "  %s (ID: %s)\n", s.Name, s.ID)
	}
	b.WriteString("Re-run with the full section name or its ID.")
	return fmt.Errorf("%s", b.String())
}
