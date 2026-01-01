package commands

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/contriboss/ore-light/internal/cache"
)

// RunCache implements the ore cache command
func RunCache(args []string) error {
	if len(args) == 0 {
		printCacheHelp()
		return nil
	}

	switch args[0] {
	case "info":
		return runCacheInfo(args[1:])
	case "prune":
		return runCachePrune(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown cache subcommand %q\n\n", args[0])
		printCacheHelp()
		return nil
	}
}

func printCacheHelp() {
	fmt.Print(`Usage: ore cache <subcommand>

Subcommands:
  info         Show cache location, size, and gem count
  prune        Remove all cached gems
`)
}

func runCacheInfo(args []string) error {
	fs := flag.NewFlagSet("cache info", flag.ContinueOnError)
	workers := fs.Int("workers", runtime.NumCPU(), "Number of concurrent operations (unused but reserved)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = workers // Reserved for future use

	cacheDir, err := defaultCacheDir()
	if err != nil {
		return err
	}

	stats, err := collectCacheStats(cacheDir)
	if err != nil {
		return err
	}

	fmt.Printf("Cache directory: %s\n", cacheDir)
	fmt.Printf("Cached gems:    %d\n", stats.Files)
	fmt.Printf("Total size:     %s\n", humanBytes(stats.TotalSize))
	return nil
}

func runCachePrune(args []string) error {
	fs := flag.NewFlagSet("cache prune", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be removed without deleting files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cacheDir, err := defaultCacheDir()
	if err != nil {
		return err
	}

	if *dryRun {
		stats, err := collectCacheStats(cacheDir)
		if err != nil {
			return err
		}
		fmt.Printf("[dry-run] Would remove %d files (%s) from %s\n", stats.Files, humanBytes(stats.TotalSize), cacheDir)
		return nil
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to prune cache: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to recreate cache dir: %w", err)
	}

	fmt.Printf("Cache cleared: %s\n", cacheDir)
	return nil
}

type cacheStats = cache.Stats

func collectCacheStats(cacheDir string) (cacheStats, error) {
	return cache.CollectStats(cacheDir)
}

func humanBytes(size int64) string {
	return cache.HumanBytes(size)
}
