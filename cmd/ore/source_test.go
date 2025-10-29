package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/gemfile"
)

// TestSourceBlockParsing validates that gems inside source blocks
// are correctly tracked with their respective source URLs
func TestSourceBlockParsing(t *testing.T) {
	gemfileContent := `source 'https://rubygems.org'

source 'https://gem.coop' do
  gem 'sidekiq', '~> 7.0'
  gem 'activerecord', '~> 7.2.0'
end

gem 'rack', '~> 3.0'

platform :mri do
  gem 'pg', '~> 1.5'
end

platform :jruby do
  gem 'jdbc-postgres', '~> 42.0'
end
`

	// Write temp Gemfile
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, "Gemfile")
	if err := os.WriteFile(tmpfile, []byte(gemfileContent), 0644); err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	// Parse Gemfile
	parser := gemfile.NewGemfileParser(tmpfile)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	// Validate source tracking
	expectedSources := map[string]string{
		"sidekiq":       "https://gem.coop",
		"activerecord":  "https://gem.coop",
		"rack":          "", // default source (nil)
		"pg":            "", // default source
		"jdbc-postgres": "", // default source
	}

	for _, dep := range parsed.Dependencies {
		expected, ok := expectedSources[dep.Name]
		if !ok {
			continue // Skip gems we're not testing
		}

		actual := ""
		if dep.Source != nil {
			actual = dep.Source.URL
		}

		if actual != expected {
			t.Errorf("Gem %s: expected source %q, got %q", dep.Name, expected, actual)
		}
	}

	// Verify gem.coop gems are found
	foundSidekiq := false
	foundActiveRecord := false
	for _, dep := range parsed.Dependencies {
		if dep.Name == "sidekiq" && dep.Source != nil && dep.Source.URL == "https://gem.coop" {
			foundSidekiq = true
		}
		if dep.Name == "activerecord" && dep.Source != nil && dep.Source.URL == "https://gem.coop" {
			foundActiveRecord = true
		}
	}

	if !foundSidekiq {
		t.Error("sidekiq not found with gem.coop source")
	}
	if !foundActiveRecord {
		t.Error("activerecord not found with gem.coop source")
	}
}

func TestInlineSourceParsing(t *testing.T) {
	gemfileContent := `gem 'nokogiri', source: 'https://gems.ruby-china.com'

source 'https://rubygems.org' do
  gem 'rails'
end

source 'https://gem.coop' do
  gem 'pg', '~> 1.5'
end
`

	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, "Gemfile")
	if err := os.WriteFile(tmpfile, []byte(gemfileContent), 0644); err != nil {
		t.Fatalf("Failed to write test Gemfile: %v", err)
	}

	parser := gemfile.NewGemfileParser(tmpfile)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse Gemfile: %v", err)
	}

	tests := map[string]string{
		"nokogiri": "https://gems.ruby-china.com",
		"rails":    "https://rubygems.org",
		"pg":       "https://gem.coop",
	}

	for _, dep := range parsed.Dependencies {
		expected, ok := tests[dep.Name]
		if !ok {
			continue
		}

		actual := ""
		if dep.Source != nil {
			actual = dep.Source.URL
		}

		if actual != expected {
			t.Errorf("Gem %s: expected source %q, got %q", dep.Name, expected, actual)
		}
	}
}
