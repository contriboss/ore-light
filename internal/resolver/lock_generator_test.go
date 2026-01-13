package resolver

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPlatforms_UsesExistingLockfile(t *testing.T) {
	dir := t.TempDir()
	lockfilePath := filepath.Join(dir, "Gemfile.lock")

	content := `GEM
  remote: https://rubygems.org/
  specs:
    foo (1.0.0)

PLATFORMS
  aarch64-linux
  arm64-darwin
  x86_64-linux

DEPENDENCIES
  foo
`

	if err := os.WriteFile(lockfilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	got := detectPlatforms(lockfilePath, []string{"x86_64-linux-musl"})
	gotSet := make(map[string]bool, len(got))
	for _, p := range got {
		gotSet[p] = true
	}

	required := []string{
		"aarch64-linux",
		"arm64-darwin",
		"x86_64-linux",
		"x86_64-linux-musl",
	}

	for _, want := range required {
		if !gotSet[want] {
			t.Fatalf("detectPlatforms missing %q in %#v", want, got)
		}
	}

	cmd := exec.Command("ruby", "-e", "puts RUBY_PLATFORM")
	if output, err := cmd.Output(); err == nil {
		platform := strings.TrimSpace(string(output))
		if platform != "" && !gotSet[platform] {
			t.Fatalf("detectPlatforms missing current ruby platform %q in %#v", platform, got)
		}
	}
}
