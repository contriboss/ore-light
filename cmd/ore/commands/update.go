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
	lockfilePath := *gemfilePath + ".lock"

	// Parse Gemfile to ensure it exists and is valid
	parser := gemfile.NewGemfileParser(*gemfilePath)
	_, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
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
	if len(gems) == 0 {
		// Update all gems
		if *verbose {
			fmt.Println("🔄 Updating all gems...")
		}
	} else {
		// Update specific gems
		if *verbose {
			fmt.Printf("🔄 Updating gems: %v\n", gems)
		}
	}

	// If we have an existing lockfile, we'll keep versions of gems we're not updating
	// For now, we'll just regenerate the entire lockfile
	// TODO: Implement selective update by pinning non-updated gems
	if existingLock != nil && len(gems) > 0 && *verbose {
		fmt.Println("Note: Currently updates all transitive dependencies. Selective update coming soon.")
	}

	// Regenerate lockfile (this updates everything for now)
	if err := resolver.GenerateLockfile(*gemfilePath); err != nil {
		return fmt.Errorf("failed to update lockfile: %w", err)
	}

	fmt.Printf("✨ Updated %s\n", lockfilePath)
	fmt.Println("💡 Run `ore install` to fetch the updated gems.")
	return nil
}

func defaultGemfilePath() string {
	if env := os.Getenv("ORE_GEMFILE"); env != "" {
		return env
	}
	return "Gemfile"
}
