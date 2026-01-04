package ruby

import (
	"path/filepath"
	"testing"
)

func TestToMajorMinor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3.4.7", "3.4.0"},
		{"3.4.0", "3.4.0"},
		{"3.4", "3.4.0"},
		{"3", "3.0.0"},
		{"", ""},
		{"4.0.1", "4.0.0"},
		{"2.7.8", "2.7.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToMajorMinor(tt.input)
			if got != tt.want {
				t.Errorf("ToMajorMinor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestScopeFormat(t *testing.T) {
	// Test that Scope returns a path in the format "engine/version"
	scope := Scope()

	// Should contain a path separator
	if filepath.Dir(scope) == "." {
		t.Errorf("Scope() = %q, expected format 'engine/version'", scope)
	}

	// First component should be an engine name
	engine := filepath.Dir(scope)
	validEngines := map[string]bool{
		"ruby":        true,
		"jruby":       true,
		"truffleruby": true,
		"mruby":       true,
	}
	if !validEngines[engine] {
		t.Errorf("Scope() engine = %q, not a valid Ruby engine", engine)
	}

	// Version component should be in X.Y.0 format
	version := filepath.Base(scope)
	if len(version) < 5 { // minimum "X.Y.0"
		t.Errorf("Scope() version = %q, expected X.Y.0 format", version)
	}
}
