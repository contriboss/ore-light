package commands

import (
	"flag"
	"fmt"
	"os"
)

// RunTree implements the ore tree command
func RunTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	lockfilePath := fs.String("lockfile", defaultLockfilePath(), "Path to Gemfile.lock")
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsed, err := loadLockfile(*lockfilePath)
	if err != nil {
		return err
	}

	// Enrich with group information from Gemfile
	gemfilePath := detectGemfileFromLock(*lockfilePath)
	if gemfilePath != "" {
		if err := enrichGemsWithGroups(gemfilePath, parsed); err != nil {
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
