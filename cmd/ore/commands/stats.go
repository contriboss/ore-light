package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// RubyVersion represents a Ruby installation with gem count and size
type RubyVersion struct {
	Version  string
	GemCount int
	GemSize  int64
	GemDir   string
	IsActive bool
}

// VersionManager represents a detected Ruby version manager
type VersionManager struct {
	Name     string
	Detected bool
	Path     string
}

// RunStats implements the ore stats command
func RunStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Detect version manager
	manager := detectVersionManager()

	// Detect active Ruby version
	activeVersion := detectActiveRuby()

	// List Ruby versions and count gems
	versions, err := listRubyVersions(manager, activeVersion)
	if err != nil {
		return err
	}

	// Render stats
	renderStats(manager, activeVersion, versions)

	return nil
}

// detectVersionManager detects which Ruby version manager is installed
func detectVersionManager() *VersionManager {
	// Try mise
	if cmd := exec.Command("mise", "--version"); cmd.Run() == nil {
		return &VersionManager{Name: "mise", Detected: true}
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".local", "share", "mise")); err == nil {
			return &VersionManager{Name: "mise", Detected: true}
		}
	}

	// Try rbenv
	if cmd := exec.Command("rbenv", "--version"); cmd.Run() == nil {
		return &VersionManager{Name: "rbenv", Detected: true}
	}
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".rbenv")); err == nil {
			return &VersionManager{Name: "rbenv", Detected: true}
		}
	}

	// Try asdf
	if cmd := exec.Command("asdf", "--version"); cmd.Run() == nil {
		return &VersionManager{Name: "asdf", Detected: true}
	}
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".asdf")); err == nil {
			return &VersionManager{Name: "asdf", Detected: true}
		}
	}

	// Try rvm
	if cmd := exec.Command("rvm", "--version"); cmd.Run() == nil {
		return &VersionManager{Name: "rvm", Detected: true}
	}
	if home != "" {
		if _, err := os.Stat(filepath.Join(home, ".rvm")); err == nil {
			return &VersionManager{Name: "rvm", Detected: true}
		}
	}

	// No manager detected
	return &VersionManager{Name: "None", Detected: false}
}

// detectActiveRuby returns the currently active Ruby version
func detectActiveRuby() string {
	cmd := exec.Command("ruby", "-v")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse "ruby 3.4.7 (2025-10-08 revision ...) [platform]"
	str := string(output)
	if len(str) > 5 && str[:4] == "ruby" {
		// Find version between "ruby " and " ("
		start := 5
		end := start
		for end < len(str) && str[end] != ' ' && str[end] != '(' {
			end++
		}
		if end > start {
			return str[start:end]
		}
	}

	return ""
}

// listRubyVersions lists all installed Ruby versions and counts gems
func listRubyVersions(manager *VersionManager, activeVersion string) ([]RubyVersion, error) {
	if !manager.Detected {
		// Check if system Ruby exists
		if activeVersion == "" {
			return nil, nil
		}

		// Try to get gem directory for system Ruby
		cmd := exec.Command("ruby", "-e", "puts Gem.dir")
		output, err := cmd.Output()
		if err != nil {
			return []RubyVersion{{
				Version:  activeVersion,
				GemCount: 0,
				IsActive: true,
			}}, nil
		}

		gemDir := strings.TrimSpace(string(output))
		count, size, _ := countGems(gemDir)

		return []RubyVersion{{
			Version:  activeVersion,
			GemCount: count,
			GemSize:  size,
			GemDir:   gemDir,
			IsActive: true,
		}}, nil
	}

	switch manager.Name {
	case "mise":
		return listMiseRubies(activeVersion)
	case "rbenv":
		return listRbenvRubies(activeVersion)
	case "asdf":
		return listAsdfRubies(activeVersion)
	case "rvm":
		return listRvmRubies(activeVersion)
	default:
		return nil, nil
	}
}

// listMiseRubies lists Ruby versions installed via mise
func listMiseRubies(activeVersion string) ([]RubyVersion, error) {
	// Try using mise list ruby --json for accurate parsing
	cmd := exec.Command("mise", "list", "ruby", "--json")
	output, err := cmd.Output()

	var versions []RubyVersion

	if err == nil {
		// Parse JSON output
		var miseVersions []struct {
			Version   string `json:"version"`
			Active    bool   `json:"active"`
			Installed bool   `json:"installed"`
		}

		if json.Unmarshal(output, &miseVersions) == nil {
			versions = make([]RubyVersion, 0, len(miseVersions))

			// Use goroutines for concurrent gem counting
			var wg sync.WaitGroup
			versionChan := make(chan RubyVersion, len(miseVersions))

			for _, mv := range miseVersions {
				if !mv.Installed {
					continue
				}

				wg.Go(func() {
					home, err := os.UserHomeDir()
					if err != nil {
						return
					}

					gemDir := findMiseGemDir(home, mv.Version)
					if gemDir == "" {
						versionChan <- RubyVersion{
							Version:  mv.Version,
							GemCount: 0,
							GemSize:  0,
							IsActive: mv.Active || mv.Version == activeVersion,
						}
						return
					}

					count, size, _ := countGems(gemDir)
					versionChan <- RubyVersion{
						Version:  mv.Version,
						GemCount: count,
						GemSize:  size,
						GemDir:   gemDir,
						IsActive: mv.Active || mv.Version == activeVersion,
					}
				})
			}

			wg.Wait()
			close(versionChan)

			for v := range versionChan {
				versions = append(versions, v)
			}

			// Sort versions
			sort.Slice(versions, func(i, j int) bool {
				// Active version first
				if versions[i].IsActive != versions[j].IsActive {
					return versions[i].IsActive
				}
				return versions[i].Version > versions[j].Version
			})

			return versions, nil
		}
	}

	// Fallback: list directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	miseDir := filepath.Join(home, ".local", "share", "mise", "installs", "ruby")
	entries, err := os.ReadDir(miseDir)
	if err != nil {
		return nil, err
	}

	versions = make([]RubyVersion, 0, len(entries))
	var wg sync.WaitGroup
	versionChan := make(chan RubyVersion, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip symlinks
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		wg.Go(func() {
			gemDir := findMiseGemDir(home, entry.Name())
			if gemDir == "" {
				versionChan <- RubyVersion{
					Version:  entry.Name(),
					GemCount: 0,
					GemSize:  0,
					IsActive: entry.Name() == activeVersion,
				}
				return
			}

			count, size, _ := countGems(gemDir)
			versionChan <- RubyVersion{
				Version:  entry.Name(),
				GemCount: count,
				GemSize:  size,
				GemDir:   gemDir,
				IsActive: entry.Name() == activeVersion,
			}
		})
	}

	wg.Wait()
	close(versionChan)

	for v := range versionChan {
		versions = append(versions, v)
	}

	// Sort versions
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].IsActive != versions[j].IsActive {
			return versions[i].IsActive
		}
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// findMiseGemDir finds the gem directory for a mise Ruby version
func findMiseGemDir(home, version string) string {
	baseDir := filepath.Join(home, ".local", "share", "mise", "installs", "ruby", version)

	// Try common gem directory patterns
	patterns := []string{
		filepath.Join(baseDir, "lib", "ruby", "gems", "*"),
		filepath.Join(baseDir, "lib", "jruby", "gems", "*"), // JRuby
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			// Return the first match (usually there's only one)
			return matches[0]
		}
	}

	return ""
}

// listRbenvRubies lists Ruby versions installed via rbenv
func listRbenvRubies(activeVersion string) ([]RubyVersion, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rbenvDir := filepath.Join(home, ".rbenv", "versions")
	return listVersionsFromDir(rbenvDir, activeVersion, "rbenv")
}

// listAsdfRubies lists Ruby versions installed via asdf
func listAsdfRubies(activeVersion string) ([]RubyVersion, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	asdfDir := filepath.Join(home, ".asdf", "installs", "ruby")
	return listVersionsFromDir(asdfDir, activeVersion, "asdf")
}

// listRvmRubies lists Ruby versions installed via rvm
func listRvmRubies(activeVersion string) ([]RubyVersion, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	rvmDir := filepath.Join(home, ".rvm", "rubies")
	versions, err := listVersionsFromDir(rvmDir, activeVersion, "rvm")
	if err != nil {
		return nil, err
	}

	// Strip "ruby-" prefix from rvm versions
	for i := range versions {
		versions[i].Version = strings.TrimPrefix(versions[i].Version, "ruby-")
	}

	return versions, nil
}

// listVersionsFromDir lists Ruby versions from a directory
func listVersionsFromDir(dir string, activeVersion string, manager string) ([]RubyVersion, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	versions := make([]RubyVersion, 0, len(entries))
	var wg sync.WaitGroup
	versionChan := make(chan RubyVersion, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip symlinks
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		wg.Go(func() {
			versionDir := filepath.Join(dir, entry.Name())

			// Find gem directory
			gemDir := findGemDir(versionDir)
			if gemDir == "" {
				versionChan <- RubyVersion{
					Version:  entry.Name(),
					GemCount: 0,
					GemSize:  0,
					IsActive: entry.Name() == activeVersion,
				}
				return
			}

			count, size, _ := countGems(gemDir)
			versionChan <- RubyVersion{
				Version:  entry.Name(),
				GemCount: count,
				GemSize:  size,
				GemDir:   gemDir,
				IsActive: entry.Name() == activeVersion,
			}
		})
	}

	wg.Wait()
	close(versionChan)

	for v := range versionChan {
		versions = append(versions, v)
	}

	// Sort versions
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].IsActive != versions[j].IsActive {
			return versions[i].IsActive
		}
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// findGemDir finds the gem directory for a Ruby installation
func findGemDir(rubyDir string) string {
	// Try common gem directory patterns
	patterns := []string{
		filepath.Join(rubyDir, "lib", "ruby", "gems", "*"),
		filepath.Join(rubyDir, "lib", "jruby", "gems", "*"), // JRuby
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}

	return ""
}

// countGems counts the number of gems and total size in a gem directory
func countGems(gemDir string) (int, int64, error) {
	gemsDir := filepath.Join(gemDir, "gems")
	specDir := filepath.Join(gemDir, "specifications")

	// Count gemspecs
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return 0, 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gemspec") {
			count++
		}
	}

	// Calculate total size of gems directory
	var totalSize int64
	err = filepath.WalkDir(gemsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})

	if err != nil {
		// Non-fatal, just return count
		return count, totalSize, nil
	}

	return count, totalSize, nil
}

// rubyImplementationType detects the Ruby implementation from version string
type rubyImplementationType int

const (
	rubyTypeCRuby rubyImplementationType = iota
	rubyTypeJRuby
	rubyTypeTruffleRuby
	rubyTypeMRuby
)

func detectRubyType(version string) rubyImplementationType {
	lower := strings.ToLower(version)
	if strings.Contains(lower, "jruby") {
		return rubyTypeJRuby
	}
	if strings.Contains(lower, "truffleruby") {
		return rubyTypeTruffleRuby
	}
	if strings.Contains(lower, "mruby") {
		return rubyTypeMRuby
	}
	return rubyTypeCRuby
}

// humanBytes formats bytes as human-readable string
func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

// renderStats renders the statistics using bubbles table
func renderStats(manager *VersionManager, activeVersion string, versions []RubyVersion) {
	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")) // Light gray

	managerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")) // Cyan

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")) // Yellow

	// Render environment info
	fmt.Println(headerStyle.Render("Ruby Environment"))
	fmt.Println()

	managerName := manager.Name
	if !manager.Detected {
		managerName = "None detected"
	}
	fmt.Printf("%s %s\n",
		labelStyle.Render("Manager:"),
		managerStyle.Render(managerName))

	if activeVersion != "" {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Active: "),
			versionStyle.Render(activeVersion))
	} else {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Active: "),
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("(none)"))
	}

	fmt.Println()

	// Render versions table
	if len(versions) == 0 {
		noVersionsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		fmt.Println(noVersionsStyle.Render("No Ruby versions found."))
		return
	}

	fmt.Println(headerStyle.Render("Installed Versions"))
	fmt.Println()

	// Define styles
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10")) // Green

	// Ruby implementation type styles
	crubyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")) // Blue

	jrubyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")) // Orange

	truffleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("13")) // Magenta

	mrubyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")) // Yellow

	// Header
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	fmt.Printf("   %-25s  %-12s  %-15s\n",
		"Version",
		"Gems",
		"Size")
	fmt.Println(grayStyle.Render(strings.Repeat("─", 60)))

	// Render rows
	for _, v := range versions {
		marker := " "

		// Determine style based on Ruby type
		var style lipgloss.Style
		if v.IsActive {
			// Active version: green + bold
			style = activeStyle
			marker = "✓"
		} else {
			// Non-active: color by Ruby type
			rubyType := detectRubyType(v.Version)
			switch rubyType {
			case rubyTypeJRuby:
				style = jrubyStyle
			case rubyTypeTruffleRuby:
				style = truffleStyle
			case rubyTypeMRuby:
				style = mrubyStyle
			default: // CRuby
				style = crubyStyle
			}
		}

		// Format and pad plain text first, then apply styling
		version := fmt.Sprintf("%-25s", v.Version)
		gems := fmt.Sprintf("%-12s", fmt.Sprintf("%d", v.GemCount))
		size := fmt.Sprintf("%-15s", humanBytes(v.GemSize))

		// Apply styling after padding
		versionText := style.Render(version)
		gemText := style.Render(gems)
		sizeText := style.Render(size)

		fmt.Printf(" %s %s  %s  %s\n",
			style.Render(marker),
			versionText,
			gemText,
			sizeText)
	}
}
