package upgrade_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/setup/setuptest"
	"github.com/arobson/proji/internal/upgrade"
)

const (
	fakeBinaryContent = "fake proji binary contents"
	testLatestTag     = "proji-v0.2.0"
)

func fakeBinarySHA256() string {
	sum := sha256.Sum256([]byte(fakeBinaryContent))
	return hex.EncodeToString(sum[:])
}

// newTestUpgrader returns an *upgrade.Upgrader wired to a fake GitHub API
// server (serving /repos/.../releases/latest) and a fake asset server
// (serving the binary + checksums.txt), both pinned to testLatestTag as
// the "latest" release.
func newTestUpgrader(t *testing.T, currentVersion string) (*upgrade.Upgrader, *setuptest.FakeCommandRunner, string) {
	t.Helper()
	latestTag := testLatestTag

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/repos/arobson/proji/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, latestTag)
	})
	apiServer := httptest.NewServer(apiMux)
	t.Cleanup(apiServer.Close)

	assetMux := http.NewServeMux()
	binPath := fmt.Sprintf("/arobson/proji/releases/download/%s/proji-linux-amd64", latestTag)
	checksumPath := fmt.Sprintf("/arobson/proji/releases/download/%s/checksums.txt", latestTag)
	assetMux.HandleFunc(binPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fakeBinaryContent))
	})
	assetMux.HandleFunc(checksumPath, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  proji-linux-amd64\n", fakeBinarySHA256())
	})
	assetServer := httptest.NewServer(assetMux)
	t.Cleanup(assetServer.Close)

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "proji")
	if err := os.WriteFile(dest, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("seed old binary: %v", err)
	}

	runner := &setuptest.FakeCommandRunner{}
	out := &strings.Builder{}
	u := &upgrade.Upgrader{
		Repo:           "arobson/proji",
		CurrentVersion: currentVersion,
		GOOS:           "linux",
		GOARCH:         "amd64",
		Out:            out,
		ExecutablePath: func() (string, error) { return dest, nil },
		Runner:         runner,
		APIBaseURL:     apiServer.URL,
		AssetBaseURL:   assetServer.URL,
	}
	return u, runner, dest
}

func TestUpgrader_Run_InstallsNewerVersion(t *testing.T) {
	u, _, dest := newTestUpgrader(t, "proji-v0.1.0")

	if err := u.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(data) != fakeBinaryContent {
		t.Errorf("installed binary content = %q, want %q", data, fakeBinaryContent)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("installed binary mode = %v, want executable", info.Mode())
	}
}

func TestUpgrader_Run_AlreadyLatest(t *testing.T) {
	u, _, dest := newTestUpgrader(t, "proji-v0.2.0")

	if err := u.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old binary" {
		t.Error("should not have replaced the binary when already up to date")
	}
}

func TestUpgrader_Run_UnsupportedPlatform(t *testing.T) {
	u, _, _ := newTestUpgrader(t, "proji-v0.1.0")
	u.GOOS, u.GOARCH = "windows", "amd64"

	if err := u.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want an error for an unsupported platform")
	}
}

func TestUpgrader_Run_ChecksumMismatch(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/repos/arobson/proji/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"tag_name": "proji-v0.2.0"}`)
	})
	apiServer := httptest.NewServer(apiMux)
	t.Cleanup(apiServer.Close)

	assetMux := http.NewServeMux()
	assetMux.HandleFunc("/arobson/proji/releases/download/proji-v0.2.0/proji-linux-amd64", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fakeBinaryContent))
	})
	assetMux.HandleFunc("/arobson/proji/releases/download/proji-v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "0000000000000000000000000000000000000000000000000000000000000000  proji-linux-amd64\n")
	})
	assetServer := httptest.NewServer(assetMux)
	t.Cleanup(assetServer.Close)

	dest := filepath.Join(t.TempDir(), "proji")
	if err := os.WriteFile(dest, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := &upgrade.Upgrader{
		Repo:           "arobson/proji",
		CurrentVersion: "proji-v0.1.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		Out:            &strings.Builder{},
		ExecutablePath: func() (string, error) { return dest, nil },
		Runner:         &setuptest.FakeCommandRunner{},
		APIBaseURL:     apiServer.URL,
		AssetBaseURL:   assetServer.URL,
	}

	if err := u.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want a checksum mismatch error")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old binary" {
		t.Error("should not have installed a binary that failed checksum verification")
	}
}

func TestUpgrader_Run_FallsBackToSudoWhenDirNotWritable(t *testing.T) {
	u, runner, dest := newTestUpgrader(t, "proji-v0.1.0")

	// Make the destination directory read-only so the direct-copy path is
	// unavailable and the sudo-install fallback is exercised instead.
	dir := filepath.Dir(dest)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := u.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	found := false
	for _, c := range runner.Calls {
		if c.Name == "sudo" && len(c.Args) > 0 && c.Args[0] == "install" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a sudo install call, got calls: %v", runner.Calls)
	}
}
