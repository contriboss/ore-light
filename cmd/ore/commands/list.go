package commands

import (
	"flag"
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/contriboss/gemfile-go/lockfile"
)

type gemEntry struct {
	name    string
	version string
	source  string
	typ     string // "gem", "git", "path"
}

// RunList implements the ore list command
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Show gem sources")
	useTable := fs.Bool("table", false, "Display as table")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse lockfile
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Collect all gems
	var allGems []gemEntry

	// Regular gems
	for _, spec := range lock.GemSpecs {
		source := spec.SourceURL
		if source == "" {
			source = "rubygems.org"
		}
		allGems = append(allGems, gemEntry{
			name:    spec.Name,
			version: spec.Version,
			source:  source,
			typ:     "gem",
		})
	}

	// Git gems
	for _, spec := range lock.GitSpecs {
		source := spec.Remote
		if spec.Branch != "" {
			source += " (branch: " + spec.Branch + ")"
		} else if spec.Tag != "" {
			source += " (tag: " + spec.Tag + ")"
		} else {
			source += " (rev: " + spec.Revision[:7] + ")"
		}
		allGems = append(allGems, gemEntry{
			name:    spec.Name,
			version: spec.Version,
			source:  source,
			typ:     "git",
		})
	}

	// Path gems
	for _, spec := range lock.PathSpecs {
		allGems = append(allGems, gemEntry{
			name:    spec.Name,
			version: spec.Version,
			source:  spec.Remote,
			typ:     "path",
		})
	}

	// Sort by name
	sort.Slice(allGems, func(i, j int) bool {
		return allGems[i].name < allGems[j].name
	})

	// Print gems
	if *useTable {
		printGemsTable(allGems, *verbose)
	} else {
		fmt.Printf("Gems included in the bundle:\n")
		for _, gem := range allGems {
			if *verbose {
				fmt.Printf("  * %s (%s) [%s]\n", gem.name, gem.version, gem.source)
			} else {
				fmt.Printf("  * %s (%s)\n", gem.name, gem.version)
			}
		}
	}

	fmt.Printf("\nTotal: %d gems\n", len(allGems))

	return nil
}

func printGemsTable(gems []gemEntry, verbose bool) {
	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99"))

	gemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))

	sourceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	// Prepare table data
	var rows [][]string
	if verbose {
		rows = make([][]string, 0, len(gems)+1)
		// Header
		rows = append(rows, []string{"GEM", "VERSION", "SOURCE", "TYPE"})
		// Data rows
		for _, gem := range gems {
			typeIcon := getTypeIcon(gem.typ)
			rows = append(rows, []string{
				gemStyle.Render(gem.name),
				versionStyle.Render(gem.version),
				sourceStyle.Render(truncateSource(gem.source)),
				typeIcon,
			})
		}
	} else {
		rows = make([][]string, 0, len(gems)+1)
		// Header
		rows = append(rows, []string{"GEM", "VERSION", "TYPE"})
		// Data rows
		for _, gem := range gems {
			typeIcon := getTypeIcon(gem.typ)
			rows = append(rows, []string{
				gemStyle.Render(gem.name),
				versionStyle.Render(gem.version),
				typeIcon,
			})
		}
	}

	// Create table with new lipgloss v1.1.0 features
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}
			return lipgloss.NewStyle()
		}).
		Headers(rows[0]...).
		Rows(rows[1:]...)

	fmt.Println(t)
}

func getTypeIcon(typ string) string {
	switch typ {
	case "gem":
		return "💎"
	case "git":
		return "📦"
	case "path":
		return "📁"
	default:
		return "•"
	}
}

func truncateSource(source string) string {
	if len(source) > 50 {
		return source[:47] + "..."
	}
	return source
}
