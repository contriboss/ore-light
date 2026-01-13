package resolver

import (
	"os"
	"path/filepath"
	"reflect"
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
	want := []string{
		"aarch64-linux",
		"arm64-darwin",
		"x86_64-linux",
		"x86_64-linux-musl",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectPlatforms = %#v, want %#v", got, want)
	}
}
