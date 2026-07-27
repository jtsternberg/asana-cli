package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

var (
	service = "asana"
	user    = "user"
)

func Set(secret string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- keyring.Set(service, user, secret)
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to set secret: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout while trying to set secret in keyring")
	}
}

// Get returns the token to authenticate with, preferring an environment
// override over the keyring. See GetWithSource when the origin matters.
func Get() (string, error) {
	token, _, err := GetWithSource()
	return token, err
}

// GetWithSource returns the token and where it came from.
//
// Precedence is environment over keyring: an unattended job on a headless box
// must not depend on a Secret Service being present and unlocked, and on a
// machine with no keyring at all (containers, CI) the environment is the only
// way in. When an override is set the keyring is not consulted at all, so a
// broken or absent one cannot fail the command.
func GetWithSource() (string, Source, error) {
	if token, source := EnvOverride(); source != SourceNone {
		return token, source, nil
	}

	token, err := getFromKeyring()
	if err != nil {
		return "", SourceNone, err
	}
	return token, SourceKeyring, nil
}

// GetStored returns the token held in the keyring, ignoring any environment
// override. Commands that manage the stored credential — `auth login`,
// `auth logout` — need this view; everything that just needs to authenticate
// wants Get.
func GetStored() (string, error) {
	return getFromKeyring()
}

func getFromKeyring() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)
		secret, err := keyring.Get(service, user)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- secret
		}
	}()

	select {
	case secret := <-resultCh:
		return secret, nil
	case err := <-errCh:
		return "", fmt.Errorf("failed to get secret: %w", err)
	case <-ctx.Done():
		return "", fmt.Errorf("timeout while trying to get secret in keyring")
	}
}

func Delete() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- keyring.Delete(service, user)
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to delete secret: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout while trying to delete secret in keyring")
	}
}

// DeleteStored removes the keyring entry and reports whether there was one to
// remove. An absent entry is not an error: with an environment override in play
// a caller can be authenticated without anything ever having been stored.
func DeleteStored() (bool, error) {
	err := Delete()
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, keyring.ErrNotFound):
		return false, nil
	default:
		return false, err
	}
}

func MockInit() {
	keyring.MockInit()
}

func MockInitWithError(err error) {
	keyring.MockInitWithError(err)
}
