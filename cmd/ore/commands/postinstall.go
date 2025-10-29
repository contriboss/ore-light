package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/contriboss/gemfile-go/gemfile"
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

// readMessagesFromSpecDir reads post-install messages from a specifications directory using tree-sitter
func readMessagesFromSpecDir(specDir string) ([]PostInstallMessage, error) {
	// Find all gemspec files
	matches, err := filepath.Glob(filepath.Join(specDir, "*.gemspec"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob gemspec files: %w", err)
	}

	var messages []PostInstallMessage
	for _, gemspecPath := range matches {
		// Read gemspec file
		content, err := os.ReadFile(gemspecPath)
		if err != nil {
			// Skip files we can't read
			continue
		}

		// Parse with tree-sitter
		parser := gemfile.NewTreeSitterGemspecParser(content)
		gemspec, err := parser.ParseWithTreeSitter()
		if err != nil {
			// Skip gemspecs we can't parse
			continue
		}

		// Only include gems with post-install messages
		if gemspec.PostInstallMessage != "" {
			messages = append(messages, PostInstallMessage{
				GemName: gemspec.Name,
				Version: gemspec.Version,
				Message: gemspec.PostInstallMessage,
			})
		}
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
