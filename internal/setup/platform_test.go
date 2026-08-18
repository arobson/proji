package setup_test

import (
	"testing"

	"github.com/arobson/proji/internal/setup"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		osRelease map[string]string
		want      setup.Platform
	}{
		{"macOS", "darwin", nil, setup.PlatformMacOS},
		{"debian", "linux", map[string]string{"ID": "debian"}, setup.PlatformDebianFamily},
		{"ubuntu", "linux", map[string]string{"ID": "ubuntu"}, setup.PlatformDebianFamily},
		{"raspbian", "linux", map[string]string{"ID": "raspbian"}, setup.PlatformDebianFamily},
		{"debian derivative via ID_LIKE", "linux", map[string]string{"ID": "linuxmint", "ID_LIKE": "ubuntu debian"}, setup.PlatformDebianFamily},
		{"fedora is unsupported", "linux", map[string]string{"ID": "fedora"}, setup.PlatformUnsupported},
		{"windows is unsupported", "windows", nil, setup.PlatformUnsupported},
		{"linux with no os-release data", "linux", nil, setup.PlatformUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setup.DetectPlatform(tt.goos, tt.osRelease); got != tt.want {
				t.Errorf("DetectPlatform(%q, %v) = %q, want %q", tt.goos, tt.osRelease, got, tt.want)
			}
		})
	}
}
