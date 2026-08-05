package userref

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/h2non/gock"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

type obj map[string]any

func user(id, name, email string) *asana.User {
	return &asana.User{ID: id, Name: name, Email: email}
}

// workspace mirrors the shape that caused the report: duplicated first names, and
// names that contain "me" as a substring.
func workspace() []*asana.User {
	return []*asana.User{
		user("U_ANGIE", "Angie Meeker", "angie@example.com"),
		user("U_TOM", "Tom Mendez", "tom@example.com"),
		user("U_ALYSSA1", "Alyssa Rivera", "alyssa.r@example.com"),
		user("U_ALYSSA2", "Alyssa Chen", "alyssa.c@example.com"),
		user("U_ME", "Justin Sternberg", "me@example.com"),
		user("U_KATE1", "Kate Green", "kate.green@example.com"),
		user("U_KATE2", "Kate Greenwood", "kate.greenwood@example.com"),
	}
}

func resolver() *Resolver {
	return NewWithUsers(workspace(), func() (string, error) { return "U_ME", nil })
}

func TestResolve_UniqueFullName(t *testing.T) {
	got, err := resolver().Resolve("Tom Mendez")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_TOM" {
		t.Errorf("resolved %q; want U_TOM", got.ID)
	}
}

func TestResolve_UniquePartialName(t *testing.T) {
	got, err := resolver().Resolve("angie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_ANGIE" {
		t.Errorf("resolved %q; want U_ANGIE", got.ID)
	}
}

// The core of the report: two Alyssas, and the CLI used to pick whichever came
// back first.
func TestResolve_AmbiguousFirstNameIsAnError(t *testing.T) {
	_, err := resolver().Resolve("Alyssa")
	if err == nil {
		t.Fatal("expected an ambiguity error; a silent pick mis-assigns work")
	}

	var ambiguous AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error should be an AmbiguousError so callers can annotate it, got %T", err)
	}
	if len(ambiguous.Candidates) != 2 {
		t.Errorf("got %d candidates; want 2", len(ambiguous.Candidates))
	}

	// Both people, both emails and both gids — anything less and the caller
	// cannot retry unambiguously.
	for _, want := range []string{
		"Alyssa Rivera", "alyssa.r@example.com", "U_ALYSSA1",
		"Alyssa Chen", "alyssa.c@example.com", "U_ALYSSA2",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ambiguity error missing %q\nGot: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Tom Mendez") {
		t.Errorf("ambiguity error listed a non-matching user\nGot: %v", err)
	}
}

// An exact match must win outright, or "Kate Green" would be unreachable while
// "Kate Greenwood" exists.
func TestResolve_ExactNameBeatsPartial(t *testing.T) {
	got, err := resolver().Resolve("Kate Green")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_KATE1" {
		t.Errorf("resolved %q (%q); want U_KATE1", got.ID, got.Name)
	}
}

// Email is the escape hatch the ambiguity error points at, so it has to work.
func TestResolve_ByEmail(t *testing.T) {
	got, err := resolver().Resolve("alyssa.c@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_ALYSSA2" {
		t.Errorf("resolved %q; want U_ALYSSA2", got.ID)
	}
}

func TestResolve_ByGID(t *testing.T) {
	users := []*asana.User{user("1001", "Angie Meeker", ""), user("1002", "Tom Mendez", "")}
	r := NewWithUsers(users, nil)

	got, err := r.Resolve("1002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "1002" {
		t.Errorf("resolved %q; want 1002", got.ID)
	}
}

func TestResolve_UnknownGID(t *testing.T) {
	users := []*asana.User{user("1001", "Angie Meeker", "")}
	_, err := NewWithUsers(users, nil).Resolve("9999")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found, got: %v", err)
	}
}

// The bug behind asana-cli-2z7: "me" is a substring of Meeker and Mendez, so it
// has to be reserved before any name matching happens.
func TestResolve_MeIsReservedBeforeNameMatching(t *testing.T) {
	got, err := resolver().Resolve("me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_ME" {
		t.Errorf(`"me" resolved to %q (%q); want the authenticated user U_ME`, got.ID, got.Name)
	}
}

func TestResolve_MeIsCaseInsensitiveAndTrimmed(t *testing.T) {
	for _, ref := range []string{"ME", " me ", "Me"} {
		got, err := resolver().Resolve(ref)
		if err != nil {
			t.Fatalf("Resolve(%q) error: %v", ref, err)
		}
		if got.ID != "U_ME" {
			t.Errorf("Resolve(%q) = %q; want U_ME", ref, got.ID)
		}
	}
}

// A literal name containing "me" must still be reachable — reserving the token
// must not shadow real people.
func TestResolve_NamesContainingMeStillResolve(t *testing.T) {
	got, err := resolver().Resolve("Meeker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "U_ANGIE" {
		t.Errorf("resolved %q; want U_ANGIE", got.ID)
	}
}

func TestResolve_MeWithoutAnAuthenticatedUser(t *testing.T) {
	_, err := NewWithUsers(workspace(), nil).Resolve("me")
	if err == nil || !strings.Contains(err.Error(), "me") {
		t.Errorf("expected an explanatory error, got: %v", err)
	}
}

func TestResolve_MeNotInWorkspace(t *testing.T) {
	r := NewWithUsers(workspace(), func() (string, error) { return "U_ELSEWHERE", nil })
	_, err := r.Resolve("me")
	if err == nil || !strings.Contains(err.Error(), "not a member") {
		t.Errorf("expected a not-a-member error, got: %v", err)
	}
}

func TestResolve_Empty(t *testing.T) {
	_, err := resolver().Resolve("   ")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected an empty-reference error, got: %v", err)
	}
}

func TestResolveAll_SkipsBlanksAndKeepsOrder(t *testing.T) {
	got, err := resolver().ResolveAll([]string{"Tom Mendez", "", "  ", "me"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d users; want 2", len(got))
	}
	if got[0].ID != "U_TOM" || got[1].ID != "U_ME" {
		t.Errorf("got %q, %q; want U_TOM, U_ME", got[0].ID, got[1].ID)
	}
}

// All-or-nothing: a follower list with one bad name must not half-apply.
func TestResolveAll_FailsOnFirstAmbiguity(t *testing.T) {
	_, err := resolver().ResolveAll([]string{"Tom Mendez", "Alyssa"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected the ambiguity to fail the whole call, got: %v", err)
	}
}

// --- fetching ---

func TestUsers_RequestsEmailAndPaginates(t *testing.T) {
	defer gock.Off()

	var firstOptFields string
	gock.New("https://app.asana.com").
		Get("/api/1.0/users").
		MatchParam("limit", "100").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			if firstOptFields == "" {
				firstOptFields = req.URL.Query().Get("opt_fields")
			}
			return true, nil
		}).
		Reply(200).
		JSON(obj{
			"data":      []obj{{"gid": "U1", "name": "Angie Meeker", "email": "angie@example.com"}},
			"next_page": obj{"offset": "abc", "path": "/users?offset=abc"},
		})

	gock.New("https://app.asana.com").
		Get("/api/1.0/users").
		MatchParam("offset", "abc").
		Reply(200).
		JSON(obj{"data": []obj{{"gid": "U2", "name": "Tom Mendez", "email": "tom@example.com"}}})

	r := New(asana.NewClient(http.DefaultClient), "WS1", nil)
	users, err := r.Users()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users; want 2 (pagination dropped a page)", len(users))
	}
	// Without opt_fields the /users endpoint returns no email, and an ambiguity
	// error would have nothing to tell two same-named people apart by.
	if !strings.Contains(firstOptFields, "email") {
		t.Errorf("opt_fields = %q; want it to include email", firstOptFields)
	}
}

func TestUsers_FetchesOnce(t *testing.T) {
	defer gock.Off()

	gock.New("https://app.asana.com").
		Get("/api/1.0/users").
		Times(1).
		Reply(200).
		JSON(obj{"data": []obj{{"gid": "U1", "name": "Angie Meeker"}}})

	r := New(asana.NewClient(http.DefaultClient), "WS1", nil)
	for range 3 {
		if _, err := r.Users(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// A second HTTP call would find no mock left and error above.
}

func TestIsMe(t *testing.T) {
	for _, ref := range []string{"me", "ME", " Me "} {
		if !IsMe(ref) {
			t.Errorf("IsMe(%q) = false; want true", ref)
		}
	}
	for _, ref := range []string{"", "Meeker", "melissa", "me@example.com"} {
		if IsMe(ref) {
			t.Errorf("IsMe(%q) = true; want false", ref)
		}
	}
}

func TestIsGID(t *testing.T) {
	for _, ref := range []string{"1", "580196049969505", " 1002 "} {
		if !IsGID(ref) {
			t.Errorf("IsGID(%q) = false; want true", ref)
		}
	}
	for _, ref := range []string{"", "U_ME", "Tom", "12a", "-1"} {
		if IsGID(ref) {
			t.Errorf("IsGID(%q) = true; want false", ref)
		}
	}
}
