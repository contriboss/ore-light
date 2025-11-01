package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GemInfo represents information about an installed gem
type GemInfo struct {
	Name         string
	Version      string
	Path         string
	Summary      string
	Dependencies []Dependency
}

// Dependency represents a gem dependency
type Dependency struct {
	Name         string
	Requirements string
	Type         string // "runtime" or "development"
}

// RunGems lists all installed gems in the system
func RunGems(filter string) error {
	// Get gem directory
	gemDir, err := getGemDirectory()
	if err != nil {
		return fmt.Errorf("failed to get gem directory: %w", err)
	}

	// Find all installed gems
	gems, err := findInstalledGems(gemDir)
	if err != nil {
		return fmt.Errorf("failed to find installed gems: %w", err)
	}

	// Filter if needed
	if filter != "" {
		gems = filterGems(gems, filter)
	}

	// Sort by name
	sort.Slice(gems, func(i, j int) bool {
		if gems[i].Name == gems[j].Name {
			return gems[i].Version < gems[j].Version
		}
		return gems[i].Name < gems[j].Name
	})

	// Display gems
	displayGems(gems, filter)

	return nil
}

// getGemDirectory gets the system gem directory
func getGemDirectory() (string, error) {
	// Try to get from `gem environment gemdir`
	cmd := exec.Command("gem", "environment", "gemdir")
	output, err := cmd.Output()
	if err == nil {
		gemDir := strings.TrimSpace(string(output))
		if gemDir != "" {
			return gemDir, nil
		}
	}

	// Fallback: use default paths
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Common gem paths
	candidates := []string{
		filepath.Join(home, ".gem", "ruby"),
		filepath.Join(home, ".local", "share", "gem", "ruby"),
		filepath.Join(home, ".rbenv", "versions"),
		filepath.Join(home, ".local", "share", "mise", "installs", "ruby"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			// Find first Ruby version directory
			entries, err := os.ReadDir(candidate)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					// Found a version directory
					gemPath := filepath.Join(candidate, entry.Name(), "lib", "ruby", "gems")
					if _, err := os.Stat(gemPath); err == nil {
						// Find version subdirectory
						subEntries, err := os.ReadDir(gemPath)
						if err == nil && len(subEntries) > 0 {
							return filepath.Join(gemPath, subEntries[0].Name()), nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not find gem directory")
}

// findInstalledGems finds all installed gems in a directory
func findInstalledGems(gemDir string) ([]GemInfo, error) {
	gemsPath := filepath.Join(gemDir, "gems")

	entries, err := os.ReadDir(gemsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read gems directory %s: %w", gemsPath, err)
	}

	var gems []GemInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse gem name and version from directory name
		// Format: gemname-version
		name := entry.Name()
		lastDash := strings.LastIndex(name, "-")
		if lastDash == -1 {
			continue // Invalid format
		}

		gemName := name[:lastDash]
		version := name[lastDash+1:]

		gems = append(gems, GemInfo{
			Name:    gemName,
			Version: version,
			Path:    filepath.Join(gemsPath, name),
		})
	}

	return gems, nil
}

// filterGems filters gems by name
func filterGems(gems []GemInfo, filter string) []GemInfo {
	filter = strings.ToLower(filter)
	var filtered []GemInfo

	for _, gem := range gems {
		if strings.Contains(strings.ToLower(gem.Name), filter) {
			filtered = append(filtered, gem)
		}
	}

	return filtered
}

// displayGems displays gems with color-coded output
func displayGems(gems []GemInfo, filter string) {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("246"))

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	if filter != "" {
		fmt.Printf("%s (filter: %q)\n\n", headerStyle.Render("Installed gems"), filter)
	} else {
		fmt.Printf("%s\n\n", headerStyle.Render("Installed gems"))
	}

	if len(gems) == 0 {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))
		fmt.Printf("%s\n", errorStyle.Render("No gems found"))
		return
	}

	// Group by gem name (multiple versions)
	gemVersions := make(map[string][]string)
	for _, gem := range gems {
		gemVersions[gem.Name] = append(gemVersions[gem.Name], gem.Version)
	}

	// Sort gem names
	var gemNames []string
	for name := range gemVersions {
		gemNames = append(gemNames, name)
	}
	sort.Strings(gemNames)

	// Display
	for _, name := range gemNames {
		versions := gemVersions[name]
		if len(versions) == 1 {
			fmt.Printf("  %s %s\n",
				nameStyle.Render(name),
				versionStyle.Render("("+versions[0]+")"))
		} else {
			// Multiple versions
			fmt.Printf("  %s %s\n",
				nameStyle.Render(name),
				versionStyle.Render("("+strings.Join(versions, ", ")+")"))
		}
	}

	fmt.Printf("\n%s %d gems (%d total installations)\n",
		headerStyle.Render("Total:"),
		len(gemNames),
		len(gems))
}

// loadAllGemMetadata loads metadata for all gems in a single Ruby call
func loadAllGemMetadata(gemDir string, gems *[]GemInfo) error {
	if len(*gems) == 0 {
		return nil
	}

	// Build list of all gemspec paths
	var specPaths []string
	for _, gem := range *gems {
		specPath := filepath.Join(gemDir, "specifications", fmt.Sprintf("%s-%s.gemspec", gem.Name, gem.Version))
		if _, err := os.Stat(specPath); err == nil {
			specPaths = append(specPaths, specPath)
		}
	}

	if len(specPaths) == 0 {
		return nil
	}

	// Use Ruby to read all gemspecs at once
	rubyScript := `
require 'rubygems'

ARGV.each do |spec_path|
  begin
    spec = Gem::Specification.load(spec_path)
    next unless spec

    # Output format: NAME|VERSION|SUMMARY
    puts "GEM|#{spec.name}|#{spec.version}|#{spec.summary.to_s.gsub("\n", " ").strip}"

    # Output dependencies: DEP|NAME|REQUIREMENT
    spec.dependencies.each do |dep|
      next unless dep.type == :runtime
      puts "DEP|#{spec.name}|#{spec.version}|#{dep.name}|#{dep.requirement}"
    end
  rescue => e
    # Skip gems that fail to load
  end
end
`

	args := append([]string{"-e", rubyScript}, specPaths...)
	cmd := exec.Command("ruby", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil // Skip on error
	}

	// Parse pipe-delimited output
	// Format: GEM|name|version|summary
	//         DEP|name|version|dep_name|requirement
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Build a map for quick lookup
	gemMap := make(map[string]*GemInfo)
	for i := range *gems {
		key := fmt.Sprintf("%s-%s", (*gems)[i].Name, (*gems)[i].Version)
		gemMap[key] = &(*gems)[i]
	}

	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "GEM":
			if len(parts) >= 4 {
				name, version, summary := parts[1], parts[2], parts[3]
				key := fmt.Sprintf("%s-%s", name, version)
				if gem, ok := gemMap[key]; ok {
					gem.Summary = summary
				}
			}
		case "DEP":
			if len(parts) >= 5 {
				name, version, depName, requirement := parts[1], parts[2], parts[3], parts[4]
				key := fmt.Sprintf("%s-%s", name, version)
				if gem, ok := gemMap[key]; ok {
					gem.Dependencies = append(gem.Dependencies, Dependency{
						Name:         depName,
						Requirements: requirement,
						Type:         "runtime",
					})
				}
			}
		}
	}

	return nil
}
