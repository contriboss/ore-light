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

	if *verbose {
		fmt.Println("🔍 Checking installed gems...")
	}

	gemsDir := filepath.Join(*vendorDir, "gems")
	missing := []string{}
	installed := 0

	// Check regular gems
	for _, spec := range lock.GemSpecs {
		gemPath := filepath.Join(gemsDir, spec.FullName())
		if _, err := os.Stat(gemPath); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s)", spec.Name, spec.Version))
			if *verbose {
				fmt.Printf("  ✗ %s (%s) - not found\n", spec.Name, spec.Version)
			}
		} else {
			installed++
			if *verbose {
				fmt.Printf("  ✓ %s (%s)\n", spec.Name, spec.Version)
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
		fmt.Printf("\nRun `ore install` to install missing gems.\n")
		return fmt.Errorf("missing %d gem(s)", len(missing))
	}

	fmt.Printf("✅ All gems are installed (%d total)\n", installed)
	return nil
}
