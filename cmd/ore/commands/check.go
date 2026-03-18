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
	gitGemsDir := filepath.Join(*vendorDir, "bundler", "gems")

	if *verbose {
		fmt.Println("🔍 Checking installed gems...")
		fmt.Printf("Vendor directory: %s\n", *vendorDir)
		fmt.Printf("Gems directory: %s\n", gemsDir)
		fmt.Printf("Git gems directory: %s\n", gitGemsDir)
	}

	// Only require the regular gems directory when the lockfile has regular gems.
	if len(lock.GemSpecs) > 0 {
		if _, err := os.Stat(gemsDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("gems directory does not exist: %s", gemsDir)
			}
			return fmt.Errorf("failed to access gems directory %s: %w", gemsDir, err)
		}
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
	for _, spec := range lock.GitSpecs {
		primaryPath := filepath.Join(gitGemsDir, gitGemDirName(spec.Name, spec.Revision))

		if _, err := os.Stat(primaryPath); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s) [git]", spec.Name, spec.Version))
			if *verbose {
				fmt.Printf("  ✗ %s (%s) [git] - not found at: %s\n", spec.Name, spec.Version, primaryPath)
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
		fmt.Printf("  Git gems directory: %s\n", gitGemsDir)

		if len(lock.GemSpecs) > 0 {
			printDirDebug("gems directory", gemsDir)
		}
		if len(lock.GitSpecs) > 0 {
			printDirDebug("git gems directory", gitGemsDir)
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

func gitGemDirName(name, revision string) string {
	return fmt.Sprintf("%s-%s", name, shortGitRevision(revision))
}

func printDirDebug(label, dir string) {
	stat, err := os.Stat(dir)
	if err != nil {
		fmt.Printf("  ⚠️  %s doesn't exist or is inaccessible: %s\n", label, dir)
		return
	}
	if !stat.IsDir() {
		fmt.Printf("  ⚠️  %s is not a directory: %s\n", label, dir)
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("  ⚠️  Failed to read %s: %v\n", label, err)
		return
	}

	fmt.Printf("  Contents of %s (%d items):\n", label, len(entries))
	for i, entry := range entries {
		if i < 10 {
			fmt.Printf("    - %s\n", entry.Name())
		}
	}
	if len(entries) > 10 {
		fmt.Printf("    ... and %d more\n", len(entries)-10)
	}
}
