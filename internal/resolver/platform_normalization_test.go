package resolver

import (
	"testing"

	"github.com/contriboss/ore-light/internal/compactindex"
)

func TestNormalizePlatformForIndex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"ruby", ""},
		{"arm64-darwin-24", "arm64-darwin"},
		{"x86_64-linux-gnu", "x86_64-linux"},
		{"x86_64-linux-musl", "x86_64-linux"},
	}

	for _, tt := range tests {
		if got := normalizePlatformForIndex(tt.input); got != tt.want {
			t.Fatalf("normalizePlatformForIndex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSelectBestPlatformVersion_PrefersGNU(t *testing.T) {
	infoList := []compactindex.VersionInfo{
		{Version: "1.0.0", Platform: "x86_64-linux-musl"},
		{Version: "1.0.0", Platform: "x86_64-linux-gnu"},
	}

	version, _, platform := selectBestPlatformVersion(infoList, "x86_64-linux", "x86_64-linux", nil, "", true)
	if version != "1.0.0" {
		t.Fatalf("version = %q, want %q", version, "1.0.0")
	}
	if platform != "x86_64-linux-gnu" {
		t.Fatalf("platform = %q, want %q", platform, "x86_64-linux-gnu")
	}
}
