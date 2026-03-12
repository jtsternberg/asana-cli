package upgrade

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/require"
	"github.com/timwehrle/asana/pkg/factory"
	"github.com/timwehrle/asana/pkg/iostreams"
)

func TestNewCmdUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		cli      string
		wantsYes bool
		wantsErr bool
	}{
		{
			name:     "no flags",
			cli:      "",
			wantsYes: false,
		},
		{
			name:     "with --yes",
			cli:      "--yes",
			wantsYes: true,
		},
		{
			name:     "with short -y",
			cli:      "-y",
			wantsYes: true,
		},
		{
			name:     "unknown flag",
			cli:      "--unknown",
			wantsErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &factory.Factory{
				IOStreams: ios,
			}

			argv, err := shlex.Split(tt.cli)
			require.NoError(t, err)

			var gotOpts *UpgradeOptions
			cmd := NewCmdUpgrade(*f, func(opts *UpgradeOptions) error {
				gotOpts = opts
				return nil
			})

			cmd.SetArgs(argv)
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			if tt.wantsErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantsYes, gotOpts.Yes)
		})
	}
}

func TestPlatformAssetName(t *testing.T) {
	// Verify the function returns a correctly structured name without error on
	// the current platform.
	name, err := platformAssetName()
	require.NoError(t, err)
	require.NotEmpty(t, name)
	require.True(t, strings.HasPrefix(name, "asana_"), "asset name should start with 'asana_'")
	require.True(t, strings.HasSuffix(name, ".tar.gz"), "asset name should end with '.tar.gz'")
	// Strip the prefix/suffix and verify we have at least OS and arch.
	middle := strings.TrimSuffix(strings.TrimPrefix(name, "asana_"), ".tar.gz")
	require.NotEmpty(t, middle, "asset name should contain OS and arch between prefix and suffix")
}

func TestIsAsanaSourceDir(t *testing.T) {
	t.Run("not a source dir (empty)", func(t *testing.T) {
		dir := t.TempDir()
		require.False(t, isAsanaSourceDir(dir))
	})

	t.Run("has .git but no cmd/asana/main.go", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		require.False(t, isAsanaSourceDir(dir))
	})

	t.Run("has cmd/asana/main.go but no .git", func(t *testing.T) {
		dir := t.TempDir()
		mainDir := filepath.Join(dir, "cmd", "asana")
		require.NoError(t, os.MkdirAll(mainDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte("package main"), 0o644))
		require.False(t, isAsanaSourceDir(dir))
	})

	t.Run("has both .git and cmd/asana/main.go", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		mainDir := filepath.Join(dir, "cmd", "asana")
		require.NoError(t, os.MkdirAll(mainDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte("package main"), 0o644))
		require.True(t, isAsanaSourceDir(dir))
	})
}

func TestReplaceBinary(t *testing.T) {
	t.Run("replaces file content", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		newBin := filepath.Join(dir, "newbin")

		require.NoError(t, os.WriteFile(target, []byte("old content"), 0o755))
		require.NoError(t, os.WriteFile(newBin, []byte("new content"), 0o755))

		require.NoError(t, replaceBinary(target, newBin))

		got, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, "new content", string(got))
	})
}

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid github url", "https://github.com/jtsternberg/asana-cli/releases/download/v1.0.0/asana_Linux_x86_64.tar.gz", false},
		{"invalid domain", "https://evil.example.com/asana.tar.gz", true},
		{"http not https", "http://github.com/foo", true},
		{"empty url", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDownloadURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

