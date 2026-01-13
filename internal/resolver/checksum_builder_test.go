package resolver

import (
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
)

func TestBuildLockfileChecksums_EmitsEmptyEntries(t *testing.T) {
	specs := []lockfile.GemSpec{
		{Name: "foo", Version: "1.0.0"},
	}
	gitSpecs := []lockfile.GitGemSpec{
		{Name: "bar", Version: "2.0.0"},
	}
	pathSpecs := []lockfile.PathGemSpec{
		{Name: "baz", Version: "3.0.0"},
	}

	checksums := buildLockfileChecksums(specs, gitSpecs, pathSpecs, nil, map[string]*RubyGemsSource{}, "https://rubygems.org")

	if _, ok := checksums["foo-1.0.0"]; !ok {
		t.Fatalf("missing checksum entry for foo-1.0.0")
	}
	if _, ok := checksums["bar-2.0.0"]; !ok {
		t.Fatalf("missing checksum entry for bar-2.0.0")
	}
	if _, ok := checksums["baz-3.0.0"]; !ok {
		t.Fatalf("missing checksum entry for baz-3.0.0")
	}
}
