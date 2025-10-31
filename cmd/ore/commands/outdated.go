package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/registry"
)

// versionCheckResult holds the result of checking a gem's latest version
type versionCheckResult struct {
	gemName       string
	latestVersion string
	err           error
}

// checkVersionsParallel fetches latest versions for multiple gems in parallel
// Uses 10 concurrent workers (similar to Bundler's approach but more conservative)
func checkVersionsParallel(ctx context.Context, client *registry.Client, gemNames []string, verbose bool) map[string]versionCheckResult {
	results := make(map[string]versionCheckResult)
	resultsMu := sync.Mutex{}

	// Limit concurrent requests to 10 (respectful to rubygems.org)
	semaphore := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, gemName := range gemNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			versions, err := client.GetGemVersions(ctx, name)

			resultsMu.Lock()
			if err != nil {
				results[name] = versionCheckResult{gemName: name, err: err}
			} else if len(versions) > 0 {
				results[name] = versionCheckResult{gemName: name, latestVersion: versions[0]}
			}
			resultsMu.Unlock()
		}(gemName)
	}

	wg.Wait()
	return results
}

// RunOutdated implements the ore outdated command
func RunOutdated(args []string) error {
	fs := flag.NewFlagSet("outdated", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse Gemfile to get constraints
	parser := gemfile.NewGemfileParser(*gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Parse lockfile to get current versions
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Build map of gem name -> current version
	currentVersions := make(map[string]string)
	for _, spec := range lock.GemSpecs {
		currentVersions[spec.Name] = spec.Version
	}

	// Build map of gem name -> constraint
	constraints := make(map[string]string)
	for _, dep := range parsed.Dependencies {
		if len(dep.Constraints) > 0 {
			constraints[dep.Name] = dep.Constraints[0]
		}
	}

	// Create registry client
	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	ctx := context.Background()

	if *verbose {
		fmt.Println("🔍 Checking for outdated gems...")
	}

	// Collect gem names
	gemNames := make([]string, len(lock.GemSpecs))
	for i, spec := range lock.GemSpecs {
		gemNames[i] = spec.Name
	}

	// Check all versions in parallel
	results := checkVersionsParallel(ctx, client, gemNames, *verbose)

	// Process results and display outdated gems
	outdatedCount := 0
	for _, spec := range lock.GemSpecs {
		result := results[spec.Name]

		if result.err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Warning: Could not fetch versions for %s: %v\n", spec.Name, result.err)
			}
			continue
		}

		if result.latestVersion == "" {
			continue
		}

		// Compare versions
		if result.latestVersion != spec.Version {
			outdatedCount++
			constraint := constraints[spec.Name]
			if constraint == "" {
				constraint = "(no constraint)"
			}

			fmt.Printf("  * %s (newest %s, installed %s, requested %s)\n",
				spec.Name, result.latestVersion, spec.Version, constraint)
		}
	}

	if outdatedCount == 0 {
		fmt.Println("✨ All gems are up to date!")
	} else {
		fmt.Printf("\n%d gem(s) can be updated.\n", outdatedCount)
		fmt.Println("Run `ore update` to update all gems, or `ore update <gem>` for specific gems.")
	}

	return nil
}
