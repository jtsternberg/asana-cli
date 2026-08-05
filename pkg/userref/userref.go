// Package userref resolves user references — a name, a gid, or the literal
// token "me" — to a single Asana user.
//
// `tasks create`, `tasks update` and `tasks search` each hand-rolled this, five
// copies in four files, and every copy did the same thing: try an exact
// case-insensitive name match, then return the *first* substring match, then try
// the gid. In a workspace with 241 people and 22 duplicated first names — five
// Davids, two Alyssas, two Tiagos — `--assignee David` silently assigned work to
// whichever David the API happened to list first.
//
// So this package refuses to guess. An ambiguous reference is an error naming
// every candidate with its email and gid, which is enough for a human or an
// agent to retry unambiguously. Mis-assigning a task is worse than failing to
// assign one: the failure is visible and the mis-assignment is not.
package userref

import (
	"fmt"
	"strings"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// maxPageSize is the Asana API's per-page maximum.
const maxPageSize = 100

// fields are the user fields every resolution needs. Without an explicit
// opt_fields list the /users endpoint returns compact records with no email, so
// an ambiguity error would have nothing to tell the two Alyssas apart by.
var fields = []string{"name", "email"}

// CurrentUserFunc reports the gid of the authenticated user. It exists so the
// cached config value can be used when present, and so tests need no network.
type CurrentUserFunc func() (string, error)

// CachedOrFetched returns a CurrentUserFunc that prefers the gid already in the
// config and falls back to /users/me. cachedID is typically cfg.UserID; passing
// the value rather than the config keeps this package free of a config import.
func CachedOrFetched(cachedID string, client *asana.Client) CurrentUserFunc {
	return func() (string, error) {
		if cachedID != "" {
			return cachedID, nil
		}
		user, err := client.CurrentUser()
		if err != nil {
			return "", err
		}
		return user.ID, nil
	}
}

// Resolver resolves user references against one workspace, fetching the user
// list at most once however many references it is asked about.
type Resolver struct {
	client      *asana.Client
	workspaceID string
	currentUser CurrentUserFunc

	users   []*asana.User
	fetched bool
}

// New returns a Resolver for the given workspace. currentUser may be nil, in
// which case the token "me" cannot be resolved.
func New(client *asana.Client, workspaceID string, currentUser CurrentUserFunc) *Resolver {
	return &Resolver{
		client:      client,
		workspaceID: workspaceID,
		currentUser: currentUser,
	}
}

// NewWithUsers returns a Resolver over an already-fetched user list. Callers that
// need the list for an interactive picker anyway use this to avoid a second
// fetch.
func NewWithUsers(users []*asana.User, currentUser CurrentUserFunc) *Resolver {
	return &Resolver{users: users, fetched: true, currentUser: currentUser}
}

// Users returns the workspace user list, fetching it on first use.
func (r *Resolver) Users() ([]*asana.User, error) {
	if r.fetched {
		return r.users, nil
	}

	ws := &asana.Workspace{ID: r.workspaceID}
	users := make([]*asana.User, 0, maxPageSize)
	options := &asana.Options{Limit: maxPageSize, Fields: fields}

	for {
		batch, nextPage, err := ws.Users(r.client, options)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch users: %w", err)
		}

		users = append(users, batch...)

		if nextPage == nil || nextPage.Offset == "" {
			break
		}

		options.Offset = nextPage.Offset
	}

	r.users = users
	r.fetched = true
	return r.users, nil
}

// IsMe reports whether ref is the reserved token for the authenticated user.
//
// It has to be checked before any name matching: "me" is a substring of Meeker,
// Mendez, Gomez and plenty of other real names, so a fuzzy pass would happily
// resolve `--followers me` to a colleague.
func IsMe(ref string) bool {
	return strings.EqualFold(strings.TrimSpace(ref), "me")
}

// IsGID reports whether ref looks like an Asana gid (all digits).
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

// Me resolves the authenticated user within the workspace.
func (r *Resolver) Me() (*asana.User, error) {
	if r.currentUser == nil {
		return nil, fmt.Errorf(`cannot resolve "me": no authenticated user available`)
	}

	id, err := r.currentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to identify the authenticated user: %w", err)
	}

	users, err := r.Users()
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if u.ID == id {
			return u, nil
		}
	}

	return nil, fmt.Errorf("the authenticated user is not a member of this workspace")
}

// Resolve resolves one reference to exactly one user.
//
// Order matters: "me" first, then gid, then an exact (case-insensitive) name
// match, then a substring match. A reference matching more than one user at the
// winning tier is an error, never a pick.
func (r *Resolver) Resolve(ref string) (*asana.User, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("user reference cannot be empty")
	}

	if IsMe(ref) {
		return r.Me()
	}

	users, err := r.Users()
	if err != nil {
		return nil, err
	}

	if IsGID(ref) {
		for _, u := range users {
			if u.ID == ref {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user %q not found in workspace", ref)
	}

	candidates := match(users, ref)

	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("user %q not found in workspace", ref)
	case 1:
		return candidates[0], nil
	default:
		return nil, AmbiguousError{Query: ref, Candidates: candidates}
	}
}

// ResolveAll resolves several references, skipping blanks. It reports the first
// failure rather than partially applying a change: assigning three of four
// followers and erroring is harder to reason about than assigning none.
func (r *Resolver) ResolveAll(refs []string) ([]*asana.User, error) {
	out := make([]*asana.User, 0, len(refs))

	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		user, err := r.Resolve(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, user)
	}

	return out, nil
}

// match returns the candidates for a name reference: exact matches if there are
// any, otherwise substring matches. An exact match beats any number of partial
// ones, so "Kate Green" still resolves when "Kate Greenwood" also exists.
//
// Email is matched exactly too, since it is the one reference that is guaranteed
// unique — it is what an ambiguity error tells the caller to use.
func match(users []*asana.User, ref string) []*asana.User {
	refLower := strings.ToLower(ref)

	var exact, partial []*asana.User
	for _, u := range users {
		switch {
		case strings.EqualFold(u.Name, ref), strings.EqualFold(u.Email, ref):
			exact = append(exact, u)
		case strings.Contains(strings.ToLower(u.Name), refLower):
			partial = append(partial, u)
		}
	}

	if len(exact) > 0 {
		return exact
	}
	return partial
}

// AmbiguousError reports a reference that matched several users. It is a type
// rather than a plain error so callers can prefix it with the flag at fault
// ("--assignee: ...") without losing the candidate list.
type AmbiguousError struct {
	Query      string
	Candidates []*asana.User
}

func (e AmbiguousError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "user %q is ambiguous — %d people match:\n", e.Query, len(e.Candidates))
	for _, u := range e.Candidates {
		if u.Email != "" {
			fmt.Fprintf(&b, "  %s <%s> (ID: %s)\n", u.Name, u.Email, u.ID)
		} else {
			fmt.Fprintf(&b, "  %s (ID: %s)\n", u.Name, u.ID)
		}
	}
	fmt.Fprintf(&b, "Re-run with the full name, the email address, or the numeric user ID "+
		"(`asana users list -q %q` lists the matches).", e.Query)
	return b.String()
}
