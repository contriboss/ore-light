package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	rubygemsclient "github.com/contriboss/rubygems-client-go"
)

// RunOutdated implements the ore outdated command
func RunOutdated(args []string) error {
	fs := flag.NewFlagSet("outdated", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	lockfilePath := *gemfilePath + ".lock"

	// Parse Gemfile to get constraints
	parser := gemfile.NewGemfileParser(*gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Parse lockfile to get current versions
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Build map of gem name -> current version
	currentVersions := make(map[string]string)
	for _, spec := range lock.GemSpecs {
		currentVersions[spec.Name] = spec.Version
	}

	// Build map of gem name -> constraint
	constraints := make(map[string]string)
	for _, dep := range parsed.Dependencies {
		if len(dep.Constraints) > 0 {
			constraints[dep.Name] = dep.Constraints[0]
		}
	}

	// Create RubyGems client
	client := rubygemsclient.NewClient()

	if *verbose {
		fmt.Println("🔍 Checking for outdated gems...")
	}

	outdatedCount := 0
	checkedCount := 0

	// Check each gem in lockfile for updates
	for _, spec := range lock.GemSpecs {
		checkedCount++
		if *verbose && checkedCount%10 == 0 {
			fmt.Printf("   Checked %d/%d gems...\n", checkedCount, len(lock.GemSpecs))
		}

		versions, err := client.GetGemVersions(spec.Name)
		if err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Warning: Could not fetch versions for %s: %v\n", spec.Name, err)
			}
			continue
		}

		if len(versions) == 0 {
			continue
		}

		// Latest version is first in the list
		latestVersion := versions[0]

		// Compare versions
		if latestVersion != spec.Version {
			outdatedCount++
			constraint := constraints[spec.Name]
			if constraint == "" {
				constraint = "(no constraint)"
			}

			fmt.Printf("  * %s (newest %s, installed %s, requested %s)\n",
				spec.Name, latestVersion, spec.Version, constraint)
		}
	}

	if outdatedCount == 0 {
		fmt.Println("✨ All gems are up to date!")
	} else {
		fmt.Printf("\n%d gem(s) can be updated.\n", outdatedCount)
		fmt.Println("Run `ore update` to update all gems, or `ore update <gem>` for specific gems.")
	}

	return nil
}
