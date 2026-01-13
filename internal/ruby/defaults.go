package ruby

const (
	// DefaultRubyVersion is the fallback Ruby version when detection fails.
	// Update this when new Ruby stable releases come out.
	DefaultRubyVersion = "3.4.8"

	// DefaultRubyGemsVersion is the unified RubyGems/Bundler version.
	// Since RubyGems 4.0, Bundler and RubyGems share the same version.
	DefaultRubyGemsVersion = "4.0.3"
)
