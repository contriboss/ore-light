package commands

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
)

// RunPlatform implements the ore platform command
func RunPlatform(args []string) error {
	fs := flag.NewFlagSet("platform", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	rubyOnly := fs.Bool("ruby", false, "Display only Ruby version requirement")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find the lockfile - supports both Gemfile.lock and gems.locked
	lockfilePath, err := findLockfilePath(*gemfilePath)
	if err != nil {
		return fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Get current platform
	currentPlatform := detectCurrentPlatform()

	if *rubyOnly {
		// Just show Ruby requirement from Gemfile
		parser := gemfile.NewGemfileParser(*gemfilePath)
		parsed, err := parser.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse Gemfile: %w", err)
		}

		if parsed.RubyVersion != "" {
			fmt.Println(parsed.RubyVersion)
		}
		return nil
	}

	// Parse Gemfile for Ruby requirement
	var rubyRequirement string
	parser := gemfile.NewGemfileParser(*gemfilePath)
	parsed, err := parser.Parse()
	if err == nil && parsed.RubyVersion != "" {
		rubyRequirement = parsed.RubyVersion
	}

	// Parse lockfile for platforms
	var platforms []string
	if _, err := os.Stat(lockfilePath); err == nil {
		lock, err := lockfile.ParseFile(lockfilePath)
		if err == nil {
			platforms = lock.Platforms
		}
	}

	// Display information
	fmt.Printf("Your platform is: %s\n", currentPlatform)

	if len(platforms) > 0 {
		fmt.Println("\nYour app has gems that work on these platforms:")
		for _, platform := range platforms {
			fmt.Printf("* %s\n", platform)
		}
	}

	if rubyRequirement != "" {
		fmt.Println("\nYour Gemfile specifies a Ruby version requirement:")
		fmt.Printf("* ruby %s\n", rubyRequirement)

		// Check if current Ruby matches
		currentRubyVersion := detectCurrentRubyVersion()
		if currentRubyVersion != "" {
			if currentRubyVersion == rubyRequirement {
				fmt.Println("\nYour current platform satisfies the Ruby version requirement.")
			} else {
				fmt.Printf("\nYour Ruby version is %s, but your Gemfile specified %s\n",
					currentRubyVersion, rubyRequirement)
			}
		}
	} else {
		fmt.Println("\nYour Gemfile does not specify a Ruby version requirement.")
	}

	return nil
}

func detectCurrentPlatform() string {
	// Try to get Ruby platform first
	cmd := exec.Command("ruby", "-e", "puts RUBY_PLATFORM")
	output, err := cmd.Output()
	if err == nil {
		platform := regexp.MustCompile(`\s+`).ReplaceAllString(string(output), "")
		if platform != "" && platform != "ruby" {
			return platform
		}
	}

	// Fallback to Go's runtime detection
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map to Ruby-style platform names
	switch goos {
	case "darwin":
		return goarch + "-darwin"
	case "linux":
		return goarch + "-linux"
	case "windows":
		return goarch + "-mingw32"
	default:
		return goarch + "-" + goos
	}
}

func detectCurrentRubyVersion() string {
	cmd := exec.Command("ruby", "-e", "puts RUBY_VERSION")
	output, err := cmd.Output()
	if err == nil {
		version := regexp.MustCompile(`\s+`).ReplaceAllString(string(output), "")
		return version
	}
	return ""
}
