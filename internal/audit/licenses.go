package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GemLicense represents a gem and its license information
type GemLicense struct {
	Name     string
	Licenses []string
}

// LicenseReport groups gems by their licenses
type LicenseReport struct {
	Gems map[string][]string // license -> []gem names
}

// ScanLicenses scans installed gems for their license information
func ScanLicenses(vendorDir string) (*LicenseReport, error) {
	// Find the specifications directory
	specDirs, err := findSpecificationDirs(vendorDir)
	if err != nil {
		return nil, err
	}

	if len(specDirs) == 0 {
		return nil, fmt.Errorf("no gem specifications found in %s (run 'ore install' first)", vendorDir)
	}

	// Use map to deduplicate gems by name
	// Prefer entries with license info over those without
	gemMap := make(map[string]GemLicense)

	// Read licenses from each specifications directory
	for _, specDir := range specDirs {
		gems, err := readGemLicenses(specDir)
		if err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to read licenses from %s: %v\n", specDir, err)
			continue
		}

		for _, gem := range gems {
			existing, exists := gemMap[gem.Name]
			if !exists {
				// First occurrence, add it
				gemMap[gem.Name] = gem
			} else if len(existing.Licenses) == 0 && len(gem.Licenses) > 0 {
				// Replace unknown with known license
				gemMap[gem.Name] = gem
			}
			// Otherwise keep existing (prefer first with license info)
		}
	}

	// Build report from deduplicated gems
	report := &LicenseReport{
		Gems: make(map[string][]string),
	}

	for _, gem := range gemMap {
		if len(gem.Licenses) == 0 {
			report.Gems["Unknown"] = append(report.Gems["Unknown"], gem.Name)
		} else {
			// Group by each license (gems can have multiple licenses)
			for _, license := range gem.Licenses {
				report.Gems[license] = append(report.Gems[license], gem.Name)
			}
		}
	}

	// Sort gem names within each license
	for license := range report.Gems {
		sort.Strings(report.Gems[license])
	}

	return report, nil
}

// findSpecificationDirs finds all specifications directories in vendor path
func findSpecificationDirs(vendorDir string) ([]string, error) {
	var specDirs []string

	// Walk vendor directory to find all specifications directories
	err := filepath.WalkDir(vendorDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() && d.Name() == "specifications" {
			specDirs = append(specDirs, path)
		}
		return nil
	})

	return specDirs, err
}

// readGemLicenses reads license information from gemspec files using Ruby
func readGemLicenses(specDir string) ([]GemLicense, error) {
	// Use Ruby to read gemspec files and extract license info
	rubyScript := `
require 'rubygems'
require 'json'

specs_dir = ARGV[0]
results = []

Dir.glob(File.join(specs_dir, '*.gemspec')).each do |gemspec_file|
  begin
    spec = Gem::Specification.load(gemspec_file)
    if spec
      results << {
        name: spec.name,
        licenses: spec.licenses || []
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
		return nil, fmt.Errorf("failed to run Ruby: %w", err)
	}

	var gems []GemLicense
	if err := json.Unmarshal(output, &gems); err != nil {
		return nil, fmt.Errorf("failed to parse Ruby output: %w", err)
	}

	return gems, nil
}

// PrintLicenseReport displays the license report with formatting
func PrintLicenseReport(report *LicenseReport) {
	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	permissiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // Green

	copyleftStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")) // Yellow

	unknownStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")) // Red

	gemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	// Header
	fmt.Println(headerStyle.Render("License Audit"))
	fmt.Println()

	// Categorize licenses
	permissive := []string{}
	copyleft := []string{}
	unknown := []string{}
	other := []string{}

	for license := range report.Gems {
		switch {
		case license == "Unknown":
			unknown = append(unknown, license)
		case isPermissive(license):
			permissive = append(permissive, license)
		case isCopyleft(license):
			copyleft = append(copyleft, license)
		default:
			other = append(other, license)
		}
	}

	sort.Strings(permissive)
	sort.Strings(copyleft)
	sort.Strings(other)

	totalGems := 0

	// Print permissive licenses
	for _, license := range permissive {
		gems := report.Gems[license]
		totalGems += len(gems)
		fmt.Printf("%s (%d gems)\n",
			permissiveStyle.Render(license),
			len(gems))
		fmt.Printf("  %s\n\n", gemStyle.Render(strings.Join(gems, ", ")))
	}

	// Print other licenses
	for _, license := range other {
		gems := report.Gems[license]
		totalGems += len(gems)
		fmt.Printf("%s (%d gems)\n", license, len(gems))
		fmt.Printf("  %s\n\n", gemStyle.Render(strings.Join(gems, ", ")))
	}

	// Print copyleft with warning
	for _, license := range copyleft {
		gems := report.Gems[license]
		totalGems += len(gems)
		fmt.Printf("⚠️  %s (%d gems)\n",
			copyleftStyle.Render(license),
			len(gems))
		fmt.Printf("  %s\n\n", gemStyle.Render(strings.Join(gems, ", ")))
	}

	// Print unknown with error
	for _, license := range unknown {
		gems := report.Gems[license]
		totalGems += len(gems)
		fmt.Printf("❌ %s (%d gems)\n",
			unknownStyle.Render(license),
			len(gems))
		fmt.Printf("  %s\n\n", gemStyle.Render(strings.Join(gems, ", ")))
	}

	// Summary
	fmt.Println(countStyle.Render(fmt.Sprintf("Total: %d gems", totalGems)))
}

// isPermissive checks if a license is permissive
func isPermissive(license string) bool {
	license = strings.ToLower(license)
	permissiveLicenses := []string{
		"mit",
		"apache",
		"bsd",
		"isc",
		"0bsd",
		"cc0",
		"unlicense",
		"ruby",
		"artistic",
	}

	for _, p := range permissiveLicenses {
		if strings.Contains(license, p) {
			return true
		}
	}
	return false
}

// isCopyleft checks if a license is copyleft
func isCopyleft(license string) bool {
	license = strings.ToLower(license)
	copyleftLicenses := []string{
		"gpl",
		"agpl",
		"lgpl",
	}

	for _, c := range copyleftLicenses {
		if strings.Contains(license, c) {
			return true
		}
	}
	return false
}
