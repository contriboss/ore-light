package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRubyVersionInfo(t *testing.T) {
	content := `
module ActiveLLM
  module VERSION
    MAJOR = 0
    MINOR = 1
    TINY  = 2
    STRING = [MAJOR, MINOR, TINY].compact.join(".")
  end
end
`
	info := parseRubyVersionInfo(content)
	if got := info.computed(); got != "0.1.2" {
		t.Fatalf("computed version = %q, want %q", got, "0.1.2")
	}
	if info.String != "" {
		t.Fatalf("expected empty STRING constant, got %q", info.String)
	}
}

func TestResolveVersionFromFiles(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib", "example")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	versionPath := filepath.Join(libDir, "version.rb")
	if err := os.WriteFile(versionPath, []byte("module Example\n  VERSION = '3.1.2'\nend\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	got := resolveVersionFromFiles(dir, "Example::VERSION")
	if got != "3.1.2" {
		t.Fatalf("resolved version = %q, want %q", got, "3.1.2")
	}
}
