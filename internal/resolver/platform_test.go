package resolver

import (
	"testing"
)

func TestPlatformScore(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		want     int
	}{
		{"empty platform (ruby)", "", 0},
		{"plain linux preferred", "x86_64-linux", 3},
		{"linux-gnu second choice", "x86_64-linux-gnu", 2},
		{"linux-musl third choice", "x86_64-linux-musl", 1},
		{"darwin scores 1", "arm64-darwin", 1},
		{"darwin versioned", "arm64-darwin-23", 1},
		{"case insensitive", "X86_64-LINUX-GNU", 2},
		{"whitespace trimmed", "  x86_64-linux  ", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlatformScore(tt.platform)
			if got != tt.want {
				t.Errorf("PlatformScore(%q) = %d, want %d", tt.platform, got, tt.want)
			}
		})
	}
}

func TestPlatformScoreWithTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		platform string
		want     int
	}{
		// musl target prefers musl
		{"musl target, musl platform", "x86_64-linux-musl", "x86_64-linux-musl", 3},
		{"musl target, gnu platform", "x86_64-linux-musl", "x86_64-linux-gnu", 1},
		{"musl target, plain linux", "x86_64-linux-musl", "x86_64-linux", 2},

		// gnu target prefers gnu
		{"gnu target, gnu platform", "x86_64-linux-gnu", "x86_64-linux-gnu", 3},
		{"gnu target, musl platform", "x86_64-linux-gnu", "x86_64-linux-musl", 1},
		{"gnu target, plain linux", "x86_64-linux-gnu", "x86_64-linux", 2},

		// non-linux falls back to PlatformScore
		{"darwin target", "arm64-darwin", "arm64-darwin", 1},
		{"darwin target, empty platform", "arm64-darwin", "", 0},

		// case insensitivity
		{"case insensitive target", "X86_64-LINUX-MUSL", "x86_64-linux-musl", 3},
		{"case insensitive platform", "x86_64-linux-musl", "X86_64-LINUX-MUSL", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlatformScoreWithTarget(tt.target, tt.platform)
			if got != tt.want {
				t.Errorf("PlatformScoreWithTarget(%q, %q) = %d, want %d", tt.target, tt.platform, got, tt.want)
			}
		})
	}
}

func TestPlatformMatchesRequirement(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		required string
		want     bool
	}{
		// empty requirement
		{"empty req, empty actual", "", "", true},
		{"empty req, non-empty actual", "x86_64-linux", "", false},

		// ruby requirement
		{"ruby req, empty actual", "", "ruby", true},
		{"ruby req, non-empty actual", "x86_64-linux", "ruby", false},

		// normalized matching
		{"exact match", "x86_64-linux", "x86_64-linux", true},
		{"no match", "arm64-darwin", "x86_64-linux", false},

		// darwin versioned platforms
		{"darwin versioned match", "arm64-darwin", "arm64-darwin-23", true},
		{"darwin versioned mismatch", "x86_64-darwin", "arm64-darwin-23", false},

		// whitespace handling
		{"whitespace in actual", "  x86_64-linux  ", "x86_64-linux", true},
		{"whitespace in required", "x86_64-linux", "  x86_64-linux  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlatformMatchesRequirement(tt.actual, tt.required)
			if got != tt.want {
				t.Errorf("PlatformMatchesRequirement(%q, %q) = %v, want %v", tt.actual, tt.required, got, tt.want)
			}
		})
	}
}

func TestPlatformRequirementSatisfied(t *testing.T) {
	tests := []struct {
		name      string
		platforms map[string]bool
		hasRuby   bool
		required  string
		want      bool
	}{
		// empty requirement
		{"empty req", map[string]bool{}, false, "", false},

		// ruby requirement
		{"ruby req, has ruby", map[string]bool{}, true, "ruby", true},
		{"ruby req, no ruby", map[string]bool{"x86_64-linux": true}, false, "ruby", false},

		// platform requirements
		{"exact platform match", map[string]bool{"x86_64-linux": true}, false, "x86_64-linux", true},
		{"platform not found", map[string]bool{"arm64-darwin": true}, false, "x86_64-linux", false},

		// darwin versioned
		{"darwin versioned satisfied", map[string]bool{"arm64-darwin": true}, false, "arm64-darwin-23", true},
		{"darwin versioned not satisfied", map[string]bool{"x86_64-darwin": true}, false, "arm64-darwin-23", false},

		// multiple platforms
		{"multiple platforms, one match", map[string]bool{"x86_64-linux": true, "arm64-darwin": true}, false, "x86_64-linux", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlatformRequirementSatisfied(tt.platforms, tt.hasRuby, tt.required)
			if got != tt.want {
				t.Errorf("PlatformRequirementSatisfied(%v, %v, %q) = %v, want %v", tt.platforms, tt.hasRuby, tt.required, got, tt.want)
			}
		})
	}
}

func TestVersionSupportsPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		hasRuby   bool
		platforms map[string]bool
		required  []string
		want      bool
	}{
		// no required platforms
		{"no required, has ruby", true, map[string]bool{}, nil, true},
		{"no required, no ruby", false, map[string]bool{"x86_64-linux": true}, nil, false},

		// ruby gem supports all platforms
		{"ruby gem supports all", true, map[string]bool{}, []string{"x86_64-linux", "arm64-darwin"}, true},

		// platform-specific gem
		{"all platforms satisfied", false, map[string]bool{"x86_64-linux": true, "arm64-darwin": true}, []string{"x86_64-linux", "arm64-darwin"}, true},
		{"some platforms missing", false, map[string]bool{"x86_64-linux": true}, []string{"x86_64-linux", "arm64-darwin"}, false},
		{"no platforms", false, map[string]bool{}, []string{"x86_64-linux"}, false},

		// empty strings in required
		{"empty string ignored", false, map[string]bool{"x86_64-linux": true}, []string{"", "x86_64-linux"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionSupportsPlatforms(tt.hasRuby, tt.platforms, tt.required)
			if got != tt.want {
				t.Errorf("VersionSupportsPlatforms(%v, %v, %v) = %v, want %v", tt.hasRuby, tt.platforms, tt.required, got, tt.want)
			}
		})
	}
}

func TestBuildPlatformTargets(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		want      []PlatformTarget
	}{
		{
			"empty input",
			nil,
			[]PlatformTarget{},
		},
		{
			"single platform",
			[]string{"x86_64-linux"},
			[]PlatformTarget{{Original: "x86_64-linux", Normalized: "x86_64-linux"}},
		},
		{
			"deduplication",
			[]string{"x86_64-linux", "x86_64-linux"},
			[]PlatformTarget{{Original: "x86_64-linux", Normalized: "x86_64-linux"}},
		},
		{
			"sorted output",
			[]string{"x86_64-linux", "arm64-darwin"},
			[]PlatformTarget{
				{Original: "arm64-darwin", Normalized: "arm64-darwin"},
				{Original: "x86_64-linux", Normalized: "x86_64-linux"},
			},
		},
		{
			"whitespace trimmed",
			[]string{"  x86_64-linux  "},
			[]PlatformTarget{{Original: "x86_64-linux", Normalized: "x86_64-linux"}},
		},
		{
			"empty strings ignored",
			[]string{"", "x86_64-linux", "  "},
			[]PlatformTarget{{Original: "x86_64-linux", Normalized: "x86_64-linux"}},
		},
		{
			"musl platform preserves original",
			[]string{"x86_64-linux-musl"},
			[]PlatformTarget{{Original: "x86_64-linux-musl", Normalized: "x86_64-linux"}},
		},
		{
			"darwin versioned preserves original",
			[]string{"arm64-darwin-23"},
			[]PlatformTarget{{Original: "arm64-darwin-23", Normalized: "arm64-darwin"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPlatformTargets(tt.platforms)
			if len(got) != len(tt.want) {
				t.Fatalf("BuildPlatformTargets(%v) returned %d targets, want %d", tt.platforms, len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Original != tt.want[i].Original || got[i].Normalized != tt.want[i].Normalized {
					t.Errorf("BuildPlatformTargets(%v)[%d] = %+v, want %+v", tt.platforms, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDeduplicatePlatforms(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		want      []string
	}{
		{"empty", nil, []string{}},
		{"single", []string{"x86_64-linux"}, []string{"x86_64-linux"}},
		{"duplicates removed", []string{"x86_64-linux", "x86_64-linux"}, []string{"x86_64-linux"}},
		{"sorted", []string{"x86_64-linux", "arm64-darwin"}, []string{"arm64-darwin", "x86_64-linux"}},
		{"whitespace trimmed", []string{"  x86_64-linux  "}, []string{"x86_64-linux"}},
		{"empty strings ignored", []string{"", "x86_64-linux", ""}, []string{"x86_64-linux"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicatePlatforms(tt.platforms)
			if len(got) != len(tt.want) {
				t.Fatalf("DeduplicatePlatforms(%v) = %v, want %v", tt.platforms, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("DeduplicatePlatforms(%v)[%d] = %q, want %q", tt.platforms, i, got[i], tt.want[i])
				}
			}
		})
	}
}
