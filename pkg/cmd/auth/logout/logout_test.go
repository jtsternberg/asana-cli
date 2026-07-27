package logout

import (
	"strings"
	"testing"

	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

func clearTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv(auth.EnvVarToken, "")
	t.Setenv(auth.EnvVarPAT, "")
}

func TestRunLogout_RemovesKeyringToken(t *testing.T) {
	clearTokenEnv(t)
	auth.MockInit()
	if err := auth.Set("stored"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	io, _, out, errOut := iostreams.Test()
	if err := runLogout(&LogoutOptions{IO: io}); err != nil {
		t.Fatalf("runLogout: %v", err)
	}

	if !strings.Contains(out.String(), "Logged out") {
		t.Errorf("expected a logged-out message, got %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("expected no warning, got %q", errOut.String())
	}
	if _, err := auth.Get(); err == nil {
		t.Error("token should be gone from the keyring")
	}
}

// Deleting the keyring entry does not stop the env var from working, so logout
// has to say that out loud.
func TestRunLogout_WarnsWhenEnvTokenRemains(t *testing.T) {
	clearTokenEnv(t)
	auth.MockInit()
	if err := auth.Set("stored"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Setenv(auth.EnvVarToken, "env-token")

	io, _, out, errOut := iostreams.Test()
	if err := runLogout(&LogoutOptions{IO: io}); err != nil {
		t.Fatalf("runLogout: %v", err)
	}

	if !strings.Contains(out.String(), "Logged out") {
		t.Errorf("expected a logged-out message, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "ASANA_TOKEN") {
		t.Errorf("expected a warning naming ASANA_TOKEN, got %q", errOut.String())
	}
}

// Authenticated purely by env var: there is nothing in the keyring to delete,
// and that must not be reported as a failure.
func TestRunLogout_NoKeyringEntryIsNotAnError(t *testing.T) {
	clearTokenEnv(t)
	auth.MockInit()
	_ = auth.Delete()
	t.Setenv(auth.EnvVarPAT, "env-token")

	io, _, out, errOut := iostreams.Test()
	if err := runLogout(&LogoutOptions{IO: io}); err != nil {
		t.Fatalf("runLogout: %v", err)
	}

	if !strings.Contains(out.String(), "No stored token") {
		t.Errorf("expected a nothing-to-remove message, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "ASANA_PAT") {
		t.Errorf("expected a warning naming ASANA_PAT, got %q", errOut.String())
	}
}

func TestRunLogout_NotAuthenticated(t *testing.T) {
	clearTokenEnv(t)
	auth.MockInit()
	_ = auth.Delete()

	io, _, _, _ := iostreams.Test()
	if err := runLogout(&LogoutOptions{IO: io}); err == nil {
		t.Fatal("expected an error when there is no token at all")
	}
}
