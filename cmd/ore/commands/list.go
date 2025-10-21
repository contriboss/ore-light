package commands

import (
	"flag"
	"fmt"
	"sort"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunList implements the ore list command
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Show gem sources")
	if err := fs.Parse(args); err != nil {
		return err
	}

	lockfilePath := *gemfilePath + ".lock"

	// Parse lockfile
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Collect all gems
	type gemEntry struct {
		name    string
		version string
		source  string
		typ     string // "gem", "git", "path"
	}

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
	fmt.Printf("Gems included in the bundle:\n")
	for _, gem := range allGems {
		if *verbose {
			fmt.Printf("  * %s (%s) [%s]\n", gem.name, gem.version, gem.source)
		} else {
			fmt.Printf("  * %s (%s)\n", gem.name, gem.version)
		}
	}

	fmt.Printf("\nTotal: %d gems\n", len(allGems))

	return nil
}
