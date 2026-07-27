package login

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/shlex"
	"github.com/jtsternberg/asana-cli/pkg/factory"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
	"github.com/stretchr/testify/require"
)

func TestNewCmdLogin(t *testing.T) {
	tests := []struct {
		name     string
		cli      string
		stdin    string
		stdinTTY bool
		wants    LoginOptions
		wantsErr bool
	}{
		{
			name:  "with-token and without workspace",
			cli:   "--with-token",
			stdin: "test-token\n",
			wants: LoginOptions{
				Token: "test-token",
			},
			wantsErr: true,
		},
		{
			name:  "with-token and with workspace",
			cli:   "--with-token --workspace \"Test Workspace\"",
			stdin: "test-token\n",
			wants: LoginOptions{
				Token:     "test-token",
				Workspace: "Test Workspace",
			},
		},
		{
			name: "with workspace and without token",
			cli:  "--workspace \"Test Workspace\"",
			wants: LoginOptions{
				Interactive: true,
				Workspace:   "Test Workspace",
			},
		},
		{
			name: "interactive login run",
			cli:  "",
			wants: LoginOptions{
				Interactive: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, stdin, _, _ := iostreams.Test()
			f := &factory.Factory{
				IOStreams: ios,
			}

			ios.IsStdoutTTY = true
			ios.IsStdinTTY = tt.stdinTTY
			if tt.stdin != "" {
				stdin.WriteString(tt.stdin)
			}

			argv, err := shlex.Split(tt.cli)
			require.NoError(t, err)

			var gotOpts *LoginOptions
			cmd := NewCmdLogin(*f, func(opts *LoginOptions) error {
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

			require.Equal(t, tt.wants.Token, gotOpts.Token)
			require.Equal(t, tt.wants.Workspace, gotOpts.Workspace)
			require.Equal(t, tt.wants.Interactive, gotOpts.Interactive)
		})
	}
}

// --- Credential persistence ordering (asana-cli-a3d) ---

// recordingWriter captures the order of the side effects a login leaves behind.
type recordingWriter struct {
	calls       []string
	storedToken string
	setErr      error
	saveErr     error
	deleteErr   error
}

func (w *recordingWriter) writer() credentialWriter {
	return credentialWriter{
		setToken: func(token string) error {
			w.calls = append(w.calls, "setToken")
			if w.setErr != nil {
				return w.setErr
			}
			w.storedToken = token
			return nil
		},
		saveConfig: func() error {
			w.calls = append(w.calls, "saveConfig")
			return w.saveErr
		},
		deleteToken: func() (bool, error) {
			w.calls = append(w.calls, "deleteToken")
			if w.deleteErr != nil {
				return false, w.deleteErr
			}
			w.storedToken = ""
			return true, nil
		},
	}
}

func TestPersistLogin_StoresTokenBeforeConfig(t *testing.T) {
	w := &recordingWriter{}

	if err := persistLogin("1/abc", w.writer()); err != nil {
		t.Fatalf("persistLogin: %v", err)
	}

	// The order is the point: a config on disk with no token behind it is the
	// one half-state nothing recovers from.
	require.Equal(t, []string{"setToken", "saveConfig"}, w.calls)
	require.Equal(t, "1/abc", w.storedToken)
}

func TestPersistLogin_ConfigFailureRollsTheTokenBack(t *testing.T) {
	w := &recordingWriter{saveErr: errors.New("disk full")}

	err := persistLogin("1/abc", w.writer())
	if err == nil {
		t.Fatal("expected an error when the config cannot be saved")
	}
	require.ErrorContains(t, err, "disk full")
	require.Equal(t, []string{"setToken", "saveConfig", "deleteToken"}, w.calls)
	require.Empty(t, w.storedToken, "a failed login should leave no token behind")
}

func TestPersistLogin_TokenFailureNeverReachesTheConfig(t *testing.T) {
	w := &recordingWriter{setErr: errors.New("keyring locked")}

	err := persistLogin("1/abc", w.writer())
	require.ErrorContains(t, err, "keyring locked")
	require.Equal(t, []string{"setToken"}, w.calls)
}

// A rollback that itself fails must not hide either failure: the user has to
// know a token was left in the keyring.
func TestPersistLogin_FailedRollbackReportsBothFailures(t *testing.T) {
	w := &recordingWriter{
		saveErr:   errors.New("disk full"),
		deleteErr: errors.New("keyring locked"),
	}

	err := persistLogin("1/abc", w.writer())
	require.ErrorContains(t, err, "disk full")
	require.ErrorContains(t, err, "keyring locked")
}
