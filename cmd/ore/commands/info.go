package commands

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/contriboss/ore-light/internal/registry"
)

// RunInfo implements the ore info command
func RunInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}

	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	ctx := context.Background()

	for _, gemName := range gems {
		if *verbose {
			fmt.Printf("🔍 Fetching info for %s...\n", gemName)
		}

		// Get versions first
		versions, err := client.GetGemVersions(ctx, gemName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not fetch versions for %s: %v\n", gemName, err)
			continue
		}

		if len(versions) == 0 {
			fmt.Printf("No versions found for gem: %s\n", gemName)
			continue
		}

		// Get info for latest version
		latestVersion := versions[0]
		info, err := client.GetGemInfo(ctx, gemName, latestVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not fetch info for %s: %v\n", gemName, err)
			continue
		}

		// Print gem information
		fmt.Printf("\n*** %s ***\n\n", gemName)
		fmt.Printf("  Latest version: %s\n", latestVersion)

		// Show available versions (limit to 20)
		fmt.Printf("  Versions: %s", versions[0])
		limit := 20
		if len(versions) > limit {
			for i := 1; i < limit; i++ {
				fmt.Printf(", %s", versions[i])
			}
			fmt.Printf(" (+ %d more)\n", len(versions)-limit)
		} else {
			for i := 1; i < len(versions); i++ {
				fmt.Printf(", %s", versions[i])
			}
			fmt.Println()
		}

		// Show dependencies
		runtimeDeps := info.Dependencies.Runtime
		devDeps := info.Dependencies.Development

		if len(runtimeDeps) > 0 {
			fmt.Printf("  Runtime dependencies:\n")
			for _, dep := range runtimeDeps {
				fmt.Printf("    - %s %s\n", dep.Name, dep.Requirements)
			}
		} else {
			fmt.Printf("  Runtime dependencies: (none)\n")
		}

		if len(devDeps) > 0 && *verbose {
			fmt.Printf("  Development dependencies:\n")
			for _, dep := range devDeps {
				fmt.Printf("    - %s %s\n", dep.Name, dep.Requirements)
			}
		}

		fmt.Println()
	}

	return nil
}
