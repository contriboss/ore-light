package compactindex

import "testing"

func TestFindChecksum(t *testing.T) {
	infoList := []VersionInfo{
		{
			Version:  "1.0.0",
			Platform: "",
			Requirements: map[string]string{
				"checksum": "abc123",
			},
		},
		{
			Version:  "1.0.0",
			Platform: "java",
			Requirements: map[string]string{
				"checksum": "java456",
			},
		},
		{
			Version:  "2.0.0",
			Platform: "ruby",
			Requirements: map[string]string{
				"checksum": "def789",
			},
		},
		{
			Version:      "3.0.0",
			Platform:     "",
			Requirements: map[string]string{},
		},
	}

	tests := []struct {
		name         string
		version      string
		platform     string
		wantChecksum string
		wantFound    bool
	}{
		{
			name:         "exact match ruby platform",
			version:      "1.0.0",
			platform:     "",
			wantChecksum: "abc123",
			wantFound:    true,
		},
		{
			name:         "ruby platform normalized to empty",
			version:      "2.0.0",
			platform:     "ruby",
			wantChecksum: "def789",
			wantFound:    true,
		},
		{
			name:         "empty platform matches ruby",
			version:      "2.0.0",
			platform:     "",
			wantChecksum: "def789",
			wantFound:    true,
		},
		{
			name:         "java platform",
			version:      "1.0.0",
			platform:     "java",
			wantChecksum: "java456",
			wantFound:    true,
		},
		{
			name:         "case insensitive platform",
			version:      "1.0.0",
			platform:     "JAVA",
			wantChecksum: "java456",
			wantFound:    true,
		},
		{
			name:         "whitespace trimmed platform",
			version:      "1.0.0",
			platform:     "  java  ",
			wantChecksum: "java456",
			wantFound:    true,
		},
		{
			name:         "version not found",
			version:      "9.9.9",
			platform:     "",
			wantChecksum: "",
			wantFound:    false,
		},
		{
			name:         "platform not found",
			version:      "1.0.0",
			platform:     "x86-mingw32",
			wantChecksum: "",
			wantFound:    false,
		},
		{
			name:         "empty checksum",
			version:      "3.0.0",
			platform:     "",
			wantChecksum: "",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChecksum, gotFound := FindChecksum(infoList, tt.version, tt.platform)
			if gotChecksum != tt.wantChecksum {
				t.Errorf("FindChecksum() checksum = %v, want %v", gotChecksum, tt.wantChecksum)
			}
			if gotFound != tt.wantFound {
				t.Errorf("FindChecksum() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestFindChecksumEmptyList(t *testing.T) {
	checksum, found := FindChecksum(nil, "1.0.0", "")
	if checksum != "" || found {
		t.Errorf("FindChecksum(nil) = (%v, %v), want (\"\", false)", checksum, found)
	}

	checksum, found = FindChecksum([]VersionInfo{}, "1.0.0", "")
	if checksum != "" || found {
		t.Errorf("FindChecksum([]) = (%v, %v), want (\"\", false)", checksum, found)
	}
}

func TestNormalizePlatform(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"ruby", ""},
		{"RUBY", ""},
		{"  ruby  ", ""},
		{"java", "java"},
		{"JAVA", "java"},
		{"  java  ", "java"},
		{"x86_64-linux", "x86_64-linux"},
		{"x86-mingw32", "x86-mingw32"},
		{"universal-darwin", "universal-darwin"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePlatform(tt.input)
			if got != tt.want {
				t.Errorf("normalizePlatform(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindChecksumWhitespaceInChecksum(t *testing.T) {
	infoList := []VersionInfo{
		{
			Version:  "1.0.0",
			Platform: "",
			Requirements: map[string]string{
				"checksum": "  abc123  ",
			},
		},
	}

	checksum, found := FindChecksum(infoList, "1.0.0", "")
	if !found || checksum != "abc123" {
		t.Errorf("FindChecksum() = (%v, %v), want (\"abc123\", true)", checksum, found)
	}
}
