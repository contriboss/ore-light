package ruby

const (
	// DefaultRubyVersion is the fallback Ruby version when detection fails.
	// Update this when new Ruby stable releases come out.
	DefaultRubyVersion = "3.4.8"

	// DefaultBundlerVersion is the Bundler version to write in Gemfile.lock.
	// Update this to match the current stable Bundler release.
	DefaultBundlerVersion = "2.7.2"

	// DefaultRubyGemsVersion is the RubyGems version to write in gemspec files.
	// Update this to match the current stable RubyGems release.
	DefaultRubyGemsVersion = "3.6.4"
)
