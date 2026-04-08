package list

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/timwehrle/asana/internal/api/asana"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestNewCmdList_JSONFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if !sawOpts.JSON {
		t.Error("JSON = false; want true")
	}
}

func TestNewCmdList_WithIDFlag(t *testing.T) {
	f, _, _ := factory.NewTestFactory()

	var sawOpts *ListOptions
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		sawOpts = opts
		return nil
	})

	cmd.SetArgs([]string{"--with-id", "--limit", "10"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if sawOpts == nil {
		t.Fatal("runF was never called")
	}

	if !sawOpts.WithID {
		t.Error("WithID = false; want true")
	}
	if sawOpts.Limit != 10 {
		t.Errorf("Limit = %d; want 10", sawOpts.Limit)
	}
}

// --- Test data ---

func testUsers() []*asana.User {
	return []*asana.User{
		{
			ID:    "111",
			Name:  "Alice Wonderland",
			Email: "alice@example.com",
			Photo: map[string]string{"128x128": "https://example.com/alice.png"},
			Workspaces: []*asana.Workspace{
				{ID: "W1", Name: "My Workspace"},
			},
		},
		{
			ID:    "222",
			Name:  "Bob Builder",
			Email: "bob@example.com",
			Photo: nil,
			Workspaces: []*asana.Workspace{
				{ID: "W1", Name: "My Workspace"},
				{ID: "W2", Name: "Side Gig"},
			},
		},
		{
			ID:   "333",
			Name: "Charlie NoEmail",
		},
	}
}

// --- JSON Output Tests ---

func TestPrintUsersJSON_AllFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	users := testUsers()

	if err := printUsersJSON(io, users); err != nil {
		t.Fatalf("printUsersJSON error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 users, got %d", len(result))
	}

	// First user — has all fields
	u0 := result[0]
	assertStr(t, u0, "id", "111")
	assertStr(t, u0, "name", "Alice Wonderland")
	assertStr(t, u0, "email", "alice@example.com")

	// Photo should be present in JSON
	if _, ok := u0["photo"]; !ok {
		t.Error("JSON user[0] missing 'photo' key")
	}

	// Workspaces should be present in JSON
	ws, ok := u0["workspaces"]
	if !ok {
		t.Error("JSON user[0] missing 'workspaces' key")
	}
	wsArr, ok := ws.([]interface{})
	if !ok {
		t.Errorf("JSON user[0] workspaces is %T, not array", ws)
	} else if len(wsArr) != 1 {
		t.Errorf("JSON user[0] workspaces length = %d; want 1", len(wsArr))
	}

	// Second user — no photo, two workspaces
	u1 := result[1]
	assertStr(t, u1, "id", "222")
	assertStr(t, u1, "email", "bob@example.com")
	ws1, _ := u1["workspaces"].([]interface{})
	if len(ws1) != 2 {
		t.Errorf("JSON user[1] workspaces length = %d; want 2", len(ws1))
	}

	// Third user — no email, no photo, no workspaces
	u2 := result[2]
	assertStr(t, u2, "id", "333")
	assertStr(t, u2, "email", "")
}

func TestPrintUsersJSON_EmptyList(t *testing.T) {
	io, _, out, _ := iostreams.Test()

	if err := printUsersJSON(io, []*asana.User{}); err != nil {
		t.Fatalf("printUsersJSON error: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// --- Text Output Tests ---

func TestPrintUsers_ShowsEmail(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	users := testUsers()

	if err := printUsers(io, "My Workspace", users, false); err != nil {
		t.Fatalf("printUsers error: %v", err)
	}

	output := out.String()

	mustContain := []string{
		"Alice Wonderland",
		"alice@example.com",
		"Bob Builder",
		"bob@example.com",
		"Charlie NoEmail",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
	}

	// Charlie has no email — should NOT show empty parens or angle brackets
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Charlie NoEmail") {
			if strings.Contains(line, "<>") || strings.Contains(line, "()") {
				t.Errorf("Charlie's line should not have empty email markers\nGot: %s", line)
			}
		}
	}
}

func TestPrintUsers_ShowsIDAndEmail(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	users := testUsers()

	if err := printUsers(io, "My Workspace", users, true); err != nil {
		t.Fatalf("printUsers error: %v", err)
	}

	output := out.String()

	// Should contain both IDs and emails
	mustContain := []string{
		"111",
		"alice@example.com",
		"222",
		"bob@example.com",
	}

	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q\nGot:\n%s", want, output)
		}
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
	got, _ := val.(string)
	if got != want {
		t.Errorf("JSON %q = %q; want %q", key, got, want)
	}
}
