package resolver

import "testing"

func TestIsPrereleaseVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"6.0.0.a1", true},
		{"1.0.0.rc1", true},
		{"1.0.0.pre", true},
		{"1.0.0-rc1", true},
		{"1.0.0", false},
		{"1.0.0.1", false},
	}

	for _, test := range tests {
		if got := isPrereleaseVersion(test.version); got != test.want {
			t.Errorf("isPrereleaseVersion(%q) = %v, want %v", test.version, got, test.want)
		}
	}
}
