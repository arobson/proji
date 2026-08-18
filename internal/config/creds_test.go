package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/arobson/proji/internal/config"
)

func TestStore_SaveAndLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "creds.yml")
	store := config.NewStore(path)

	want := config.Creds{
		Token:     "ghp_abc123",
		TokenType: "oauth",
		Username:  "learner",
		SavedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if *got != want {
		t.Fatalf("Load() = %+v, want %+v", *got, want)
	}
}

func TestStore_Load_MissingFileReturnsErrNotFound(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "missing.yml"))

	_, err := store.Load()
	if !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestStore_Load_CorruptYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.yml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	store := config.NewStore(path)

	if _, err := store.Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for corrupt YAML")
	}
}

func TestStore_Save_SetsRestrictivePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "creds.yml")
	store := config.NewStore(path)

	if err := store.Save(config.Creds{Token: "t"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat creds file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("creds file mode = %o, want 0600", perm)
	}

	di, err := os.Stat(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("stat creds dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("creds dir mode = %o, want 0700", perm)
	}
}
