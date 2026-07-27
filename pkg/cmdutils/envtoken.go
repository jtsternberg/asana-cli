package cmdutils

import (
	"fmt"

	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

// WarnEnvTokenBeforeStore warns that a token in the environment takes
// precedence over the keyring, so the token about to be written to the keyring
// (by `auth login` or `auth update`) is not the one the CLI will actually use.
// Reports whether it warned.
func WarnEnvTokenBeforeStore(io *iostreams.IOStreams) bool {
	_, source := auth.EnvOverride()
	if source == auth.SourceNone {
		return false
	}

	cs := io.ColorScheme()
	fmt.Fprintf(
		io.ErrOut,
		"%s The %s is set and overrides the keyring. The token stored in the keyring will not be used until you unset it.\n",
		cs.WarningIcon,
		cs.Bold(source.Describe()),
	)
	return true
}

// WarnEnvTokenAfterLogout warns that removing the keyring entry does not log you
// out while a token is still in the environment. Reports whether it warned.
func WarnEnvTokenAfterLogout(io *iostreams.IOStreams) bool {
	_, source := auth.EnvOverride()
	if source == auth.SourceNone {
		return false
	}

	cs := io.ColorScheme()
	fmt.Fprintf(
		io.ErrOut,
		"%s The %s is still set, so the CLI remains authenticated. Unset it to finish logging out.\n",
		cs.WarningIcon,
		cs.Bold(source.Describe()),
	)
	return true
}
