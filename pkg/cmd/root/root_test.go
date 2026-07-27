package root

import "testing"

func TestIsNoAuthCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		// Version and help are pure local output. An install script, a container
		// health check or "what have I got" reaches for these first, and telling
		// someone to run `asana upgrade` is useless if they cannot read their
		// current version.
		{"--version", []string{"asana", "--version"}, true},
		{"-v", []string{"asana", "-v"}, true},
		{"--help", []string{"asana", "--help"}, true},
		{"-h", []string{"asana", "-h"}, true},
		{"help subcommand", []string{"asana", "help"}, true},
		{"bare invocation prints help", []string{"asana"}, true},
		{"no args at all", []string{}, true},

		// Help on a subcommand is still just help.
		{"subcommand --help", []string{"asana", "tasks", "create", "--help"}, true},
		{"subcommand -h", []string{"asana", "tasks", "-h"}, true},

		// These manage the credential or the binary itself, so they cannot
		// require a working one.
		{"auth", []string{"asana", "auth", "status"}, true},
		{"upgrade", []string{"asana", "upgrade"}, true},
		{"completion", []string{"asana", "completion", "zsh"}, true},

		// Everything else talks to Asana and needs both a token and a workspace.
		{"tasks list", []string{"asana", "tasks", "list"}, false},
		{"projects list", []string{"asana", "projects", "list"}, false},
		{"config get", []string{"asana", "config", "get", "workspace"}, false},

		// A task named "--help" is an argument, not a request for help, but the
		// flag scan cannot tell them apart. Erring toward exempting is the safe
		// direction: the command still runs and fails on its own terms, whereas
		// wrongly demanding auth would block a legitimate help request.
		{"help-shaped value", []string{"asana", "tasks", "create", "-n", "--help"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoAuthCommand(tt.args); got != tt.want {
				t.Errorf("isNoAuthCommand(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
