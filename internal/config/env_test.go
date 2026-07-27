package config

import (
	"strings"
	"testing"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearWorkspaceEnv unsets the override for the duration of the test, so a
// developer's own ASANA_WORKSPACE cannot make these pass or fail by accident.
func clearWorkspaceEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvVarWorkspace, "")
}

func TestEnvWorkspace_Unset(t *testing.T) {
	clearWorkspaceEnv(t)

	ws, err := EnvWorkspace()
	require.NoError(t, err)
	assert.Nil(t, ws, "no override means no workspace, not an error")
}

func TestEnvWorkspace_BlankIsTreatedAsUnset(t *testing.T) {
	clearWorkspaceEnv(t)
	t.Setenv(EnvVarWorkspace, "  \n ")

	ws, err := EnvWorkspace()
	require.NoError(t, err)
	assert.Nil(t, ws)
}

func TestEnvWorkspace_GID(t *testing.T) {
	clearWorkspaceEnv(t)
	t.Setenv(EnvVarWorkspace, " 14748072439266 ")

	ws, err := EnvWorkspace()
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "14748072439266", ws.ID)
	// No name is available without a network call, and the ID is what every
	// request actually uses.
	assert.Equal(t, "", ws.Name)
}

// A name cannot be resolved to a GID without an API call, and this package makes
// none. Failing loudly with the command that finds the GID beats silently
// sending a name Asana will reject.
func TestEnvWorkspace_NameIsRejectedWithGuidance(t *testing.T) {
	clearWorkspaceEnv(t)
	t.Setenv(EnvVarWorkspace, "awesomemotive.com")

	ws, err := EnvWorkspace()
	require.Error(t, err)
	assert.Nil(t, ws)
	for _, want := range []string{EnvVarWorkspace, "GID", "workspaces list"} {
		assert.Contains(t, err.Error(), want)
	}
}

func TestEnvWorkspace_RejectsNonDigits(t *testing.T) {
	clearWorkspaceEnv(t)
	for _, value := range []string{"123abc", "12-34", "gid:123", "1 2"} {
		t.Setenv(EnvVarWorkspace, value)
		ws, err := EnvWorkspace()
		assert.Error(t, err, "value %q should be rejected", value)
		assert.Nil(t, ws)
	}
}

// --- RequireWorkspace precedence ---

func TestRequireWorkspace_EnvBeatsConfigFile(t *testing.T) {
	clearWorkspaceEnv(t)
	cfg := &Config{Workspace: &asana.Workspace{ID: "W-from-file", Name: "From File"}}
	t.Setenv(EnvVarWorkspace, "999")

	ws, err := cfg.RequireWorkspace()
	require.NoError(t, err)
	assert.Equal(t, "999", ws.ID, "the environment outranks the config file")
}

func TestRequireWorkspace_FallsBackToConfigFile(t *testing.T) {
	clearWorkspaceEnv(t)
	cfg := &Config{Workspace: &asana.Workspace{ID: "W1", Name: "From File"}}

	ws, err := cfg.RequireWorkspace()
	require.NoError(t, err)
	assert.Equal(t, "W1", ws.ID)
	assert.Equal(t, "From File", ws.Name)
}

// The whole point: a fresh machine with no config file can still run.
func TestRequireWorkspace_EnvAloneIsEnough(t *testing.T) {
	clearWorkspaceEnv(t)
	t.Setenv(EnvVarWorkspace, "14748072439266")
	cfg := &Config{}

	ws, err := cfg.RequireWorkspace()
	require.NoError(t, err)
	assert.Equal(t, "14748072439266", ws.ID)
}

func TestRequireWorkspace_NeitherSourceNamesBothWaysOut(t *testing.T) {
	clearWorkspaceEnv(t)
	cfg := &Config{}

	_, err := cfg.RequireWorkspace()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth login")
	assert.Contains(t, err.Error(), EnvVarWorkspace,
		"the error should name the non-interactive way out, not only the interactive one")
}

// A malformed override must not silently fall through to the config file: that
// would make a typo look like it worked while using a different workspace.
func TestRequireWorkspace_InvalidEnvDoesNotFallThrough(t *testing.T) {
	clearWorkspaceEnv(t)
	t.Setenv(EnvVarWorkspace, "not-a-gid")
	cfg := &Config{Workspace: &asana.Workspace{ID: "W1", Name: "From File"}}

	_, err := cfg.RequireWorkspace()
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvVarWorkspace)
}

// --- Load tolerating a missing file ---

// Load's job is to read what is there. A missing config file is not an
// authentication failure, and treating it as one is what made `asana --version`
// and every env-configured run die on a fresh box.
func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	t.Setenv(xdgConfigHome, t.TempDir())
	clearWorkspaceEnv(t)

	cfg := &Config{}
	require.NoError(t, cfg.Load(), "a missing config file should load as an empty config")
	assert.Nil(t, cfg.Workspace)
	assert.Equal(t, "", cfg.Username)
}

func TestLoad_MissingFileStillLeavesRequireWorkspaceToComplain(t *testing.T) {
	t.Setenv(xdgConfigHome, t.TempDir())
	clearWorkspaceEnv(t)

	cfg := &Config{}
	require.NoError(t, cfg.Load())

	_, err := cfg.RequireWorkspace()
	require.Error(t, err, "the complaint belongs here, where it can name both ways out")
	assert.NotContains(t, strings.ToLower(err.Error()), "no configuration file found")
}

// --- Source reporting ---

func TestWorkspaceWithSource(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		clearWorkspaceEnv(t)
		t.Setenv(EnvVarWorkspace, "999")
		cfg := &Config{Workspace: &asana.Workspace{ID: "W1", Name: "From File"}}

		ws, source, err := cfg.WorkspaceWithSource()
		require.NoError(t, err)
		assert.Equal(t, "999", ws.ID)
		assert.Equal(t, WorkspaceSourceEnv, source)
		assert.Equal(t, EnvVarWorkspace+" environment variable", source.Describe())
	})

	t.Run("config file", func(t *testing.T) {
		clearWorkspaceEnv(t)
		cfg := &Config{Workspace: &asana.Workspace{ID: "W1", Name: "From File"}}

		ws, source, err := cfg.WorkspaceWithSource()
		require.NoError(t, err)
		assert.Equal(t, "W1", ws.ID)
		assert.Equal(t, WorkspaceSourceConfigFile, source)
		assert.Equal(t, "config file", source.Describe())
	})

	t.Run("neither", func(t *testing.T) {
		clearWorkspaceEnv(t)
		cfg := &Config{}

		ws, source, err := cfg.WorkspaceWithSource()
		require.Error(t, err)
		assert.Nil(t, ws)
		assert.Equal(t, WorkspaceSourceNone, source)
	})
}
