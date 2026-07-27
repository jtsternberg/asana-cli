package auth

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// clearEnv unsets both token variables for the duration of the test, so a
// developer's own ASANA_TOKEN cannot make these tests pass or fail by accident.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvVarToken, "")
	t.Setenv(EnvVarPAT, "")
}

func TestEnvOverride_UnsetReportsNoSource(t *testing.T) {
	clearEnv(t)

	token, source := EnvOverride()
	if source != SourceNone {
		t.Errorf("source = %q, want SourceNone", source)
	}
	if token != "" {
		t.Errorf("token = %q, want empty", token)
	}
}

func TestEnvOverride_BlankValueIsTreatedAsUnset(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvVarToken, "   \n\t ")

	if _, source := EnvOverride(); source != SourceNone {
		t.Errorf("source = %q, want SourceNone for a whitespace-only value", source)
	}
}

func TestEnvOverride_TrimsSurroundingWhitespace(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvVarToken, "  1/abc\n")

	token, source := EnvOverride()
	if source != SourceEnvToken {
		t.Errorf("source = %q, want %q", source, SourceEnvToken)
	}
	if token != "1/abc" {
		t.Errorf("token = %q, want %q", token, "1/abc")
	}
}

func TestEnvOverride_PATAliasIsAccepted(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvVarPAT, "pat-token")

	token, source := EnvOverride()
	if source != SourceEnvPAT {
		t.Errorf("source = %q, want %q", source, SourceEnvPAT)
	}
	if token != "pat-token" {
		t.Errorf("token = %q, want %q", token, "pat-token")
	}
}

func TestEnvOverride_TokenWinsOverPAT(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvVarToken, "primary")
	t.Setenv(EnvVarPAT, "alias")

	token, source := EnvOverride()
	if source != SourceEnvToken {
		t.Errorf("source = %q, want %q", source, SourceEnvToken)
	}
	if token != "primary" {
		t.Errorf("token = %q, want %q", token, "primary")
	}
}

func TestGetWithSource_EnvBeatsKeyring(t *testing.T) {
	clearEnv(t)
	MockInit()
	if err := Set("keyring-token"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Setenv(EnvVarToken, "env-token")

	token, source, err := GetWithSource()
	if err != nil {
		t.Fatalf("GetWithSource: %v", err)
	}
	if token != "env-token" {
		t.Errorf("token = %q, want env-token", token)
	}
	if source != SourceEnvToken {
		t.Errorf("source = %q, want %q", source, SourceEnvToken)
	}
}

// The whole point of the override is that a headless box need not have a working
// Secret Service at all, so a broken keyring must not be consulted.
func TestGetWithSource_EnvDoesNotTouchKeyring(t *testing.T) {
	clearEnv(t)
	MockInitWithError(errors.New("no D-Bus session"))
	t.Cleanup(MockInit)
	t.Setenv(EnvVarPAT, "env-token")

	token, source, err := GetWithSource()
	if err != nil {
		t.Fatalf("GetWithSource: %v", err)
	}
	if token != "env-token" || source != SourceEnvPAT {
		t.Errorf("got (%q, %q), want (env-token, %q)", token, source, SourceEnvPAT)
	}
}

func TestGetWithSource_FallsBackToKeyring(t *testing.T) {
	clearEnv(t)
	MockInit()
	if err := Set("keyring-token"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	token, source, err := GetWithSource()
	if err != nil {
		t.Fatalf("GetWithSource: %v", err)
	}
	if token != "keyring-token" {
		t.Errorf("token = %q, want keyring-token", token)
	}
	if source != SourceKeyring {
		t.Errorf("source = %q, want %q", source, SourceKeyring)
	}
}

func TestGetWithSource_NoTokenAnywhere(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()

	_, source, err := GetWithSource()
	if err == nil {
		t.Fatal("expected an error when no token is available")
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("error %v should wrap keyring.ErrNotFound", err)
	}
	if source != SourceNone {
		t.Errorf("source = %q, want SourceNone", source)
	}
}

func TestGet_UsesEnvOverride(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()
	t.Setenv(EnvVarToken, "env-token")

	token, err := Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if token != "env-token" {
		t.Errorf("token = %q, want env-token", token)
	}
}

func TestCheck_SucceedsWithEnvAndNoKeyringEntry(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()
	t.Setenv(EnvVarToken, "env-token")

	if err := Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestCheck_FailsWithNothingAvailable(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()

	err := Check()
	if err == nil {
		t.Fatal("expected Check to fail with no token")
	}
	var authErr AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("error %v is not an AuthenticationError", err)
	}
	if authErr.Message != ErrMsgNotAuthenticated {
		t.Errorf("message = %q, want %q", authErr.Message, ErrMsgNotAuthenticated)
	}
}

// `auth login` and `auth logout` act on the keyring specifically, so they need a
// view of it that an environment override cannot mask.
func TestGetStored_IgnoresEnvOverride(t *testing.T) {
	clearEnv(t)
	MockInit()
	if err := Set("keyring-token"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Setenv(EnvVarToken, "env-token")

	token, err := GetStored()
	if err != nil {
		t.Fatalf("GetStored: %v", err)
	}
	if token != "keyring-token" {
		t.Errorf("token = %q, want keyring-token", token)
	}
}

func TestGetStored_ErrorsWhenNothingStored(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()
	t.Setenv(EnvVarToken, "env-token")

	if _, err := GetStored(); err == nil {
		t.Error("expected an error when the keyring is empty")
	}
}

func TestDeleteStored_ReportsRemoval(t *testing.T) {
	clearEnv(t)
	MockInit()
	if err := Set("keyring-token"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	removed, err := DeleteStored()
	if err != nil {
		t.Fatalf("DeleteStored: %v", err)
	}
	if !removed {
		t.Error("removed = false, want true")
	}
}

func TestDeleteStored_MissingEntryIsNotAnError(t *testing.T) {
	clearEnv(t)
	MockInit()
	_ = Delete()

	removed, err := DeleteStored()
	if err != nil {
		t.Fatalf("DeleteStored: %v", err)
	}
	if removed {
		t.Error("removed = true, want false")
	}
}

func TestSource_IsEnv(t *testing.T) {
	for _, tc := range []struct {
		source Source
		want   bool
	}{
		{SourceEnvToken, true},
		{SourceEnvPAT, true},
		{SourceKeyring, false},
		{SourceNone, false},
	} {
		if got := tc.source.IsEnv(); got != tc.want {
			t.Errorf("%q.IsEnv() = %v, want %v", tc.source, got, tc.want)
		}
	}
}

func TestSource_Describe(t *testing.T) {
	if got := SourceEnvToken.Describe(); got != "ASANA_TOKEN environment variable" {
		t.Errorf("SourceEnvToken.Describe() = %q", got)
	}
	if got := SourceEnvPAT.Describe(); got != "ASANA_PAT environment variable" {
		t.Errorf("SourceEnvPAT.Describe() = %q", got)
	}
	if got := SourceKeyring.Describe(); got != "system keyring" {
		t.Errorf("SourceKeyring.Describe() = %q", got)
	}
	if got := SourceNone.Describe(); got != "none" {
		t.Errorf("SourceNone.Describe() = %q", got)
	}
}
