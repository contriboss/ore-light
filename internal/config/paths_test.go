package config

import "testing"

func TestToMajorMinor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3.4.7", "3.4.0"},
		{"3.1", "3.1.0"},
		{"3", "3.0.0"},
		{"2.7.6", "2.7.0"},
		{"3.3.0", "3.3.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToMajorMinor(tt.input)
			if result != tt.expected {
				t.Errorf("ToMajorMinor(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
