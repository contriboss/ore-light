package commands

import (
	"flag"
	"fmt"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/ore-light/internal/resolver"
)

// RunUpdate implements the ore update command
func RunUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective Gemfile path
	effectiveGemfilePath := *gemfilePath
	if effectiveGemfilePath == "" {
		effectiveGemfilePath = defaultGemfilePath()
	}

	gems := fs.Args()

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(effectiveGemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse Gemfile to ensure it exists and is valid
	parser := gemfile.NewGemfileParser(effectiveGemfilePath)
	_, parseErr := parser.Parse()
	if parseErr != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", parseErr)
	}

	// Determine which gems to update
	var versionPins map[string]string
	if len(gems) == 0 {
		// Update all gems - no pins
		if *verbose {
			fmt.Println("🔄 Updating all gems...")
		}
	} else {
		// Selective update for specific gems
		// For now, just re-resolve without any pins
		// TODO: Implement conservative update strategy
		if *verbose {
			fmt.Printf("🔄 Updating gems: %v (re-resolving all dependencies)\n", gems)
		}
	}

	// Regenerate lockfile with version pins for selective update
	if err := resolver.GenerateLockfileWithPins(effectiveGemfilePath, versionPins); err != nil {
		return fmt.Errorf("failed to update lockfile: %w", err)
	}

	fmt.Printf("✨ Updated %s\n", lockfilePath)
	fmt.Println("💡 Run `ore install` to fetch the updated gems.")
	return nil
}
