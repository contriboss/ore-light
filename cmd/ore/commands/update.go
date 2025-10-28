package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/resolver"
)

// RunUpdate implements the ore update command
func RunUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse Gemfile to ensure it exists and is valid
	parser := gemfile.NewGemfileParser(*gemfilePath)
	_, parseErr := parser.Parse()
	if parseErr != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", parseErr)
	}

	// Check if Gemfile.lock exists
	var existingLock *lockfile.Lockfile
	if _, err := os.Stat(lockfilePath); err == nil {
		existingLock, err = lockfile.ParseFile(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to parse existing Gemfile.lock: %w", err)
		}
	}

	// Determine which gems to update
	var versionPins map[string]string
	if len(gems) == 0 {
		// Update all gems
		if *verbose {
			fmt.Println("🔄 Updating all gems...")
		}
	} else {
		// Update specific gems - pin non-updated gems to their current versions
		if *verbose {
			fmt.Printf("🔄 Updating gems: %v\n", gems)
		}

		if existingLock != nil {
			// Create a map of gems to update for quick lookup
			gemsToUpdate := make(map[string]bool)
			for _, gem := range gems {
				gemsToUpdate[gem] = true
			}

			// Pin all other gems to their current versions
			versionPins = make(map[string]string)
			for _, spec := range existingLock.GemSpecs {
				if !gemsToUpdate[spec.Name] {
					versionPins[spec.Name] = spec.Version
					if *verbose {
						fmt.Printf("  Pinning %s to %s\n", spec.Name, spec.Version)
					}
				}
			}

			// Also pin git and path gems
			for _, spec := range existingLock.GitSpecs {
				if !gemsToUpdate[spec.Name] {
					// Git gems are already pinned by revision in the Gemfile
					if *verbose {
						fmt.Printf("  Keeping git gem %s at revision %s\n", spec.Name, spec.Revision)
					}
				}
			}
		}
	}

	// Regenerate lockfile with version pins for selective update
	if err := resolver.GenerateLockfileWithPins(*gemfilePath, versionPins); err != nil {
		return fmt.Errorf("failed to update lockfile: %w", err)
	}

	fmt.Printf("✨ Updated %s\n", lockfilePath)
	fmt.Println("💡 Run `ore install` to fetch the updated gems.")
	return nil
}
