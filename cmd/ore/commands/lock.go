package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/contriboss/ore-light/internal/resolver"
)

// RunLock implements the ore lock command
func RunLock(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	cpuProfile := fs.String("cpuprofile", "", "Write CPU profile to file")
	allowPrerelease := fs.Bool("prerelease", false, "Allow prerelease versions")

	// Multi-value flag for platforms (like bundle lock --add-platform)
	var platforms []string
	fs.Func("add-platform", "Add a platform to the lockfile (can be repeated)", func(s string) error {
		platforms = append(platforms, s)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective Gemfile path
	effectiveGemfilePath := *gemfilePath
	if effectiveGemfilePath == "" {
		effectiveGemfilePath = defaultGemfilePath()
	}

	// Enable CPU profiling if requested
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return fmt.Errorf("failed to create CPU profile: %w", err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = fmt.Errorf("failed to close CPU profile: %w", cerr)
			}
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	if _, err := os.Stat(effectiveGemfilePath); err != nil {
		return fmt.Errorf("gemfile not found at %s", effectiveGemfilePath)
	}

	if *verbose {
		fmt.Printf("Resolving dependencies from %s...\n", effectiveGemfilePath)
	}

	resolver.SetAllowPrereleases(*allowPrerelease)

	startTime := time.Now()
	if err := resolver.GenerateLockfileWithPlatforms(effectiveGemfilePath, nil, platforms); err != nil {
		return fmt.Errorf("failed to generate lockfile: %w", err)
	}
	elapsed := time.Since(startTime)

	if *cpuProfile != "" {
		fmt.Printf("Resolution took: %v\n", elapsed)
	}

	// Determine lockfile path for display
	lockfilePath := effectiveGemfilePath + ".lock"
	if filepath.Base(effectiveGemfilePath) == "gems.rb" {
		lockfilePath = filepath.Join(filepath.Dir(effectiveGemfilePath), "gems.locked")
	}

	if *verbose {
		fmt.Printf("Updated %s\n", lockfilePath)
	} else {
		fmt.Printf("Wrote %s\n", lockfilePath)
	}

	fmt.Println("Run `ore install` to fetch the resolved gems.")
	return nil
}
