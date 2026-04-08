package list

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

// sampleTeams returns a slice of teams for testing — the Avengers roster
// of test data, if you will.
func sampleTeams() []*asana.Team {
	return []*asana.Team{
		{
			ID:          "111",
			Name:        "Engineering",
			Description: "We build things and occasionally break them",
			Organization: &asana.Workspace{
				ID:   "org-1",
				Name: "Acme Corp",
			},
		},
		{
			ID:          "222",
			Name:        "Design",
			Description: "Making pixels pretty since 2015. This is a longer description that should get truncated in text output because nobody reads past fifty characters anyway",
			Organization: &asana.Workspace{
				ID:   "org-1",
				Name: "Acme Corp",
			},
		},
		{
			ID:   "333",
			Name: "Marketing",
			// No description, no organization — the minimalist team
		},
	}
}

// --- JSON Output Tests ---

func TestDisplayTeams_JSONAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	teams := sampleTeams()

	if err := displayJSON(teams, io); err != nil {
		t.Fatalf("displayJSON error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 teams, got %d", len(result))
	}

	// First team — full fields
	team0 := result[0]
	assertStr(t, team0, "id", "111")
	assertStr(t, team0, "name", "Engineering")
	assertStr(t, team0, "description", "We build things and occasionally break them")

	org := assertObj(t, team0, "organization")
	if org != nil {
		assertStr(t, org, "id", "org-1")
		assertStr(t, org, "name", "Acme Corp")
	}

	// Third team — minimal fields
	team2 := result[2]
	assertStr(t, team2, "id", "333")
	assertStr(t, team2, "name", "Marketing")
	assertStr(t, team2, "description", "")

	if team2["organization"] != nil {
		t.Error("expected organization to be null for team with no org")
	}
}

func TestDisplayTeams_JSONEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := displayJSON([]*asana.Team{}, io); err != nil {
		t.Fatalf("displayJSON error: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// --- Text Output Tests ---

func TestDisplayTeams_TextAllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	teams := sampleTeams()

	displayText(teams, io, "Test Workspace")

	output := out.String()

	mustContain := []string{
		"Test Workspace",
		"Engineering",
		"We build things and occasionally break them",
		"111",
		"Design",
		"222",
		"Marketing",
		"333",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestDisplayTeams_TextDescriptionTruncation(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	teams := sampleTeams()

	displayText(teams, io, "Test Workspace")

	output := out.String()

	// The long description should be truncated
	if strings.Contains(output, "nobody reads past fifty characters anyway") {
		t.Error("long description should be truncated in text output")
	}
	// But it should contain the truncated version with ellipsis
	if !strings.Contains(output, "...") {
		t.Error("truncated description should end with '...'")
	}
}

func TestDisplayTeams_TextEmptyDescription(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	teams := []*asana.Team{
		{
			ID:   "444",
			Name: "Solo Team",
		},
	}

	displayText(teams, io, "Test Workspace")

	output := out.String()
	if !strings.Contains(output, "Solo Team") {
		t.Errorf("text output missing team name\nGot:\n%s", output)
	}
	if !strings.Contains(output, "444") {
		t.Errorf("text output missing team ID\nGot:\n%s", output)
	}
}

// --- Flag Tests ---

func TestNewCmdList_JSONFlag(t *testing.T) {
	f := factory.Factory{
		IOStreams: func() *iostreams.IOStreams {
			io, _, _, _ := iostreams.Test()
			return io
		}(),
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
		t.Error("expected JSON flag to be true")
	}
}

// --- Test Helpers ---

func assertStr(t *testing.T, m map[string]interface{}, key, want string) {
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

func assertObj(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("JSON missing key %q", key)
		return nil
	}
	obj, ok := val.(map[string]interface{})
	if !ok {
		t.Errorf("JSON key %q is %T, not object", key, val)
		return nil
	}
	return obj
}
