package setup

import (
	"os"
	"strings"
)

// Platform identifies which install strategy applies to the current
// machine.
type Platform string

const (
	PlatformMacOS        Platform = "macos"
	PlatformDebianFamily Platform = "debian-family" // Debian, Ubuntu, Raspbian, and derivatives
	PlatformUnsupported  Platform = "unsupported"
)

// DetectPlatform classifies a machine from its GOOS and (for Linux)
// /etc/os-release contents.
func DetectPlatform(goos string, osRelease map[string]string) Platform {
	switch goos {
	case "darwin":
		return PlatformMacOS
	case "linux":
		switch osRelease["ID"] {
		case "debian", "ubuntu", "raspbian":
			return PlatformDebianFamily
		}
		if strings.Contains(osRelease["ID_LIKE"], "debian") {
			return PlatformDebianFamily
		}
	}
	return PlatformUnsupported
}

// ReadOSRelease reads and parses /etc/os-release (the standard source of
// Linux distro identity: ID, ID_LIKE, etc.).
func ReadOSRelease() (map[string]string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}
	return parseOSRelease(string(data)), nil
}

func parseOSRelease(data string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		result[key] = strings.Trim(value, `"'`)
	}
	return result
}
