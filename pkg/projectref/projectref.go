// Package projectref resolves project and section references — a name or a gid —
// to exactly one object.
//
// It is the sibling of pkg/userref, and it exists for the same reason. Six
// separate copies of "exact name match, else the *first* substring match" were
// spread across `tasks create`, `tasks move`, `projects tasks` and the `projects
// sections` commands. In a workspace with 1203 projects that means
// `--project Rocks` silently picks one of 211 matches, and in a project with
// seven "Q3 2026 Rocks - <name>" sections `--section "Q3 2026 Rocks"` silently
// picks one of seven.
//
// So an ambiguous reference is an error listing the candidates, never a guess.
// Writing a task into the wrong project is invisible to the person who asked for
// it; a failed command is not.
package projectref

import (
	"fmt"
	"strings"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// maxPageSize is the Asana API's per-page maximum.
const maxPageSize = 100

// maxListedCandidates caps how many matches an ambiguity error spells out. A
// query like "rocks" can match hundreds of projects, and an error nobody can
// read is barely better than a silent wrong guess.
const maxListedCandidates = 10

// IsGID reports whether ref looks like an Asana gid (all digits).
//
// This is intentionally greedy: an all-digit reference is always treated as a
// gid, never resolved by name. An object literally named after a bare integer
// (e.g. "2024") is therefore unreachable by that name — an accepted trade-off,
// since such names are vanishingly rare and inherently ambiguous with gids.
func IsGID(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// AmbiguousError reports a reference that matched several objects.
//
// It is a type rather than a plain error so callers can prefix it with the flag
// at fault ("--project: ...") without losing the candidate list.
type AmbiguousError struct {
	// Kind is the object type, for the message ("project", "section").
	Kind string
	// Scope describes where the search happened, for the message. Empty for a
	// workspace-wide search.
	Scope string
	Query string
	// Candidates holds every match, even the ones the message elides.
	Candidates []Candidate
	// Hint tells the caller how to narrow the search.
	Hint string
}

// Candidate is one thing a reference could have meant.
type Candidate struct {
	ID   string
	Name string
}

func (e AmbiguousError) Error() string {
	var b strings.Builder

	scope := ""
	if e.Scope != "" {
		scope = fmt.Sprintf(" in %q", e.Scope)
	}
	fmt.Fprintf(&b, "%s %q is ambiguous%s — %d %ss match:\n",
		e.Kind, e.Query, scope, len(e.Candidates), e.Kind)

	shown := e.Candidates
	if len(shown) > maxListedCandidates {
		shown = shown[:maxListedCandidates]
	}
	for _, c := range shown {
		fmt.Fprintf(&b, "  %s (ID: %s)\n", c.Name, c.ID)
	}
	if elided := len(e.Candidates) - len(shown); elided > 0 {
		fmt.Fprintf(&b, "  …and %d more\n", elided)
	}

	b.WriteString(e.Hint)
	return b.String()
}

// candidates picks the matches for a name reference: exact (case-insensitive)
// matches if there are any, otherwise substring matches.
//
// An exact match beats any number of partial ones, so "Lindris" resolves to the
// project named exactly that even alongside "Lindris Previous Rocks".
func candidates[T any](items []T, ref string, name func(T) string) []T {
	refLower := strings.ToLower(strings.TrimSpace(ref))

	var exact, partial []T
	for _, item := range items {
		itemLower := strings.ToLower(name(item))
		switch {
		case itemLower == refLower:
			exact = append(exact, item)
		case strings.Contains(itemLower, refLower):
			partial = append(partial, item)
		}
	}

	if len(exact) > 0 {
		return exact
	}
	return partial
}

// --- Projects ---

// ResolveProject resolves a project by gid or name without enumerating the whole
// workspace. Numeric input is fetched directly; everything else goes through the
// workspace typeahead API (the endpoint `projects list -q` uses), which has no
// project-count ceiling.
//
// Enumerating instead — the way most of these call sites used to work — costs
// several seconds against 1203 projects and can 400 outright with "The result is
// too large".
func ResolveProject(
	client *asana.Client,
	ws *asana.Workspace,
	ref string,
) (*asana.Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	if IsGID(ref) {
		project := &asana.Project{}
		project.ID = ref
		if err := project.Fetch(client); err != nil {
			return nil, fmt.Errorf("project %q not found: %w", ref, err)
		}
		return project, nil
	}

	projects, err := ws.SearchProjects(client, ref, maxPageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to search projects: %w", err)
	}

	return FindProject(projects, ref)
}

// FindProject picks exactly one project out of an already-fetched list, with the
// same no-guessing contract as ResolveProject. Callers holding a list for an
// interactive picker use this to avoid a second fetch.
func FindProject(projects []*asana.Project, ref string) (*asana.Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	if IsGID(ref) {
		for _, p := range projects {
			if p.ID == ref {
				return p, nil
			}
		}
		return nil, fmt.Errorf("project %q not found in workspace", ref)
	}

	matches := candidates(projects, ref, func(p *asana.Project) string { return p.Name })

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("project %q not found in workspace", ref)
	case 1:
		return matches[0], nil
	default:
		return nil, AmbiguousError{
			Kind:       "project",
			Query:      ref,
			Candidates: projectCandidates(matches),
			Hint: fmt.Sprintf("Re-run with the full project name or its ID "+
				"(`asana projects list -q %q` lists the matches).", ref),
		}
	}
}

func projectCandidates(projects []*asana.Project) []Candidate {
	out := make([]Candidate, len(projects))
	for i, p := range projects {
		out[i] = Candidate{ID: p.ID, Name: p.Name}
	}
	return out
}

// --- Sections ---

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
func ResolveSection(
	client *asana.Client,
	project *asana.Project,
	ref string,
) (*asana.Section, error) {
	// Validate before spending a request: an empty name can never resolve.
	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("section name cannot be empty")
	}

	sections, err := FetchAllSections(client, project)
	if err != nil {
		return nil, err
	}

	return FindSection(sections, project.Name, ref)
}

// FindSection picks exactly one section out of an already-fetched list, with the
// same no-guessing contract as ResolveSection. Callers that need the ordered
// section list anyway (reordering, an interactive picker) use this to avoid a
// second fetch.
func FindSection(
	sections []*asana.Section,
	projectName string,
	ref string,
) (*asana.Section, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("section name cannot be empty")
	}

	if IsGID(ref) {
		for _, s := range sections {
			if s.ID == ref {
				return s, nil
			}
		}
		return nil, fmt.Errorf("section %q not found in project %q", ref, projectName)
	}

	matches := candidates(sections, ref, func(s *asana.Section) string { return s.Name })

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("section %q not found in project %q", ref, projectName)
	case 1:
		return matches[0], nil
	default:
		return nil, AmbiguousError{
			Kind:       "section",
			Scope:      projectName,
			Query:      ref,
			Candidates: sectionCandidates(matches),
			Hint: fmt.Sprintf("Re-run with the full section name or its ID "+
				"(`asana projects sections %q` lists them).", projectName),
		}
	}
}

func sectionCandidates(sections []*asana.Section) []Candidate {
	out := make([]Candidate, len(sections))
	for i, s := range sections {
		out[i] = Candidate{ID: s.ID, Name: s.Name}
	}
	return out
}
