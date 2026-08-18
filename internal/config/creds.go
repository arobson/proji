package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by Store.Load when no credentials file exists yet.
var ErrNotFound = errors.New("no stored credentials")

// Creds is the persisted shape of ~/.proji/creds.yml.
type Creds struct {
	Token     string    `yaml:"token"`
	TokenType string    `yaml:"token_type"`
	Username  string    `yaml:"username"`
	SavedAt   time.Time `yaml:"saved_at"`
}

// Store reads and writes Creds at a fixed path.
type Store struct {
	path string
}

// NewStore returns a Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads stored credentials, returning ErrNotFound if none exist yet.
func (s *Store) Load() (*Creds, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var c Creds
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes credentials, creating the parent directory if needed.
// The directory is created at 0700 and the file at 0600 since it holds a
// live GitHub token.
func (s *Store) Save(c Creds) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
