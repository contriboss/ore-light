package resolver

import (
	"fmt"

	"github.com/contriboss/pubgrub-go"
)

// RubyGemsSource implements pubgrub.Source using Bundler's compact index.
//
// This now delegates to CompactIndexSource which uses Bundler's cache.
// Kept for backward compatibility with existing code.
type RubyGemsSource struct {
	compactSource *CompactIndexSource                  // Compact index client (Bundler-compatible)
	cache         map[string]map[string][]pubgrub.Term // Legacy cache (unused now)
	sourceURL     string                               // The source URL
	versionPins   map[string]string                    // Optional version pins
}

// NewRubyGemsSource creates a new RubyGems source for dependency resolution
func NewRubyGemsSource() *RubyGemsSource {
	return NewRubyGemsSourceWithURL("https://rubygems.org")
}

// NewRubyGemsSourceWithURL creates a RubyGems source for a specific gem server.
// This now uses compact index exclusively (Bundler-compatible).
func NewRubyGemsSourceWithURL(baseURL string) *RubyGemsSource {
	// Use compact index client (writes to Bundler's cache)
	compactSource, err := NewCompactIndexSource(baseURL)
	if err != nil {
		panic(fmt.Sprintf("failed to create compact index source: %v", err))
	}

	// Wrap in RubyGemsSource for backward compatibility
	return &RubyGemsSource{
		compactSource: compactSource,
		cache:         make(map[string]map[string][]pubgrub.Term),
		sourceURL:     baseURL,
		versionPins:   nil,
	}
}

// SetVersionPins sets version pins for selective updates.
// When a gem is pinned, GetVersions will return only the pinned version.
func (s *RubyGemsSource) SetVersionPins(pins map[string]string) {
	s.compactSource.SetVersionPins(pins)
	s.versionPins = pins
}

// SourceURL returns the URL of this gem source
func (s *RubyGemsSource) SourceURL() string {
	return s.sourceURL
}

// GetDependencies returns the dependencies for a specific package version.
// Delegates to compact index source.
func (s *RubyGemsSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	return s.compactSource.GetDependencies(name, version)
}

// GetVersions returns all available versions for a package.
// Delegates to compact index source.
func (s *RubyGemsSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	return s.compactSource.GetVersions(name)
}
