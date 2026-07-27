package factory

import (
	"github.com/jtsternberg/asana-cli/internal/api/asana"
	"github.com/jtsternberg/asana-cli/internal/auth"
	"github.com/jtsternberg/asana-cli/internal/config"
	"github.com/jtsternberg/asana-cli/internal/prompter"
	"github.com/jtsternberg/asana-cli/pkg/iostreams"
)

type Factory struct {
	Config func() (*config.Config, error)
	Client func() (*asana.Client, error)

	Prompter  prompter.Prompter
	IOStreams *iostreams.IOStreams
}

func New() *Factory {
	f := &Factory{}

	f.IOStreams = ioStreams()
	f.Prompter = newPrompter()
	f.Client = newClientFunc()
	f.Config = newConfigFunc()

	return f
}

func newConfigFunc() func() (*config.Config, error) {
	return func() (*config.Config, error) {
		cfg := &config.Config{}

		if err := cfg.Load(); err != nil {
			return nil, err
		}

		return cfg, nil
	}
}

func newClientFunc() func() (*asana.Client, error) {
	return func() (*asana.Client, error) {
		token, err := auth.Get()
		if err != nil {
			return nil, err
		}

		return asana.NewClientWithAccessToken(token), nil
	}
}

func newPrompter() prompter.Prompter {
	return prompter.New()
}

func ioStreams() *iostreams.IOStreams {
	io := iostreams.System()

	return io
}
