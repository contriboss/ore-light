package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Open opens a gem's source directory in the user's editor
func Open(gemName, vendorDir string) error {
	if gemName == "" {
		return fmt.Errorf("gem name is required")
	}

	// Find the gem's installation directory
	gemPath, err := findGemPath(gemName, vendorDir)
	if err != nil {
		return err
	}

	// Get the editor
	editor := getEditor()
	if editor == "" {
		return fmt.Errorf("no editor found. Set $EDITOR, $VISUAL, or $BUNDLER_EDITOR")
	}

	// Display what we're doing
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))
	fmt.Printf("%s %s in %s\n",
		infoStyle.Render("Opening"),
		gemName,
		editor)

	// Execute the editor
	cmd := exec.Command(editor, gemPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// findGemPath locates the installation directory for a gem
func findGemPath(gemName, vendorDir string) (string, error) {
	// Walk the vendor directory to find matching gems
	var candidates []string
	err := filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() {
			return nil
		}

		// Check if this is a gems directory
		if d.Name() == "gems" {
			// List contents
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				// Check if this matches our gem
				if strings.HasPrefix(entry.Name(), gemName+"-") {
					candidates = append(candidates, filepath.Join(path, entry.Name()))
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to search for gem: %w", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("gem %q not found in %s (run 'ore install' first)", gemName, vendorDir)
	}

	if len(candidates) > 1 {
		// Multiple versions found, use the first one and warn
		warnStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))
		fmt.Fprintf(os.Stderr, "%s Multiple versions of %q found, opening: %s\n",
			warnStyle.Render("Warning:"),
			gemName,
			filepath.Base(candidates[0]))
	}

	return candidates[0], nil
}

// getEditor returns the editor to use, checking environment variables
func getEditor() string {
	// Check in order of precedence (same as Bundler)
	editors := []string{
		os.Getenv("BUNDLER_EDITOR"),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
	}

	for _, editor := range editors {
		if editor != "" {
			return editor
		}
	}

	return ""
}
