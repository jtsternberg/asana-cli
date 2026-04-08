package list

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/internal/config"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func fakeWorkspaces() []*asana.Workspace {
	return []*asana.Workspace{
		{
			ID:             "111",
			Name:           "Acme Corp",
			IsOrganization: true,
			EmailDomains:   []string{"acme.com", "acme.io"},
		},
		{
			ID:             "222",
			Name:           "Side Project",
			IsOrganization: false,
		},
	}
}

// --- JSON Output Tests ---

func TestRunList_JSONOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	workspaces := fakeWorkspaces()

	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Username: "jtester"}, nil
		},
		Client: func() (*asana.Client, error) {
			return nil, nil // not used directly; we call runList which calls client
		},
		JSON: true,
	}

	// We need to test the display functions directly since runList calls the API.
	err := displayWorkspaces(opts, workspaces)
	if err != nil {
		t.Fatalf("displayWorkspaces error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(result))
	}

	// First workspace: organization with email domains
	ws0 := result[0]
	assertString(t, ws0, "id", "111")
	assertString(t, ws0, "name", "Acme Corp")
	assertBool(t, ws0, "is_organization", true)

	domains, ok := ws0["email_domains"].([]interface{})
	if !ok {
		t.Fatal("email_domains is not an array")
	}
	if len(domains) != 2 {
		t.Errorf("email_domains length = %d; want 2", len(domains))
	}
	if domains[0] != "acme.com" {
		t.Errorf("email_domains[0] = %v; want acme.com", domains[0])
	}

	// Second workspace: plain workspace, no email domains
	ws1 := result[1]
	assertString(t, ws1, "id", "222")
	assertString(t, ws1, "name", "Side Project")
	assertBool(t, ws1, "is_organization", false)
}

func TestRunList_JSONEmptyWorkspaces(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Username: "jtester"}, nil
		},
		JSON: true,
	}

	err := displayWorkspaces(opts, []*asana.Workspace{})
	if err != nil {
		t.Fatalf("displayWorkspaces error: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "No workspaces found for jtester" {
		t.Errorf("expected empty message, got: %s", output)
	}
}

// --- Text Output Tests ---

func TestRunList_TextOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Username: "jtester"}, nil
		},
		JSON: false,
	}

	err := displayWorkspaces(opts, fakeWorkspaces())
	if err != nil {
		t.Fatalf("displayWorkspaces error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Acme Corp",
		"Organization",
		"111",
		"Side Project",
		"Workspace",
		"222",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestRunList_TextEmptyWorkspaces(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	opts := &ListOptions{
		IO: io,
		Config: func() (*config.Config, error) {
			return &config.Config{Username: "jtester"}, nil
		},
		JSON: false,
	}

	err := displayWorkspaces(opts, []*asana.Workspace{})
	if err != nil {
		t.Fatalf("displayWorkspaces error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No workspaces found") {
		t.Errorf("expected empty message, got: %s", output)
	}
}

// --- Flag Tests ---

func TestNewCmdList_JSONFlag(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := factory.Factory{
		IOStreams: ios,
	}

	var capturedOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		capturedOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute error: %v", err)
	}

	if capturedOpts == nil {
		t.Fatal("runF was not called")
	}
	if !capturedOpts.JSON {
		t.Error("expected JSON=true when --json flag is set")
	}
}

// --- Test Helpers ---

func assertString(t *testing.T, m map[string]interface{}, key, want string) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return
	}
	got, ok := val.(string)
	if !ok {
		t.Errorf("JSON key %q is %T, not string", key, val)
		return
	}
	if got != want {
		t.Errorf("JSON %q = %q; want %q", key, got, want)
	}
}

func assertBool(t *testing.T, m map[string]interface{}, key string, want bool) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return
	}
	got, ok := val.(bool)
	if !ok {
		t.Errorf("JSON key %q is %T, not bool", key, val)
		return
	}
	if got != want {
		t.Errorf("JSON %q = %v; want %v", key, got, want)
	}
}
