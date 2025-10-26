package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PostInstallMessage represents a gem's post-install message
type PostInstallMessage struct {
	GemName string
	Version string
	Message string
}

// ReadPostInstallMessages reads post-install messages from installed gemspecs
func ReadPostInstallMessages(vendorDir string) ([]PostInstallMessage, error) {
	// Find specifications directories
	var specDirs []string
	err := filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == "specifications" {
			specDirs = append(specDirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var messages []PostInstallMessage

	for _, specDir := range specDirs {
		msgs, err := readMessagesFromSpecDir(specDir)
		if err != nil {
			// Non-fatal, just skip this directory
			continue
		}
		messages = append(messages, msgs...)
	}

	return messages, nil
}

// readMessagesFromSpecDir reads post-install messages from a specifications directory
func readMessagesFromSpecDir(specDir string) ([]PostInstallMessage, error) {
	// Use Ruby to read gemspec files and extract post_install_message
	rubyScript := `
require 'rubygems'
require 'json'

specs_dir = ARGV[0]
results = []

Dir.glob(File.join(specs_dir, '*.gemspec')).each do |gemspec_file|
  begin
    spec = Gem::Specification.load(gemspec_file)
    if spec && spec.post_install_message && !spec.post_install_message.empty?
      results << {
        name: spec.name,
        version: spec.version.to_s,
        message: spec.post_install_message
      }
    end
  rescue => e
    # Skip problematic gemspecs
  end
end

puts JSON.generate(results)
`

	cmd := exec.Command("ruby", "-e", rubyScript, specDir)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ruby script: %w", err)
	}

	var results []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse ruby output: %w", err)
	}

	var messages []PostInstallMessage
	for _, r := range results {
		messages = append(messages, PostInstallMessage{
			GemName: r.Name,
			Version: r.Version,
			Message: r.Message,
		})
	}

	return messages, nil
}

// DisplayPostInstallMessages displays post-install messages with nice formatting
func DisplayPostInstallMessages(messages []PostInstallMessage) {
	if len(messages) == 0 {
		return
	}

	fmt.Println()

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	gemNameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	for _, msg := range messages {
		// Header
		fmt.Println(borderStyle.Render(strings.Repeat("─", 80)))
		fmt.Printf("%s %s (%s)\n",
			headerStyle.Render("Post-install message from"),
			gemNameStyle.Render(msg.GemName),
			msg.Version)
		fmt.Println(borderStyle.Render(strings.Repeat("─", 80)))

		// Message (trim and add slight indentation)
		lines := strings.Split(strings.TrimSpace(msg.Message), "\n")
		for _, line := range lines {
			fmt.Println(messageStyle.Render("  " + line))
		}
		fmt.Println()
	}
}
