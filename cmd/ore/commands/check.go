package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunCheck implements the ore check command
func RunCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile (used to derive lockfile path)")
	lockfilePath := fs.String("lockfile", "", "Path to Gemfile.lock")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Vendor directory to check")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective lockfile path
	finalLockfilePath, err := resolveLockfilePath(*gemfilePath, *lockfilePath)
	if err != nil {
		return err
	}

	// Parse lockfile
	lock, err := lockfile.ParseLockfile(finalLockfilePath)
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
	// Bundler stores git gems under <gem_home>/bundler/gems/<name>-<revision[:12]>
	// (not under the regular <gem_home>/gems directory).
	gitGemsDir := filepath.Join(*vendorDir, "bundler", "gems")
	for _, spec := range lock.GitSpecs {
		primaryName := spec.FullName()
		if spec.Revision != "" {
			primaryName = fmt.Sprintf("%s-%s", spec.Name, shortGitRevision(spec.Revision))
		}
		primaryPath := filepath.Join(gitGemsDir, primaryName)
		legacyPath := filepath.Join(gitGemsDir, spec.FullName())

		if _, err := os.Stat(primaryPath); err != nil {
			// Backward compatibility: older layouts/tests may still use FullName()
			if _, legacyErr := os.Stat(legacyPath); legacyErr != nil {
				missing = append(missing, fmt.Sprintf("%s (%s) [git]", spec.Name, spec.Version))
				if *verbose {
					fmt.Printf("  ✗ %s (%s) [git] - not found at: %s (or %s)\n", spec.Name, spec.Version, primaryPath, legacyPath)
				}
			} else {
				installed++
				if *verbose {
					fmt.Printf("  ✓ %s (%s) [git]\n", spec.Name, spec.Version)
				}
			}
		} else {
			installed++
			if *verbose {
				fmt.Printf("  ✓ %s (%s) [git]\n", spec.Name, spec.Version)
			}
		}
	}

	// Check path gems (these should always be available at their source)
	lockfileDir := filepath.Dir(finalLockfilePath)
	for _, spec := range lock.PathSpecs {
		remotePath := spec.Remote
		if !filepath.IsAbs(remotePath) && lockfileDir != "" {
			remotePath = filepath.Join(lockfileDir, remotePath)
		}
		if _, err := os.Stat(remotePath); err != nil {
			// Show resolved path in summary for clarity
			missing = append(missing, fmt.Sprintf("%s (%s) [path: %s (resolved to: %s)]", spec.Name, spec.Version, spec.Remote, remotePath))
			if *verbose {
				fmt.Printf("  ✗ %s (%s) [path] - source not found at %s\n", spec.Name, spec.Version, remotePath)
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
		fmt.Printf("  Lockfile: %s\n", finalLockfilePath)
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

func shortGitRevision(rev string) string {
	rev = strings.TrimSpace(rev)
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
