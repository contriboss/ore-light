package main

import (
	"flag"
	"fmt"
	"net"
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

type commandSpec struct {
	Name        string
	Aliases     []string
	Description string
	Run         func(args []string) error
}

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

	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		printHelp()
		return
	}
	if cmd == "--version" || cmd == "-V" || cmd == "-v" || cmd == "version" {
		printVersion()
		return
	}

	if spec := lookupCommand(cmd); spec != nil {
		if err := spec.Run(args); err != nil {
			exitWithError(err)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
	printHelp()
	os.Exit(1)
}

func printHelp() {
	fmt.Println("ore")
	fmt.Println("")
	fmt.Println("Usage: ore [OPTIONS] [COMMAND]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -V, --version    Print version info and exit")
	fmt.Println("  -h, --help       Print help")
	fmt.Println("")
	fmt.Println("Commands:")
	for _, spec := range commandSpecs() {
		fmt.Printf("    %-12s %s\n", spec.Name, spec.Description)
	}
	fmt.Println("")
	fmt.Println("See 'ore <command> --help' for more information on a specific command.")
}

func printVersion() {
	fmt.Println(versionInfo())
}

func versionInfo() string {
	hash := shortHash(buildCommit)
	return fmt.Sprintf("ore %s (%s) default-ruby %s", version, hash, ruby.DefaultRubyVersion)
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
	return config.DefaultVendorDir(appConfig, detectRubyVersion, getSystemGemDir)
}

func defaultLockfilePath() string {
	return config.DefaultLockfilePath()
}

// configAdapter converts main.Config to internal/config.Config
// toMajorMinor converts "3.4.7" to "3.4.0" (Bundler convention)
func toMajorMinor(version string) string {
	return config.ToMajorMinor(version)
}

// readBundleConfigPath reads BUNDLE_PATH from .bundle/config
// detectRubyVersion detects the Ruby version to use for gem installation
// Priority: 1) Gemfile.lock, 2) Gemfile, 3) default Ruby version
func detectRubyVersion() string {
	return ruby.DetectRubyVersion(defaultLockfilePath(), defaultGemfilePath(), toMajorMinor, ruby.DefaultRubyVersion)
}

// getSystemGemDir returns the system gem directory without requiring Ruby
// Tries: 1) GEM_HOME env, 2) Standard OS paths, 3) User gem dir, 4) gem command
func getSystemGemDir() string {
	return ruby.GetSystemGemDir(detectRubyVersion)
}

func defaultGemfilePath() string {
	return config.DefaultGemfilePath(appConfig)
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
	return config.DefaultCacheDir(appConfig)
}

func defaultDownloadWorkers() int {
	return config.DefaultDownloadWorkers(appConfig)
}

func defaultHTTPClient(workers int) *http.Client {
	maxConnsPerHost := clampInt(workers, 4, 32)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxConnsPerHost * 4,
		MaxIdleConnsPerHost:   maxConnsPerHost,
		MaxConnsPerHost:       maxConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   150 * time.Second, // 2.5 minutes for large gems
		Transport: transport,
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func getGemSources() []SourceConfig {
	sources, _ := config.ResolveGemSources(appConfig)
	return sources
}

func commandSpecs() []commandSpec {
	return []commandSpec{
		{Name: "init", Description: "Create a new Gemfile", Run: commands.RunInit},
		{Name: "add", Description: "Add gems to Gemfile", Run: commands.RunAdd},
		{Name: "remove", Description: "Remove gems from Gemfile", Run: commands.RunRemove},
		{Name: "update", Description: "Update gems to their latest versions within constraints", Run: commands.RunUpdate},
		{Name: "outdated", Description: "List gems with newer versions available", Run: commands.RunOutdated},
		{Name: "lock", Description: "Regenerate Gemfile.lock from Gemfile", Run: commands.RunLock},
		{Name: "self-update", Aliases: []string{"selfupdate"}, Description: "Update ore to the latest version", Run: func(args []string) error {
			return commands.RunSelfUpdate(args, version, buildCommit)
		}},
		{Name: "fetch", Description: "Download gems into cache (no Ruby required)", Run: commands.RunFetch},
		{Name: "install", Description: "Install gems from Gemfile.lock", Run: commands.RunInstallDefault},
		{Name: "check", Description: "Verify all gems are installed", Run: commands.RunCheck},
		{Name: "list", Description: "List all gems in the current bundle", Run: commands.RunList},
		{Name: "show", Description: "Show the source location of a gem", Run: commands.RunShow},
		{Name: "info", Description: "Show detailed information about a gem", Run: commands.RunInfo},
		{Name: "search", Description: "Search for gems on RubyGems.org", Run: commands.RunSearchDefault},
		{Name: "why", Description: "Show dependency chains for a gem", Run: commands.RunWhy},
		{Name: "exec", Description: "Run commands with ore-managed environment", Run: commands.RunExecDefault},
		{Name: "clean", Description: "Remove unused gems from vendor directory", Run: commands.RunClean},
		{Name: "cache", Description: "Inspect or prune the ore gem cache", Run: commands.RunCache},
		{Name: "pristine", Description: "Restore gems to pristine condition (no Ruby required)", Run: commands.RunPristine},
		{Name: "config", Description: "Get and set Bundler configuration options", Run: commands.RunConfig},
		{Name: "platform", Description: "Display platform compatibility information", Run: commands.RunPlatform},
		{Name: "stats", Description: "Show Ruby environment statistics", Run: commands.RunStats},
		{Name: "completion", Description: "Generate shell completion scripts", Run: runCompletionCommand},
		{Name: "audit", Description: "Audit dependencies for known vulnerabilities", Run: commands.RunAudit},
		{Name: "tree", Description: "Display dependency tree visualization", Run: commands.RunTree},
		{Name: "gems", Description: "List all installed system gems", Run: runGemsCommand},
		{Name: "browse", Description: "Interactive TUI to browse installed gems", Run: func(args []string) error {
			return commands.RunBrowse()
		}},
	}
}

func lookupCommand(name string) *commandSpec {
	specs := commandSpecs()
	for i := range specs {
		if specs[i].Name == name {
			return &specs[i]
		}
		for _, alias := range specs[i].Aliases {
			if alias == name {
				return &specs[i]
			}
		}
	}
	return nil
}

func runGemsCommand(args []string) error {
	fs := flag.NewFlagSet("gems", flag.ContinueOnError)
	filter := fs.String("filter", "", "Filter gems by name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return commands.RunGems(*filter)
}
