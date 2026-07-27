package status

import (
	"strings"
	"testing"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

func TestPrintStatus_ReportsKeyringSource(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	status := &Status{
		LoggedIn:       true,
		APIOperational: true,
		TokenSource:    auth.SourceKeyring,
		User:           &asana.User{ID: "1", Name: "Justin Sternberg"},
		WorkspaceID:    "W1",
		WorkspaceName:  "awesomemotive.com",
	}

	if err := printStatus(io, status); err != nil {
		t.Fatalf("printStatus: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "system keyring") {
		t.Errorf("output should name the token source:\n%s", got)
	}
}

// A silent env override is a debugging trap, so status has to say so.
func TestPrintStatus_ReportsEnvSource(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	status := &Status{
		LoggedIn:       true,
		APIOperational: true,
		TokenSource:    auth.SourceEnvPAT,
		User:           &asana.User{ID: "1", Name: "Justin Sternberg"},
	}

	if err := printStatus(io, status); err != nil {
		t.Fatalf("printStatus: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "ASANA_PAT") {
		t.Errorf("output should name the environment variable in use:\n%s", got)
	}
}

func TestPrintStatus_NotLoggedInSaysNothingAboutSource(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := printStatus(io, &Status{TokenSource: auth.SourceNone}); err != nil {
		t.Fatalf("printStatus: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Not logged in") {
		t.Errorf("expected a not-logged-in message, got:\n%s", got)
	}
	if strings.Contains(got, "Token source") {
		t.Errorf("no source to report when not logged in, got:\n%s", got)
	}
}
