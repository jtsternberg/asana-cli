package cmdutils

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

func TestWarnEnvTokenBeforeStore_Quiet(t *testing.T) {
	clearTokenEnv(t)
	io, _, out, errOut := iostreams.Test()

	if WarnEnvTokenBeforeStore(io) {
		t.Error("expected no warning when no env token is set")
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("expected no output, got stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestWarnEnvTokenBeforeStore_Warns(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv(auth.EnvVarToken, "env-token")
	io, _, _, errOut := iostreams.Test()

	if !WarnEnvTokenBeforeStore(io) {
		t.Error("expected a warning when ASANA_TOKEN is set")
	}
	got := errOut.String()
	for _, want := range []string{"ASANA_TOKEN", "overrides", "keyring"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q should mention %q", got, want)
		}
	}
}

func TestWarnEnvTokenBeforeStore_NamesThePATAlias(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv(auth.EnvVarPAT, "env-token")
	io, _, _, errOut := iostreams.Test()

	if !WarnEnvTokenBeforeStore(io) {
		t.Error("expected a warning when ASANA_PAT is set")
	}
	if got := errOut.String(); !strings.Contains(got, "ASANA_PAT") {
		t.Errorf("warning %q should name the variable that is actually set", got)
	}
}

func TestWarnEnvTokenAfterLogout_Quiet(t *testing.T) {
	clearTokenEnv(t)
	io, _, out, errOut := iostreams.Test()

	if WarnEnvTokenAfterLogout(io) {
		t.Error("expected no warning when no env token is set")
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("expected no output, got stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestWarnEnvTokenAfterLogout_Warns(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv(auth.EnvVarToken, "env-token")
	io, _, _, errOut := iostreams.Test()

	if !WarnEnvTokenAfterLogout(io) {
		t.Error("expected a warning when ASANA_TOKEN is set")
	}
	got := errOut.String()
	for _, want := range []string{"ASANA_TOKEN", "still"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q should mention %q", got, want)
		}
	}
}
