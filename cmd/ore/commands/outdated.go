package commands

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/mattn/go-isatty"
)

// RunOutdated implements the ore outdated command
// Auto-detects TTY: shows TUI if interactive terminal, plain text if piped
func RunOutdated(args []string) error {
	fs := flag.NewFlagSet("outdated", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	plainText := fs.Bool("plain", false, "Force plain text output (no TUI)")
	cpuProfile := fs.String("cpuprofile", "", "Write CPU profile to file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// CPU profiling support
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		defer func() { _ = f.Close() }()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Auto-detect TTY: require both stdin and stdout to be terminals for the TUI
	stdoutTTY := isatty.IsTerminal(os.Stdout.Fd())
	stdinTTY := isatty.IsTerminal(os.Stdin.Fd())

	if !*plainText && stdoutTTY && stdinTTY {
		if err := RunOutdatedTUI(*gemfilePath); err == nil {
			return nil
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not start interactive TUI, falling back to plain text output: %v\n", err)
		}
	} else if !*plainText && (!stdoutTTY || !stdinTTY) {
		fmt.Fprintln(os.Stderr, "warning: interactive mode requires a TTY; falling back to plain text output")
	}

	// Plain text output (for pipes, scripts, or --plain flag)
	if *verbose {
		fmt.Println("🔍 Checking for outdated gems...")
	}

	start := time.Now()
	gems, err := LoadOutdatedGems(*gemfilePath)
	if *verbose {
		fmt.Printf("⏱️  Loaded outdated gems in %v\n", time.Since(start))
	}
	if err != nil {
		return err
	}

	if len(gems) == 0 {
		fmt.Println("✨ All gems are up to date!")
		return nil
	}

	// Display outdated gems in plain text
	for _, gem := range gems {
		constraint := gem.Constraint
		if constraint == "" {
			constraint = "(no constraint)"
		}

		fmt.Printf("  * %s (newest %s, installed %s, requested %s)\n",
			gem.Name, gem.LatestVersion, gem.CurrentVersion, constraint)
	}

	fmt.Printf("\n%d gem(s) can be updated.\n", len(gems))
	fmt.Println("Run `ore update` to update all gems, or `ore update <gem>` for specific gems.")

	return nil
}
