package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// RunInit implements the ore init command
func RunInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "Gemfile", "Path for new Gemfile")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if Gemfile already exists
	if _, err := os.Stat(*gemfilePath); err == nil {
		return fmt.Errorf("%s already exists", *gemfilePath)
	}

	// Get Ruby version if available
	rubyVersion := detectRubyVersion()

	// Create Gemfile content
	content := `# frozen_string_literal: true

source "https://rubygems.org"

`
	if rubyVersion != "" {
		content += fmt.Sprintf("ruby \"%s\"\n\n", rubyVersion)
	}

	content += `# gem "rails"
`

	// Write Gemfile
	if err := os.WriteFile(*gemfilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write Gemfile: %w", err)
	}

	absPath, _ := filepath.Abs(*gemfilePath)
	fmt.Printf("Writing new %s to %s\n", filepath.Base(*gemfilePath), absPath)

	return nil
}

func detectRubyVersion() string {
	// Try to detect Ruby version from .ruby-version file first
	if data, err := os.ReadFile(".ruby-version"); err == nil {
		version := string(data)
		// Trim whitespace and newlines
		if len(version) > 0 {
			for i := len(version) - 1; i >= 0; i-- {
				if version[i] == '\n' || version[i] == '\r' || version[i] == ' ' {
					version = version[:i]
				} else {
					break
				}
			}
			if len(version) > 0 {
				return version
			}
		}
	}

	// Could also try `ruby -v` but for simplicity, just return empty
	// The user can manually add ruby version to Gemfile
	return ""
}
