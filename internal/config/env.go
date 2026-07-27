package config

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// EnvVarWorkspace supplies the default workspace without a config file.
//
// It is the sibling of auth.EnvVarToken. The token override removed the keyring
// dependency for unattended runs; this removes the remaining dependency on
// `asana auth login`, which is the only thing that writes a config file and
// cannot run unprompted.
const EnvVarWorkspace = "ASANA_WORKSPACE"

// EnvWorkspace returns the workspace named by the environment, or nil when the
// variable is unset or blank. A blank value counts as unset, so `ASANA_WORKSPACE=`
// in a systemd unit falls through to the config file rather than shadowing it.
//
// Only a GID is accepted. Resolving a name would need an API call, which this
// package deliberately never makes — every consumer of a *asana.Workspace uses
// the ID, and a name would have to be looked up before it could be used anyway.
// Rejecting one loudly, with the command that finds the GID, beats sending Asana
// a name it will refuse.
func EnvWorkspace() (*asana.Workspace, error) {
	value := strings.TrimSpace(getenv(EnvVarWorkspace))
	if value == "" {
		return nil, nil
	}

	if !isGID(value) {
		return nil, Error{Message: heredoc.Docf(`
            $%[2]s must be a workspace GID, but is %[3]q.

            Find the GID with %[1]sasana workspaces list --json%[1]s, then set it:
              export %[2]s=1234567890
        `, "`", EnvVarWorkspace, value)}
	}

	// Name is left empty on purpose: it is cosmetic, and inventing one from the
	// GID would put a wrong value in front of the user.
	return &asana.Workspace{ID: value}, nil
}

// isGID reports whether s is an Asana GID: a non-empty run of digits.
func isGID(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// WorkspaceSource identifies where the default workspace came from. It mirrors
// auth.Source: a silently overridden workspace is the same debugging trap as a
// silently overridden token.
type WorkspaceSource string

const (
	// WorkspaceSourceNone means no workspace could be resolved.
	WorkspaceSourceNone WorkspaceSource = ""
	// WorkspaceSourceEnv means the workspace came from $ASANA_WORKSPACE.
	WorkspaceSourceEnv WorkspaceSource = EnvVarWorkspace
	// WorkspaceSourceConfigFile means the workspace came from config.yaml.
	WorkspaceSourceConfigFile WorkspaceSource = "config file"
)

// Describe returns a human-readable name for the source.
func (s WorkspaceSource) Describe() string {
	switch s {
	case WorkspaceSourceEnv:
		return EnvVarWorkspace + " environment variable"
	case WorkspaceSourceConfigFile:
		return "config file"
	default:
		return "none"
	}
}
