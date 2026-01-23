package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunCheck implements the ore check command
func RunCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Vendor directory to check")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w - run 'ore lock' first", err)
	}

	// Parse lockfile
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	gemsDir := filepath.Join(*vendorDir, "gems")

	if *verbose {
		fmt.Println("🔍 Checking installed gems...")
		fmt.Printf("Vendor directory: %s\n", *vendorDir)
		fmt.Printf("Gems directory: %s\n", gemsDir)
	}

	// Verify gems directory exists
	if _, err := os.Stat(gemsDir); os.IsNotExist(err) {
		return fmt.Errorf("gems directory does not exist: %s", gemsDir)
	}

	missing := []string{}
	installed := 0

	// Check regular gems
	for _, spec := range lock.GemSpecs {
		gemPath := filepath.Join(gemsDir, spec.FullName())
		if _, err := os.Stat(gemPath); err != nil {
			// Show full name with platform in error message for clarity
			if spec.Platform != "" && spec.Platform != "ruby" {
				missing = append(missing, fmt.Sprintf("%s (%s-%s)", spec.Name, spec.Version, spec.Platform))
			} else {
				missing = append(missing, fmt.Sprintf("%s (%s)", spec.Name, spec.Version))
			}
			if *verbose {
				fmt.Printf("  ✗ %s - not found at: %s\n", spec.FullName(), gemPath)
			}
		} else {
			installed++
			if *verbose {
				fmt.Printf("  ✓ %s\n", spec.FullName())
			}
		}
	}

	// Check git gems
	for _, spec := range lock.GitSpecs {
		gemPath := filepath.Join(gemsDir, spec.FullName())
		if _, err := os.Stat(gemPath); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s) [git]", spec.Name, spec.Version))
			if *verbose {
				fmt.Printf("  ✗ %s (%s) [git] - not found\n", spec.Name, spec.Version)
			}
		} else {
			installed++
			if *verbose {
				fmt.Printf("  ✓ %s (%s) [git]\n", spec.Name, spec.Version)
			}
		}
	}

	// Check path gems (these should always be available at their source)
	for _, spec := range lock.PathSpecs {
		if _, err := os.Stat(spec.Remote); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s) [path: %s]", spec.Name, spec.Version, spec.Remote))
			if *verbose {
				fmt.Printf("  ✗ %s (%s) [path] - source not found at %s\n", spec.Name, spec.Version, spec.Remote)
			}
		} else {
			installed++
			if *verbose {
				fmt.Printf("  ✓ %s (%s) [path]\n", spec.Name, spec.Version)
			}
		}
	}

	// Print summary
	if len(missing) > 0 {
		fmt.Printf("\n❌ The following gems are missing:\n")
		for _, gem := range missing {
			fmt.Printf("  * %s\n", gem)
		}

		// Enhanced debugging for CI failures
		fmt.Printf("\nDebug Information:\n")
		fmt.Printf("  Gemfile: %s\n", *gemfilePath)
		fmt.Printf("  Lockfile: %s\n", lockfilePath)
		fmt.Printf("  Vendor directory: %s\n", *vendorDir)
		fmt.Printf("  Gems directory: %s\n", gemsDir)

		// Check if gems directory exists
		if stat, err := os.Stat(gemsDir); err != nil {
			fmt.Printf("  ⚠️  Gems directory doesn't exist or is inaccessible\n")
		} else if stat.IsDir() {
			// List what IS in the gems directory
			if entries, err := os.ReadDir(gemsDir); err == nil {
				fmt.Printf("  Contents of gems directory (%d items):\n", len(entries))
				for i, entry := range entries {
					if i < 10 { // Show first 10
						fmt.Printf("    - %s\n", entry.Name())
					}
				}
				if len(entries) > 10 {
					fmt.Printf("    ... and %d more\n", len(entries)-10)
				}
			}
		}

		fmt.Printf("\nRun `ore install` to install missing gems.\n")
		return fmt.Errorf("missing %d gem(s)", len(missing))
	}

	fmt.Printf("✅ All gems are installed (%d total)\n", installed)
	return nil
}
