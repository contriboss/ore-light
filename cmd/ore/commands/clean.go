package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunClean implements the ore clean command
func RunClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Vendor directory")
	dryRun := fs.Bool("dry-run", false, "Print what would be removed without actually removing")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse lockfile to get gems that should be kept
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Build set of gems that should exist
	keepGems := make(map[string]bool)
	for _, spec := range lock.GemSpecs {
		keepGems[spec.FullName()] = true
	}
	for _, spec := range lock.GitSpecs {
		keepGems[spec.FullName()] = true
	}
	for _, spec := range lock.PathSpecs {
		keepGems[spec.FullName()] = true
	}

	gemsDir := filepath.Join(*vendorDir, "gems")

	// Check if vendor directory exists
	if _, err := os.Stat(gemsDir); os.IsNotExist(err) {
		fmt.Printf("Nothing to clean - %s does not exist\n", gemsDir)
		return nil
	}

	// Find gems to remove
	entries, err := os.ReadDir(gemsDir)
	if err != nil {
		return fmt.Errorf("failed to read gems directory: %w", err)
	}

	var toRemove []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		gemName := entry.Name()
		if !keepGems[gemName] {
			toRemove = append(toRemove, gemName)
		}
	}

	if len(toRemove) == 0 {
		fmt.Println("✨ No unused gems to remove")
		return nil
	}

	// Show what will be removed
	if *dryRun || *verbose {
		fmt.Printf("Gems to remove:\n")
		for _, gemName := range toRemove {
			fmt.Printf("  * %s\n", gemName)
		}
	}

	if *dryRun {
		fmt.Printf("\n[dry-run] Would remove %d gem(s)\n", len(toRemove))
		return nil
	}

	// Remove unused gems
	removed := 0
	failed := 0
	for _, gemName := range toRemove {
		gemPath := filepath.Join(gemsDir, gemName)
		if err := os.RemoveAll(gemPath); err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", gemName, err)
			}
			failed++
		} else {
			if *verbose {
				fmt.Printf("Removed %s\n", gemName)
			}
			removed++
		}
	}

	fmt.Printf("✨ Removed %d unused gem(s)", removed)
	if failed > 0 {
		fmt.Printf(" (%d failed)", failed)
	}
	fmt.Println()

	return nil
}
