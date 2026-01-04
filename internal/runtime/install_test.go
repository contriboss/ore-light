package runtime

import (
	"path/filepath"
	"testing"
)

func TestShortRevision(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "long revision truncated to 12 chars",
			input: "b27518c5745b123456789abcdef0123456789",
			want:  "b27518c5745b",
		},
		{
			name:  "short revision unchanged",
			input: "abc123",
			want:  "abc123",
		},
		{
			name:  "empty revision",
			input: "",
			want:  "",
		},
		{
			name:  "exactly 12 chars",
			input: "123456789012",
			want:  "123456789012",
		},
		{
			name:  "13 chars truncated",
			input: "1234567890123",
			want:  "123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortRevision(tt.input)
			if got != tt.want {
				t.Errorf("shortRevision(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGitGemInstallPath(t *testing.T) {
	// Test that git gem paths follow Bundler convention:
	// <vendor>/<rubyScope>/bundler/gems/<name>-<revision[:12]>
	vendorDir := "/usr/local/bundle"
	rubyScope := "ruby/4.0.0"
	revision := "b27518c5745b123456789abcdef"
	gemName := "rgeo"

	expectedDir := "/usr/local/bundle/ruby/4.0.0/bundler/gems/rgeo-b27518c5745b"
	actualDir := filepath.Join(vendorDir, rubyScope, "bundler", "gems", gemName+"-"+shortRevision(revision))

	if actualDir != expectedDir {
		t.Errorf("git gem path = %q, want %q", actualDir, expectedDir)
	}
}

func TestPathGemInstallPath(t *testing.T) {
	// Test that path gem paths follow Bundler convention:
	// <vendor>/<rubyScope>/gems/<name>-<version>
	vendorDir := "/usr/local/bundle"
	rubyScope := "ruby/4.0.0"
	gemName := "my_gem"
	version := "1.0.0"

	expectedDir := "/usr/local/bundle/ruby/4.0.0/gems/my_gem-1.0.0"
	actualDir := filepath.Join(vendorDir, rubyScope, "gems", gemName+"-"+version)

	if actualDir != expectedDir {
		t.Errorf("path gem path = %q, want %q", actualDir, expectedDir)
	}
}
