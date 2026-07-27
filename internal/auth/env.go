package auth

import (
	"os"
	"strings"
)

// Environment variables that can supply the token instead of the keyring.
//
// ASANA_TOKEN is the primary name; ASANA_PAT is accepted as an alias because
// Asana's own documentation calls the credential a "Personal Access Token", so
// that is the name people reach for first.
const (
	EnvVarToken = "ASANA_TOKEN"
	EnvVarPAT   = "ASANA_PAT"
)

// Source identifies where a token was read from.
type Source string

const (
	// SourceNone means no token was found.
	SourceNone Source = ""
	// SourceKeyring means the token came from the OS keyring.
	SourceKeyring Source = "keyring"
	// SourceEnvToken means the token came from $ASANA_TOKEN.
	SourceEnvToken Source = EnvVarToken
	// SourceEnvPAT means the token came from $ASANA_PAT.
	SourceEnvPAT Source = EnvVarPAT
)

// IsEnv reports whether the token came from an environment variable rather than
// the keyring.
func (s Source) IsEnv() bool {
	return s == SourceEnvToken || s == SourceEnvPAT
}

// Describe returns a human-readable name for the source, for status output and
// warnings.
func (s Source) Describe() string {
	switch {
	case s.IsEnv():
		return string(s) + " environment variable"
	case s == SourceKeyring:
		return "system keyring"
	default:
		return "none"
	}
}

// EnvOverride returns the token supplied by the environment and which variable
// supplied it, or ("", SourceNone) when neither is set to a non-blank value.
//
// A blank value counts as unset: `ASANA_TOKEN=` in a systemd unit or a stray
// `export ASANA_TOKEN` should fall through to the keyring rather than shadow it
// with an empty string.
func EnvOverride() (string, Source) {
	for _, name := range []string{EnvVarToken, EnvVarPAT} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value, Source(name)
		}
	}
	return "", SourceNone
}
