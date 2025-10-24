package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
)

// TestSimpleGemfileParsing verifies we can parse a Gemfile using the shared gemfile-go module.
// This mirrors the parsing logic used in the original ore_reference codebase.
func TestSimpleGemfileParsing(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "simple_app")
	gemfilePath := filepath.Join(fixtureDir, "Gemfile")

	parser := gemfile.NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("failed to parse Gemfile fixture: %v", err)
	}

	if len(parsed.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(parsed.Dependencies))
	}

	var (
		foundRake     bool
		foundRack     bool
		foundMinitest bool
	)

	for _, dep := range parsed.Dependencies {
		switch dep.Name {
		case "rake":
			foundRake = true
		case "rack":
			foundRack = true
			if len(dep.Constraints) != 1 || dep.Constraints[0] != "~> 3.0" {
				t.Fatalf("expected rack constraint \"~> 3.0\", got %v", dep.Constraints)
			}
		case "minitest":
			foundMinitest = true
		}
	}

	if !foundRake {
		t.Fatalf("expected to find rake dependency in parsed Gemfile")
	}

	if !foundRack {
		t.Fatalf("expected to find rack dependency in parsed Gemfile")
	}

	if !foundMinitest {
		t.Fatalf("expected to find minitest dependency in parsed Gemfile")
	}
}

func TestVersionInfo(t *testing.T) {
	info := versionInfo()
	if !strings.Contains(info, "ore version") {
		t.Fatalf("expected version info string, got %q", info)
	}
	if !strings.Contains(info, version) {
		t.Fatalf("expected version string %q in info %q", version, info)
	}
}

func TestConfigOverrides(t *testing.T) {
	origCfg := appConfig
	appConfig = &Config{
		VendorDir: "/tmp/vendor-test",
		CacheDir:  "/tmp/cache-test",
		GemSources: []SourceConfig{
			{URL: "https://mirror.test", Fallback: ""},
		},
		Gemfile: "/tmp/CustomGemfile",
	}
	t.Cleanup(func() { appConfig = origCfg })

	type envState struct {
		key     string
		value   string
		present bool
	}

	var states []envState
	for _, key := range []string{
		"ORE_VENDOR_DIR",
		"ORE_LIGHT_VENDOR_DIR",
		"ORE_CACHE_DIR",
		"ORE_LIGHT_CACHE_DIR",
		"ORE_GEM_MIRROR",
		"ORE_LIGHT_GEM_MIRROR",
	} {
		value, present := os.LookupEnv(key)
		states = append(states, envState{key: key, value: value, present: present})
		_ = os.Unsetenv(key)
	}

	t.Cleanup(func() {
		for _, state := range states {
			if state.present {
				_ = os.Setenv(state.key, state.value)
			} else {
				_ = os.Unsetenv(state.key)
			}
		}
	})

	if got := defaultVendorDir(); got != "/tmp/vendor-test" {
		t.Fatalf("expected vendor dir from config, got %s", got)
	}

	cache, err := defaultCacheDir()
	if err != nil {
		t.Fatalf("defaultCacheDir returned error: %v", err)
	}
	if cache != "/tmp/cache-test" {
		t.Fatalf("expected cache dir from config, got %s", cache)
	}

	sources := getGemSources()
	if len(sources) != 1 || sources[0].URL != "https://mirror.test" {
		t.Fatalf("expected gem source from config, got %v", sources)
	}

	if gemfile := defaultGemfilePath(); gemfile != "/tmp/CustomGemfile" {
		t.Fatalf("expected gemfile from config, got %s", gemfile)
	}
}

func TestRunLockCommandMissingGemfile(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "Gemfile")

	err := runLockCommand([]string{"--gemfile", missing})
	if err == nil || !strings.Contains(err.Error(), "Gemfile not found") {
		t.Fatalf("expected missing Gemfile error, got %v", err)
	}
}

func TestLoadGemSpecs(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	lockfilePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "simple_app", "Gemfile.lock")

	specs, err := loadGemSpecs(lockfilePath)
	if err != nil {
		t.Fatalf("loadGemSpecs returned error: %v", err)
	}

	if len(specs) != 3 {
		t.Fatalf("expected 3 gems from lockfile, got %d", len(specs))
	}

	gemNames := map[string]bool{}
	for _, spec := range specs {
		gemNames[spec.Name] = true
	}

	if !gemNames["rack"] || !gemNames["rake"] || !gemNames["minitest"] {
		t.Fatalf("expected rack, rake, and minitest in gem specs, got %v", gemNames)
	}

	// Quick sanity check: downloading with force=false should skip already cached non-existent case,
	// so ensure the download manager builds a cache path without touching network.
	cacheDir := t.TempDir()
	sourceConfigs := []SourceConfig{
		{URL: "https://example.com", Fallback: ""},
	}
	dm, err := newDownloadManager(cacheDir, sourceConfigs, defaultHTTPClient(), 1)
	if err != nil {
		t.Fatalf("unexpected error creating download manager: %v", err)
	}

	// Force skip to avoid network; we just ensure it reports skipped when the file exists.
	fakeGem := specs[0]
	cachePath := dm.cachePathFor(fakeGem)
	if err := os.WriteFile(cachePath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to create stub cache file: %v", err)
	}

	report, err := dm.DownloadAll(context.Background(), []lockfile.GemSpec{fakeGem}, false)
	if err != nil {
		t.Fatalf("downloadAll returned error for cached gem: %v", err)
	}

	if report.Downloaded != 0 || report.Skipped != 1 {
		t.Fatalf("expected 0 downloaded, 1 skipped; got %+v", report)
	}
}

func TestInstallFromCache(t *testing.T) {
	cacheDir := t.TempDir()
	vendorDir := filepath.Join(t.TempDir(), "vendor")

	spec := lockfile.GemSpec{
		Name:    "fake",
		Version: "0.1.0",
	}

	gemPath := filepath.Join(cacheDir, gemFileName(spec))
	payload := map[string][]byte{
		"lib/fake.rb": []byte("module Fake; def self.hello; 'world'; end; end"),
		"bin/fake":    []byte("#!/usr/bin/env ruby\nputs 'fake'\n"),
	}

	rubyPath, err := exec.LookPath("ruby")
	rubyAvailable := err == nil
	var marshalData []byte
	if rubyAvailable {
		genScript := fmt.Sprintf(`require 'rubygems'
spec = Gem::Specification.new do |s|
  s.name = '%s'
  s.version = '%s'
  s.summary = 'Fake gem for ore-light tests'
  s.description = 'Fake gem for ore-light tests'
  s.authors = ['Ore Light']
  s.email = ['ore@example.com']
  s.files = ['lib/fake.rb']
  s.require_paths = ['lib']
end
STDOUT.binmode
 STDOUT.write(Marshal.dump(spec))
`, spec.Name, spec.Version)
		cmd := exec.Command(rubyPath, "-e", genScript)
		if marshalData, err = cmd.Output(); err != nil {
			t.Logf("failed to generate marshal metadata via ruby: %v", err)
			rubyAvailable = false
			marshalData = nil
		}
	}

	if err := createFakeGemArchive(gemPath, payload, marshalData); err != nil {
		t.Fatalf("failed to create fake gem archive: %v", err)
	}

	ctx := context.Background()
	extConfig := &extensions.BuildConfig{SkipExtensions: true}
	report, err := installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{spec}, false, extConfig)
	if err != nil {
		t.Fatalf("installFromCache returned error: %v", err)
	}
	if report.Installed != 1 || report.Skipped != 0 {
		t.Fatalf("unexpected install report: %+v", report)
	}

	libFile := filepath.Join(vendorDir, "gems", spec.FullName(), "lib", "fake.rb")
	if _, err := os.Stat(libFile); err != nil {
		t.Fatalf("expected lib file to exist: %v", err)
	}

	binWrapper := filepath.Join(vendorDir, "bin", "fake")
	if data, err := os.ReadFile(binWrapper); err != nil {
		t.Fatalf("expected bin wrapper to exist: %v", err)
	} else if !strings.Contains(string(data), "#!/usr/bin/env ruby") {
		t.Fatalf("expected bin/fake to be a Ruby wrapper script, got: %s", data)
	}

	marshalPath := filepath.Join(vendorDir, "specifications", "cache", fmt.Sprintf("%s.gemspec.marshal", spec.FullName()))
	if data, err := os.ReadFile(marshalPath); err != nil {
		t.Fatalf("expected marshal cache to exist: %v", err)
	} else if len(data) == 0 {
		t.Fatalf("marshal cache is empty")
	}

	specPath := filepath.Join(vendorDir, "specifications", fmt.Sprintf("%s.gemspec", spec.FullName()))
	if data, err := os.ReadFile(specPath); err != nil {
		t.Fatalf("expected gemspec shim to exist: %v", err)
	} else if !strings.Contains(string(data), "Marshal.load") {
		t.Fatalf("expected gemspec shim to load marshal data, got: %s", data)
	}

	// Second install without --force should skip
	report, err = installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{spec}, false, extConfig)
	if err != nil {
		t.Fatalf("second installFromCache returned error: %v", err)
	}
	if report.Installed != 0 || report.Skipped != 1 {
		t.Fatalf("expected skip on second install, got %+v", report)
	}

	// Force reinstall should re-extract
	report, err = installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{spec}, true, extConfig)
	if err != nil {
		t.Fatalf("forced installFromCache returned error: %v", err)
	}
	if report.Installed != 1 {
		t.Fatalf("expected reinstall with force, got %+v", report)
	}

	// Optional Ruby smoke test (skipped if ruby is unavailable)
	if rubyAvailable && len(marshalData) > 0 {
		env, err := buildExecutionEnv(vendorDir, []lockfile.GemSpec{spec})
		if err != nil {
			t.Fatalf("buildExecutionEnv failed: %v", err)
		}

		script := fmt.Sprintf(`require 'rubygems'
Gem::Specification.reset
spec = Gem::Specification.find_by_name('%s', '%s')
abort('missing spec') unless spec && spec.full_name == '%s'
puts spec.full_name
`, spec.Name, spec.Version, spec.FullName())

		cmd := exec.Command(rubyPath, "-e", script)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "Gem::SafeMarshal") {
				t.Skipf("ruby safe marshal check unavailable: %v\nOutput: %s", err, output)
			}
			t.Fatalf("ruby smoke test failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), spec.FullName()) {
			t.Fatalf("ruby smoke test did not confirm spec load, output: %s", output)
		}
	}
}
