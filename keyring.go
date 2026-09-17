package main

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// Passwords are handed to the operating system's own credential store —
// macOS Keychain, Windows Credential Manager, the Secret Service on Linux —
// instead of being written next to the config file. The OS encrypts them at
// rest, ties them to the logged-in user, and keeps other users (and other
// apps) out. Nothing about a password ever touches config.json or an
// exported setup file.
const keyringService = "STP Tunnel"

// savePassword stores (or replaces) the password for one project.
func savePassword(projectID, password string) error {
	return keyring.Set(keyringService, projectID, password)
}

// loadPassword returns the saved password, or "" when nothing is stored.
// A missing entry is a normal state, not an error worth showing anyone.
func loadPassword(projectID string) (string, error) {
	pw, err := keyring.Get(keyringService, projectID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return pw, nil
}

// forgetPassword removes a stored password — used when a project is deleted,
// and when the user asks to forget it (e.g. after changing it on the server).
func forgetPassword(projectID string) {
	_ = keyring.Delete(keyringService, projectID)
}

func hasSavedPassword(projectID string) bool {
	pw, err := loadPassword(projectID)
	return err == nil && pw != ""
}
