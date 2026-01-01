package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/cmd/ore/commands"
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/logger"
	"github.com/contriboss/ore-light/internal/ruby"
)

var (
	version     = "dev"
	buildCommit = "unknown"
)

const (
	// DEFAULT_RUBY_VERSION is the fallback Ruby version when detection fails
	// Update this when new Ruby stable releases come out
	DEFAULT_RUBY_VERSION = "3.4.7"

	// DEFAULT_BUNDLER_VERSION is the Bundler version to write in Gemfile.lock
	// Update this to match the current stable Bundler release
	DEFAULT_BUNDLER_VERSION = "2.7.2"

	// DEFAULT_RUBYGEMS_VERSION is the RubyGems version to write in gemspec files
	// Update this to match the current stable RubyGems release
	DEFAULT_RUBYGEMS_VERSION = "3.6.4"
)

func main() {
	// Ruby developers: This is like parsing ARGV in a Ruby CLI script
	// Go requires explicit length checks - no implicit nil handling like Ruby's ARGV[0]
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	// Check for global --verbose flag anywhere in args and extract command
	verbose := false
	cmd := ""
	args := []string{}

	for _, arg := range os.Args[1:] {
		if arg == "--verbose" {
			verbose = true
		} else if cmd == "" {
			cmd = arg
		} else {
			args = append(args, arg)
		}
	}

	// Setup logger with verbosity level
	logger.SetupLogger(verbose)

	// This is like Ruby's case/when, but switch in Go doesn't fall through by default!
	// In Ruby you need 'when' to match multiple conditions; Go evaluates once and exits.
	// No need for 'break' statements - they're implicit. Use 'fallthrough' for fall-through.
	switch cmd {
	case "--help", "-h", "help":
		printHelp()
	case "--version", "-V", "-v", "version":
		printVersion()
	case "add":
		if err := commands.RunAdd(args); err != nil {
			exitWithError(err)
		}
	case "remove":
		if err := commands.RunRemove(args); err != nil {
			exitWithError(err)
		}
	case "update":
		if err := commands.RunUpdate(args); err != nil {
			exitWithError(err)
		}
	case "outdated":
		if err := commands.RunOutdated(args); err != nil {
			exitWithError(err)
		}
	case "info":
		if err := commands.RunInfo(args); err != nil {
			exitWithError(err)
		}
	case "list":
		if err := commands.RunList(args); err != nil {
			exitWithError(err)
		}
	case "check":
		if err := commands.RunCheck(args); err != nil {
			exitWithError(err)
		}
	case "init":
		if err := commands.RunInit(args); err != nil {
			exitWithError(err)
		}
	case "platform":
		if err := commands.RunPlatform(args); err != nil {
			exitWithError(err)
		}
	case "open":
		if err := commands.RunOpen(args); err != nil {
			exitWithError(err)
		}
	case "show":
		if err := commands.RunShow(args); err != nil {
			exitWithError(err)
		}
	case "clean":
		if err := commands.RunClean(args); err != nil {
			exitWithError(err)
		}
	case "pristine":
		if err := commands.RunPristine(args); err != nil {
			exitWithError(err)
		}
	case "config":
		if err := commands.RunConfig(args); err != nil {
			exitWithError(err)
		}
	case "lock":
		if err := commands.RunLock(args); err != nil {
			exitWithError(err)
		}
	case "self-update", "selfupdate":
		if err := commands.RunSelfUpdate(args, version, buildCommit); err != nil {
			exitWithError(err)
		}
	case "fetch":
		if err := commands.RunFetch(args); err != nil {
			exitWithError(err)
		}
	case "install":
		if err := commands.RunInstallDefault(args); err != nil {
			exitWithError(err)
		}
	case "cache":
		if err := commands.RunCache(args); err != nil {
			exitWithError(err)
		}
	case "completion":
		if err := runCompletionCommand(args); err != nil {
			exitWithError(err)
		}
	case "exec":
		if err := commands.RunExecDefault(args); err != nil {
			exitWithError(err)
		}
	case "tree":
		if err := commands.RunTree(args); err != nil {
			exitWithError(err)
		}
	case "audit":
		if err := commands.RunAudit(args); err != nil {
			exitWithError(err)
		}
	case "stats":
		if err := commands.RunStats(args); err != nil {
			exitWithError(err)
		}
	case "why":
		if err := commands.RunWhy(args); err != nil {
			exitWithError(err)
		}
	case "search":
		if err := commands.RunSearchDefault(args); err != nil {
			exitWithError(err)
		}
	case "gems":
		fs := flag.NewFlagSet("gems", flag.ContinueOnError)
		filter := fs.String("filter", "", "Filter gems by name")
		if err := fs.Parse(args); err != nil {
			exitWithError(err)
		}
		if err := commands.RunGems(*filter); err != nil {
			exitWithError(err)
		}
	case "browse":
		if err := commands.RunBrowse(); err != nil {
			exitWithError(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`ore

Usage: ore [OPTIONS] [COMMAND]

Options:
  -V, --version    Print version info and exit
  -h, --help       Print help

Commands:
    init          Create a new Gemfile
    add           Add gems to Gemfile
    remove        Remove gems from Gemfile
    update        Update gems to their latest versions within constraints
    outdated      List gems with newer versions available
    lock          Regenerate Gemfile.lock from Gemfile
    self-update   Update ore to the latest version
    fetch         Download gems into cache (no Ruby required)
    install       Install gems from Gemfile.lock
    check         Verify all gems are installed
    list          List all gems in the current bundle
    show          Show the source location of a gem
    info          Show detailed information about a gem
    search        Search for gems on RubyGems.org
    why           Show dependency chains for a gem
    exec          Run commands with ore-managed environment
    clean         Remove unused gems from vendor directory
    cache         Inspect or prune the ore gem cache
    pristine      Restore gems to pristine condition (no Ruby required)
    config        Get and set Bundler configuration options
    platform      Display platform compatibility information
    stats         Show Ruby environment statistics
    completion    Generate shell completion scripts
    audit         Audit dependencies for known vulnerabilities

See 'ore <command> --help' for more information on a specific command.
`)
}

func printVersion() {
	fmt.Println(versionInfo())
}

func versionInfo() string {
	hash := shortHash(buildCommit)
	return fmt.Sprintf("ore %s (%s)", version, hash)
}

func shortHash(commit string) string {
	if commit == "" || commit == "unknown" {
		return "unknown"
	}
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func defaultVendorDir() string {
	cfg := configAdapter(appConfig)
	return config.DefaultVendorDir(cfg, detectRubyVersion, getSystemGemDir)
}

func defaultLockfilePath() string {
	return config.DefaultLockfilePath()
}

// configAdapter converts main.Config to internal/config.Config
func configAdapter(c *Config) *config.Config {
	if c == nil {
		return nil
	}
	return &config.Config{
		VendorDir: c.VendorDir,
		CacheDir:  c.CacheDir,
		Gemfile:   c.Gemfile,
	}
}

// toMajorMinor converts "3.4.7" to "3.4.0" (Bundler convention)
func toMajorMinor(version string) string {
	return config.ToMajorMinor(version)
}

// readBundleConfigPath reads BUNDLE_PATH from .bundle/config
// detectRubyVersion detects the Ruby version to use for gem installation
// Priority: 1) Gemfile.lock, 2) Gemfile, 3) DEFAULT_RUBY_VERSION
func detectRubyVersion() string {
	return ruby.DetectRubyVersion(defaultLockfilePath(), defaultGemfilePath(), toMajorMinor, DEFAULT_RUBY_VERSION)
}

// getSystemGemDir returns the system gem directory without requiring Ruby
// Tries: 1) GEM_HOME env, 2) Standard OS paths, 3) User gem dir, 4) gem command
func getSystemGemDir() string {
	return ruby.GetSystemGemDir(detectRubyVersion)
}

func defaultGemfilePath() string {
	return config.DefaultGemfilePath(configAdapter(appConfig))
}

func loadLockfile(lockfilePath string) (*lockfile.Lockfile, error) {
	// Ruby developers: This is like File.open with explicit error handling
	// defer is like Ruby's ensure block but scoped to the current function
	file, err := os.Open(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	parsed, err := lockfile.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	return parsed, nil
}

func loadGemSpecs(lockfilePath string) ([]lockfile.GemSpec, error) {
	parsed, err := loadLockfile(lockfilePath)
	if err != nil {
		return nil, err
	}

	return deduplicateGemSpecs(parsed.GemSpecs), nil
}

func deduplicateGemSpecs(specs []lockfile.GemSpec) []lockfile.GemSpec {
	if len(specs) == 0 {
		return nil
	}

	// This is like Ruby's specs.uniq_by(&:full_name)
	// Go uses maps for deduplication instead of built-in array methods
	unique := make(map[string]lockfile.GemSpec, len(specs))
	for _, spec := range specs {
		unique[spec.FullName()] = spec
	}

	// Convert map back to slice - Go doesn't have .values method
	result := make([]lockfile.GemSpec, 0, len(unique))
	for _, spec := range unique {
		result = append(result, spec)
	}

	return result
}

func defaultCacheDir() (string, error) {
	return config.DefaultCacheDir(configAdapter(appConfig))
}

// downloadManagerAdapter adapts main's downloadManager to commands.DownloadManager interface
type downloadManagerAdapter struct {
	dm *downloadManager
}

func (a *downloadManagerAdapter) CheckSourceHealth(ctx context.Context) {
	a.dm.CheckSourceHealth(ctx)
}

func (a *downloadManagerAdapter) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (commands.DownloadReport, error) {
	report, err := a.dm.DownloadAll(ctx, gems, force)
	if err != nil {
		return commands.DownloadReport{}, err
	}
	return commands.DownloadReport{
		Downloaded: report.Downloaded,
		Skipped:    report.Skipped,
	}, nil
}

func (a *downloadManagerAdapter) CacheDir() string {
	return a.dm.CacheDir()
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func getGemSources() []SourceConfig {
	// Check if user has configured sources in TOML
	if appConfig != nil && len(appConfig.GemSources) > 0 {
		return appConfig.GemSources
	}

	// Default to rubygems.org if no sources configured
	return []SourceConfig{
		{
			URL:      "https://rubygems.org",
			Fallback: "",
		},
	}
}
