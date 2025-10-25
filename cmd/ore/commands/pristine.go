package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/contriboss/gemfile-go/lockfile"
)

// Pristine restores gems to their pristine condition using `gem pristine`
func Pristine(gemNames []string, lockfilePath string) error {
	// Parse lockfile to get gem info
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile.lock: %w", err)
	}

	// If no gems specified, require explicit gem names (like Bundler does)
	if len(gemNames) == 0 {
		return fmt.Errorf("usage: ore pristine <gem> [<gem>...]\n\nRestores specified gems to pristine condition")
	}

	// Build map of available gems
	gemMap := make(map[string]*lockfile.GemSpec)
	for i := range lock.GemSpecs {
		gemMap[lock.GemSpecs[i].Name] = &lock.GemSpecs[i]
	}

	// Styles
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))
	gemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	var restored, failed int

	// Process each gem
	for _, gemName := range gemNames {
		gemSpec, found := gemMap[gemName]
		if !found {
			fmt.Fprintf(os.Stderr, "%s Gem %q not found in Gemfile.lock\n",
				errorStyle.Render("✗"),
				gemName)
			failed++
			continue
		}

		fmt.Printf("Restoring %s (%s)...\n",
			gemStyle.Render(gemSpec.Name),
			gemSpec.Version)

		// Use `gem pristine` to restore the gem
		if err := restoreGemWithRubyGem(gemSpec.Name, gemSpec.Version); err != nil {
			fmt.Fprintf(os.Stderr, "  %s Failed: %v\n",
				errorStyle.Render("✗"),
				err)
			failed++
			continue
		}

		fmt.Printf("  %s Restored successfully\n",
			successStyle.Render("✓"))
		restored++
	}

	fmt.Println()
	if failed > 0 {
		return fmt.Errorf("%d gem(s) could not be restored", failed)
	}

	fmt.Printf("%s Restored %d gem(s)\n",
		successStyle.Render("✓"),
		restored)

	return nil
}

// restoreGemWithRubyGem uses `gem pristine` to restore a gem
func restoreGemWithRubyGem(gemName, version string) error {
	// Run: gem pristine <gem> --version <version>
	cmd := exec.Command("gem", "pristine", gemName, "--version", version)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}

	return nil
}

// findGemInstallPath locates the installation directory for a gem
func findGemInstallPath(gemName, version, vendorDir string) (string, error) {
	// Look for gem-version directory
	expectedName := fmt.Sprintf("%s-%s", gemName, version)

	var found string
	err := filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() {
			return nil
		}

		// Check if this is a gems directory
		if d.Name() == "gems" {
			// Check for our gem
			gemPath := filepath.Join(path, expectedName)
			if stat, err := os.Stat(gemPath); err == nil && stat.IsDir() {
				found = gemPath
				return filepath.SkipAll // Found it, stop walking
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if found == "" {
		return "", fmt.Errorf("gem %s-%s not found in %s", gemName, version, vendorDir)
	}

	return found, nil
}

// removeGemDirectory removes a gem's installation directory
func removeGemDirectory(path string) error {
	return os.RemoveAll(path)
}

// getGemCachePath returns the path to a gem's cached .gem file
func getGemCachePath(cacheDir, gemName, version string) string {
	filename := fmt.Sprintf("%s-%s.gem", gemName, version)
	// Cache structure: cache/gems/*.gem
	return filepath.Join(cacheDir, "gems", filename)
}

// verifyGemInCache checks if a gem exists in the cache
func verifyGemInCache(cacheDir, gemName, version string) (bool, error) {
	gemPath := getGemCachePath(cacheDir, gemName, version)
	stat, err := os.Stat(gemPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !stat.IsDir(), nil
}

// removeGemspec removes a gem's specification file
func removeGemspec(gemName, version, vendorDir string) error {
	// Find and remove gemspec
	specName := fmt.Sprintf("%s-%s.gemspec", gemName, version)

	return filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == specName {
			// Found the gemspec
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove gemspec: %w", err)
			}
			return filepath.SkipAll
		}
		return nil
	})
}

// ValidateGemIntegrity checks if a gem's files are intact
func ValidateGemIntegrity(gemName, version, vendorDir string) (bool, []string, error) {
	gemPath, err := findGemInstallPath(gemName, version, vendorDir)
	if err != nil {
		return false, nil, err
	}

	var missingFiles []string
	var fileCount int

	// Walk the gem directory and check for common files
	err = filepath.WalkDir(gemPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// File might be missing or unreadable
			relPath := strings.TrimPrefix(path, gemPath+string(filepath.Separator))
			missingFiles = append(missingFiles, relPath)
			return nil
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})

	if err != nil {
		return false, nil, err
	}

	// If we have files but encountered errors, integrity is compromised
	intact := len(missingFiles) == 0 && fileCount > 0

	return intact, missingFiles, nil
}
