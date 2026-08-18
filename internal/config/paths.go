package config

import (
	"os"
	"path/filepath"
)

// DefaultDir returns the directory proji stores its credentials in.
// It honors PROJI_HOME (used by tests and power users) before falling back
// to ~/.proji.
func DefaultDir() (string, error) {
	if dir := os.Getenv("PROJI_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".proji"), nil
}

// CredsPath returns the full path to the credentials file.
func CredsPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "creds.yml"), nil
}
