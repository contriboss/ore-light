package commands

import (
	"flag"
	"fmt"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
)

// RunRemove implements the ore remove command
func RunRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}

	// Find Gemfile
	var gemfileToUse string
	if *gemfilePath != "" {
		gemfileToUse = *gemfilePath
	} else {
		paths, err := lockfile.FindGemfiles()
		if err != nil {
			return fmt.Errorf("failed to find Gemfile: %w", err)
		}
		gemfileToUse = paths.Gemfile
	}

	if *verbose {
		fmt.Printf("🗑️  Removing gems from %s...\n", gemfileToUse)
	}

	// Process each gem
	for _, gemName := range gems {
		// Remove gem from Gemfile using gemfile-go writer
		if err := gemfile.RemoveGemFromFile(gemfileToUse, gemName); err != nil {
			return fmt.Errorf("failed to remove gem %s: %w", gemName, err)
		}

		if *verbose {
			fmt.Printf("✅ Removed gem: %s\n", gemName)
		}
	}

	fmt.Println("✨ Gems removed successfully")
	fmt.Println("💡 Run 'bundle lock' to update Gemfile.lock, then 'ore install' to update vendor")

	return nil
}
