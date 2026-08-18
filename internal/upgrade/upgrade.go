// Package upgrade checks for a newer proji release and installs it over
// the currently running binary, so a student can run "proji upgrade"
// instead of re-running the install script to find out if one exists.
package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/arobson/proji/internal/setup"
)

const (
	defaultAPIBaseURL   = "https://api.github.com"
	defaultAssetBaseURL = "https://github.com"
)

var supportedPlatforms = map[string]bool{
	"linux-amd64":  true,
	"linux-arm64":  true,
	"darwin-amd64": true,
	"darwin-arm64": true,
}

// Upgrader checks for and installs a newer proji release. Every field has
// a production default that's easy to fake in tests.
type Upgrader struct {
	// Repo is the GitHub "owner/repo" to check releases for.
	Repo string
	// CurrentVersion is this binary's own build-time version (main.version).
	CurrentVersion string
	GOOS, GOARCH   string

	HTTPClient *http.Client
	Out        io.Writer

	// ExecutablePath returns the path of the binary to replace. Defaults
	// to os.Executable.
	ExecutablePath func() (string, error)
	// Runner runs "sudo install" when ExecutablePath's directory isn't
	// writable by the current user, so the password prompt reaches a real
	// terminal. Reuses setup.CommandRunner rather than a second copy of
	// the same "run an interactive system command" abstraction.
	Runner setup.CommandRunner

	// APIBaseURL/AssetBaseURL override GitHub's real hosts, e.g. to point
	// at an httptest server in tests.
	APIBaseURL   string
	AssetBaseURL string
}

// Run checks the latest release and, if it's different from
// CurrentVersion, downloads, verifies, and installs it over the current
// executable.
func (u *Upgrader) Run(ctx context.Context) error {
	platform := u.GOOS + "-" + u.GOARCH
	if !supportedPlatforms[platform] {
		return fmt.Errorf("proji upgrade doesn't support %s yet; download a binary yourself from https://github.com/%s/releases", platform, u.Repo)
	}

	latest, err := u.latestVersion(ctx)
	if err != nil {
		return fmt.Errorf("check for the latest version: %w", err)
	}
	if latest == u.CurrentVersion {
		fmt.Fprintf(u.Out, "You're already running the latest version (%s).\n", latest)
		return nil
	}

	dest, err := u.ExecutablePath()
	if err != nil {
		return fmt.Errorf("determine the current executable's path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(dest); err == nil {
		dest = resolved
	}

	fmt.Fprintf(u.Out, "Upgrading from %s to %s...\n", displayVersion(u.CurrentVersion), latest)

	tmp, err := u.download(ctx, latest, platform)
	if err != nil {
		return fmt.Errorf("download %s: %w", latest, err)
	}
	defer os.Remove(tmp) // #nosec G307 -- best-effort cleanup of our own temp file

	if err := u.install(ctx, tmp, dest); err != nil {
		return fmt.Errorf("install the new binary: %w", err)
	}

	fmt.Fprintf(u.Out, "Upgraded to %s.\n", latest)
	return nil
}

func displayVersion(v string) string {
	if v == "" {
		return "an unknown version"
	}
	return v
}

func (u *Upgrader) latestVersion(ctx context.Context) (string, error) {
	url := u.apiBaseURL() + "/repos/" + u.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching latest release", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest release has no tag name")
	}
	return release.TagName, nil
}

func (u *Upgrader) download(ctx context.Context, tag, platform string) (string, error) {
	binName := "proji-" + platform
	base := u.AssetBaseURL
	if base == "" {
		base = defaultAssetBaseURL
	}
	releaseBase := base + "/" + u.Repo + "/releases/download/" + tag

	checksums, err := u.fetch(ctx, releaseBase+"/checksums.txt")
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	expected := checksumFor(string(checksums), binName)
	if expected == "" {
		return "", fmt.Errorf("no checksum entry for %s in checksums.txt", binName)
	}

	data, err := u.fetch(ctx, releaseBase+"/"+binName)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", binName, err)
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", binName, actual, expected)
	}

	f, err := os.CreateTemp("", "proji-upgrade-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	if err := f.Chmod(0o755); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func (u *Upgrader) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func checksumFor(checksums, filename string) string {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return fields[0]
		}
	}
	return ""
}

// install replaces dest with the file at src, using a direct copy when
// dest's directory is writable, or shelling out to "sudo install"
// (interactively, so the password prompt reaches a real terminal) when
// it's not.
func (u *Upgrader) install(ctx context.Context, src, dest string) error {
	dir := filepath.Dir(dest)
	if dirWritable(dir) {
		return copyFile(src, dest)
	}
	fmt.Fprintf(u.Out, "Installing to %s (sudo required)...\n", dest)
	return u.Runner.Run(ctx, "sudo", "install", "-m", "0755", src, dest)
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".proji-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src) // #nosec G304 -- src is our own verified download, not user input
	if err != nil {
		return err
	}
	tmp := dest + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil { // #nosec G306 G703 -- dest is os.Executable()'s own path, not user input; the binary is meant to be world-executable
		return err
	}
	return os.Rename(tmp, dest)
}

func (u *Upgrader) httpClient() *http.Client {
	if u.HTTPClient != nil {
		return u.HTTPClient
	}
	return http.DefaultClient
}

func (u *Upgrader) apiBaseURL() string {
	if u.APIBaseURL != "" {
		return u.APIBaseURL
	}
	return defaultAPIBaseURL
}
