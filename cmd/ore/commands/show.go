package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
)

// RunShow implements the ore show command
func RunShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Vendor directory")
	paths := fs.Bool("paths", false, "List all gem paths")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse lockfile
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// If --paths flag, list all gem paths
	if *paths {
		gemsDir := filepath.Join(*vendorDir, "gems")

		// Regular gems
		for _, spec := range lock.GemSpecs {
			gemPath := filepath.Join(gemsDir, spec.FullName())
			if _, err := os.Stat(gemPath); err == nil {
				fmt.Println(gemPath)
			}
		}

		// Git gems
		for _, spec := range lock.GitSpecs {
			gemPath := filepath.Join(gemsDir, spec.FullName())
			if _, err := os.Stat(gemPath); err == nil {
				fmt.Println(gemPath)
			}
		}

		// Path gems (show source path, not vendor copy)
		for _, spec := range lock.PathSpecs {
			if absPath, err := filepath.Abs(spec.Remote); err == nil {
				fmt.Println(absPath)
			} else {
				fmt.Println(spec.Remote)
			}
		}

		return nil
	}

	// Show specific gem path
	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("gem name required (or use --paths to list all)")
	}

	gemName := gems[0]
	gemsDir := filepath.Join(*vendorDir, "gems")

	// Search in regular gems
	for _, spec := range lock.GemSpecs {
		if spec.Name == gemName {
			gemPath := filepath.Join(gemsDir, spec.FullName())
			if _, err := os.Stat(gemPath); err == nil {
				if absPath, err := filepath.Abs(gemPath); err == nil {
					fmt.Println(absPath)
				} else {
					fmt.Println(gemPath)
				}
				return nil
			}
			return fmt.Errorf("gem %s is in lockfile but not installed", gemName)
		}
	}

	// Search in git gems
	for _, spec := range lock.GitSpecs {
		if spec.Name == gemName {
			gemPath := filepath.Join(gemsDir, spec.FullName())
			if _, err := os.Stat(gemPath); err == nil {
				fmt.Println(gemPath)
				return nil
			}
			return fmt.Errorf("gem %s is in lockfile but not installed", gemName)
		}
	}

	// Search in path gems
	for _, spec := range lock.PathSpecs {
		if spec.Name == gemName {
			if absPath, err := filepath.Abs(spec.Remote); err == nil {
				fmt.Println(absPath)
			} else {
				fmt.Println(spec.Remote)
			}
			return nil
		}
	}

	return fmt.Errorf("gem %s not found in bundle", gemName)
}
