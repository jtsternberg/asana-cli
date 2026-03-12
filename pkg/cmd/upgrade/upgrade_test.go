package upgrade

import (
	"bytes"
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
