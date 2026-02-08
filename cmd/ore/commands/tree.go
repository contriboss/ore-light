package commands

import (
	"flag"
	"fmt"
	"os"
)

// RunTree implements the ore tree command
func RunTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile (used to derive lockfile path)")
	lockfilePath := fs.String("lockfile", "", "Path to Gemfile.lock")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective lockfile path
	effectiveLockfilePath, err := resolveLockfilePath(*gemfilePath, *lockfilePath)
	if err != nil {
		return err
	}

	parsed, err := loadLockfile(effectiveLockfilePath)
	if err != nil {
		return err
	}

	// Enrich with group information from Gemfile
	effectiveGemfilePath := *gemfilePath
	if effectiveGemfilePath == "" {
		effectiveGemfilePath = detectGemfileFromLock(effectiveLockfilePath)
	}
	if effectiveGemfilePath != "" {
		if err := enrichGemsWithGroups(effectiveGemfilePath, parsed); err != nil {
			// Non-fatal: continue without group info
			fmt.Fprintf(os.Stderr, "Warning: could not read Gemfile groups: %v\n", err)
		}
	}

	// Print tree with colors if TTY, plain if not
	if isTTY() {
		printDependencyTree(parsed.GemSpecs)
	} else {
		printDependencyTreePlain(parsed.GemSpecs)
	}

	return nil
}
