package status

import (
	"strings"
	"testing"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/jtsternberg/asana-cli/internal/config"
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

// A workspace supplied by $ASANA_WORKSPACE has no name — only the config file
// carries one. Reporting "No default workspace configured" when one is plainly
// in force is the same debugging trap as a silent token override.
func TestPrintStatus_ReportsEnvWorkspaceWithoutAName(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	status := &Status{
		LoggedIn:        true,
		APIOperational:  true,
		TokenSource:     auth.SourceEnvToken,
		WorkspaceID:     "14748072439266",
		WorkspaceSource: config.WorkspaceSourceEnv,
	}

	if err := printStatus(io, status); err != nil {
		t.Fatalf("printStatus: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "No default workspace") {
		t.Errorf("a workspace is configured; output should not deny it:\n%s", got)
	}
	if !strings.Contains(got, "14748072439266") {
		t.Errorf("output should show the workspace ID:\n%s", got)
	}
	if !strings.Contains(got, config.EnvVarWorkspace) {
		t.Errorf("output should name where the workspace came from:\n%s", got)
	}
}

func TestPrintStatus_ReportsConfigFileWorkspaceSource(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	status := &Status{
		LoggedIn:        true,
		APIOperational:  true,
		TokenSource:     auth.SourceKeyring,
		WorkspaceID:     "W1",
		WorkspaceName:   "awesomemotive.com",
		WorkspaceSource: config.WorkspaceSourceConfigFile,
	}

	if err := printStatus(io, status); err != nil {
		t.Fatalf("printStatus: %v", err)
	}

	got := out.String()
	for _, want := range []string{"awesomemotive.com", "W1", "config file"} {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q:\n%s", want, got)
		}
	}
}

func TestPrintStatus_StillReportsAnAbsentWorkspace(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	status := &Status{LoggedIn: true, APIOperational: true, TokenSource: auth.SourceKeyring}

	if err := printStatus(io, status); err != nil {
		t.Fatalf("printStatus: %v", err)
	}
	if !strings.Contains(out.String(), "No default workspace configured") {
		t.Errorf("an absent workspace must still be reported:\n%s", out.String())
	}
}
