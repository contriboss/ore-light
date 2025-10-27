package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/resolver"
	"gopkg.in/yaml.v3"
)

// Ruby developers: This is like a result object from bundle install
// Tracks what was installed, skipped, and extension build results
type installReport struct {
	Total             int
	Installed         int
	Skipped           int
	ExtensionsBuilt   int
	ExtensionsSkipped int
	ExtensionsFailed  int
}

func installFromCache(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gems)}

	if err := ensureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}
	if err := ensureDir(filepath.Join(vendorDir, "cache")); err != nil {
		return report, err
	}
	if err := ensureDir(filepath.Join(vendorDir, "bin")); err != nil {
		return report, err
	}
	if err := ensureDir(filepath.Join(vendorDir, "specifications", "cache")); err != nil {
		return report, err
	}

	// Create extension builder
	extBuilder := extensions.NewBuilder(extConfig)

	for _, gem := range gems {
		gemPath := findGemInCaches(cacheDir, gem)
		if gemPath == "" {
			return report, fmt.Errorf("gem %s is not cached; run `ore download` first", gem.FullName())
		}

		destDir := filepath.Join(vendorDir, "gems", gem.FullName())

		if _, err := os.Stat(destDir); err == nil && !force {
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gem.FullName(), err)
		}

		metadata, err := extractGemContents(gemPath, destDir)
		if err != nil {
			return report, fmt.Errorf("failed to extract %s: %w", gem.FullName(), err)
		}

		if err := copyGemToVendorCache(gemPath, filepath.Join(vendorDir, "cache", gemFileName(gem))); err != nil {
			return report, err
		}

		if len(metadata) > 0 {
			if err := writeGemSpecification(vendorDir, gem, metadata); err != nil {
				return report, err
			}
		}

		if err := linkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gem.FullName())
		if err != nil {
			// Extension build failure - warn but continue
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gem.FullName(), err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gem.FullName(), extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

	return report, nil
}

// findGemInCaches searches for a gem in cache directories (ore cache + system cache)
func findGemInCaches(primaryCache string, gem lockfile.GemSpec) string {
	fileName := gemFileName(gem)

	// Check primary ore cache first
	path := filepath.Join(primaryCache, fileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Try system RubyGems caches (only if Ruby available)
	if gemPaths := tryGetGemPathsForInstall(); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			systemCache := filepath.Join(gemPath, "cache", fileName)
			if _, err := os.Stat(systemCache); err == nil {
				return systemCache
			}
		}
	}

	return ""
}

// tryGetGemPathsForInstall uses same logic as download.go
func tryGetGemPathsForInstall() []string {
	cmd := exec.Command("gem", "environment", "gempath")
	output, err := cmd.Output()
	if err == nil {
		pathsStr := strings.TrimSpace(string(output))
		if pathsStr != "" {
			return strings.Split(pathsStr, string(filepath.ListSeparator))
		}
	}

	// Fallback to common locations
	var defaultPaths []string
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultPaths
	}

	rubyVer := detectRubyVersion()
	if rubyVer == "" {
		return defaultPaths
	}

	commonLocations := []string{
		filepath.Join(home, ".gem", "ruby", rubyVer),
		filepath.Join(home, ".local", "share", "gem", "ruby", rubyVer),
	}

	globPatterns := []string{
		filepath.Join(home, ".rbenv", "versions", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".asdf", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".local", "share", "mise", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
	}

	for _, pattern := range globPatterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			defaultPaths = append(defaultPaths, matches...)
		}
	}

	for _, location := range commonLocations {
		if _, err := os.Stat(location); err == nil {
			defaultPaths = append(defaultPaths, location)
		}
	}

	return defaultPaths
}

func extractGemContents(gemPath, destDir string) ([]byte, error) {
	file, err := os.Open(gemPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	var dataFound bool
	var metadata []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch header.Name {
		case "data.tar.gz":
			dataFound = true
			if err := extractDataTar(tr, destDir); err != nil {
				return nil, err
			}
		case "metadata.gz":
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			meta, err := decompressMetadata(buf)
			if err != nil {
				return nil, err
			}
			metadata = meta
		case "metadata":
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			metadata = buf
		case "data.tar.zst", "data.tar.bz2", "data.tar.xz":
			return nil, fmt.Errorf("unsupported gem payload compression (%s) for now", header.Name)
		}
	}

	if !dataFound {
		return nil, fmt.Errorf("data.tar.gz not found in %s", gemPath)
	}

	if metadata == nil {
		return nil, fmt.Errorf("metadata not found in %s", gemPath)
	}

	return metadata, nil
}

func extractDataTar(reader io.Reader, destDir string) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := ensureDir(targetPath); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := ensureDir(filepath.Dir(targetPath)); err != nil {
				return err
			}
			if err := writeFileFromReader(targetPath, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := ensureDir(filepath.Dir(targetPath)); err != nil {
				return err
			}
			if err := os.RemoveAll(targetPath); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		default:
			// Ignore other entry types for now
		}
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFileFromReader(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func copyGemToVendorCache(srcPath, destPath string) error {
	if err := ensureDir(filepath.Dir(destPath)); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return err
	}

	return nil
}

func linkGemBinaries(gemDir, binDir string) error {
	exeDir := filepath.Join(gemDir, "bin")
	entries, err := os.ReadDir(exeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Get gem name from directory (e.g., "vendor/gems/rake-13.3.0" -> "rake-13.3.0")
	gemName := filepath.Base(gemDir)

	// Get vendor root (parent of gems directory)
	vendorRoot := filepath.Dir(filepath.Dir(gemDir))

	for _, entry := range entries {
		execName := entry.Name()
		originalExec := filepath.Join(exeDir, execName)
		binstubPath := filepath.Join(binDir, execName)

		// Create binstub wrapper script
		if err := createBinstub(binstubPath, originalExec, gemName, vendorRoot); err != nil {
			return fmt.Errorf("failed to create binstub for %s: %w", execName, err)
		}
	}

	return nil
}

// createBinstub creates a Ruby wrapper script (binstub) for a gem executable
func createBinstub(binstubPath, originalExec, gemName, vendorRoot string) error {
	execName := filepath.Base(originalExec)

	// Create binstub content - manually construct to ensure proper Ruby syntax
	var binstub strings.Builder
	binstub.WriteString("#!/usr/bin/env ruby\n")
	binstub.WriteString("# frozen_string_literal: true\n")
	binstub.WriteString("\n")
	binstub.WriteString("#\n")
	binstub.WriteString("# This file was generated by ore-light.\n")
	binstub.WriteString("#\n")
	binstub.WriteString(fmt.Sprintf("# The application '%s' is installed as part of a gem, and\n", execName))
	binstub.WriteString("# this file is here to facilitate running it.\n")
	binstub.WriteString("#\n")
	binstub.WriteString("\n")
	binstub.WriteString("# Set up gem environment for ore-light vendor directory\n")
	binstub.WriteString(fmt.Sprintf("vendor_root = \"%s\"\n", vendorRoot))
	binstub.WriteString("ENV[\"GEM_HOME\"] = vendor_root\n")
	binstub.WriteString("ENV[\"GEM_PATH\"] = vendor_root\n")
	binstub.WriteString("\n")
	binstub.WriteString("# Add all gem lib directories to load path\n")
	binstub.WriteString("gems_dir = File.join(vendor_root, \"gems\")\n")
	binstub.WriteString("if File.directory?(gems_dir)\n")
	binstub.WriteString("  Dir.glob(File.join(gems_dir, \"*\", \"lib\")).each do |lib_dir|\n")
	binstub.WriteString("    $LOAD_PATH.unshift(lib_dir) unless $LOAD_PATH.include?(lib_dir)\n")
	binstub.WriteString("  end\n")
	binstub.WriteString("end\n")
	binstub.WriteString("\n")
	binstub.WriteString("# Load the actual executable\n")
	binstub.WriteString(fmt.Sprintf("load \"%s\"\n", originalExec))

	// Write binstub file
	if err := os.WriteFile(binstubPath, []byte(binstub.String()), 0755); err != nil {
		return err
	}

	return nil
}

func decompressMetadata(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress metadata: %w", err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// gemMetadata represents extracted metadata from YAML
type gemMetadata struct {
	Name        string       `yaml:"name"`
	Version     versionField `yaml:"version"`
	Authors     []string     `yaml:"authors"`
	Author      string       `yaml:"author"`
	Email       interface{}  `yaml:"email"` // Can be string or []string
	Homepage    string       `yaml:"homepage"`
	Summary     string       `yaml:"summary"`
	Description string       `yaml:"description"`
	Licenses    []string     `yaml:"licenses"`
	License     string       `yaml:"license"`
	Platform    string       `yaml:"platform"`
}

// versionField handles both nested and simple version formats
// After stripping Ruby tags, "version: !ruby/object:Gem::Version\n  version: 2.7.3"
// becomes "version:\n  version: 2.7.3" (nested map)
type versionField struct {
	Version string `yaml:"version"` // Nested version string
}

// UnmarshalYAML allows versionField to accept both string and nested object
func (v *versionField) UnmarshalYAML(node *yaml.Node) error {
	// Try unmarshaling as a simple string first
	var simpleVersion string
	if err := node.Decode(&simpleVersion); err == nil && simpleVersion != "" {
		v.Version = simpleVersion
		return nil
	}

	// Fall back to nested structure: { version: "2.7.3" }
	var nested struct {
		Version string `yaml:"version"`
	}
	if err := node.Decode(&nested); err == nil && nested.Version != "" {
		v.Version = nested.Version
		return nil
	}

	// If both fail, leave empty
	return nil
}

// String returns the version string for convenience
func (v versionField) String() string {
	return v.Version
}

var rubyTagPattern = regexp.MustCompile(`!ruby/object:[A-Za-z:]+`)

// stripRubyYAMLTags removes Ruby-specific YAML tags that gopkg.in/yaml.v3 can't parse
// Simple approach: just remove all Ruby tags and let YAML parser handle the structure
func stripRubyYAMLTags(data []byte) []byte {
	// Use regex to remove all Ruby object tags in one pass
	result := rubyTagPattern.ReplaceAll(data, []byte(""))

	// Debug: log cleaned YAML if ORE_DEBUG_YAML is set
	if os.Getenv("ORE_DEBUG_YAML") != "" {
		fmt.Fprintf(os.Stderr, "=== Cleaned YAML ===\n%s\n=== End ===\n", string(result))
	}

	return result
}

func writeGemSpecification(vendorDir string, spec lockfile.GemSpec, metadataYAML []byte) error {
	specDir := filepath.Join(vendorDir, "specifications")
	if err := ensureDir(specDir); err != nil {
		return err
	}

	// Parse YAML metadata to extract real gem info
	// Strip Ruby-specific YAML tags that yaml.v3 can't parse
	cleanedYAML := stripRubyYAMLTags(metadataYAML)

	var gemMeta gemMetadata
	if err := yaml.Unmarshal(cleanedYAML, &gemMeta); err != nil {
		// Debug: log parsing error
		if os.Getenv("ORE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "YAML parse error for %s: %v\n", spec.FullName(), err)
		}
		// If parsing fails, use basic metadata
		gemMeta = gemMetadata{
			Name:    spec.Name,
			Version: versionField{Version: spec.Version},
			Authors: []string{"Gem Authors"},
			Email:   "ore@example.com",
		}
	} else if os.Getenv("ORE_DEBUG") != "" {
		// Debug: show extracted metadata
		fmt.Fprintf(os.Stderr, "Extracted metadata for %s: name=%s version=%s authors=%v email=%v\n",
			spec.FullName(), gemMeta.Name, gemMeta.Version.String(), gemMeta.Authors, gemMeta.Email)
	}

	// Build proper Ruby gemspec code
	rubyCode := generateGemspecCode(spec, &gemMeta)

	specPath := filepath.Join(specDir, fmt.Sprintf("%s.gemspec", spec.FullName()))
	if err := os.WriteFile(specPath, []byte(rubyCode), 0o644); err != nil {
		return fmt.Errorf("failed to write gemspec for %s: %w", spec.FullName(), err)
	}

	return nil
}

// gemspecTemplate is the template for generating RubyGems-compatible gemspec files
const gemspecTemplate = `# -*- encoding: utf-8 -*-
# stub: {{.Name}} {{.Version}} {{.Platform}} lib

Gem::Specification.new do |s|
  s.name = {{printf "%q" .Name}}
  s.version = {{printf "%q" .Version}}
{{- if ne .Platform "ruby"}}
  s.platform = {{printf "%q" .Platform}}
{{- end}}
  s.authors = [{{range $i, $a := .Authors}}{{if $i}}, {{end}}{{printf "%q" $a}}{{end}}]
  s.email = {{printf "%q" .Email}}
  s.homepage = {{printf "%q" .Homepage}}
  s.licenses = [{{range $i, $l := .Licenses}}{{if $i}}, {{end}}{{printf "%q" $l}}{{end}}]
  s.required_rubygems_version = Gem::Requirement.new(">= 0")
  s.require_paths = ["lib"]
  s.rubygems_version = "3.5.0"
  s.summary = {{printf "%q" .Summary}}
  s.description = {{printf "%q" .Description}}
{{- if .Dependencies}}

{{- range .Dependencies}}
  s.add_runtime_dependency({{printf "%q" .Name}}{{if .Constraints}}, [{{range $i, $c := .Constraints}}{{if $i}}, {{end}}{{printf "%q" $c}}{{end}}]{{end}})
{{- end}}
{{- end}}
end
`

var gemspecTmpl = template.Must(template.New("gemspec").Parse(gemspecTemplate))

// gemspecData is the data structure passed to the gemspec template
type gemspecData struct {
	Name         string
	Version      string
	Platform     string
	Authors      []string
	Email        string
	Homepage     string
	Licenses     []string
	Summary      string
	Description  string
	Dependencies []lockfile.Dependency
}

// extractEmail handles both string and array email types from YAML
func extractEmail(emailField interface{}) string {
	switch v := emailField.(type) {
	case string:
		return v
	case []interface{}:
		// Array of emails - return first non-empty
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				return s
			}
		}
	case []string:
		// Already string array
		for _, s := range v {
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func generateGemspecCode(spec lockfile.GemSpec, meta *gemMetadata) string {
	// Handle authors - array or single
	authors := meta.Authors
	if len(authors) == 0 && meta.Author != "" {
		authors = []string{meta.Author}
	}
	if len(authors) == 0 {
		authors = []string{"Gem Authors"}
	}

	// Handle licenses - array or single
	licenses := meta.Licenses
	if len(licenses) == 0 && meta.License != "" {
		licenses = []string{meta.License}
	}
	if len(licenses) == 0 {
		licenses = []string{"MIT"}
	}

	// Platform defaults
	platform := meta.Platform
	if platform == "" {
		platform = spec.Platform
	}
	if platform == "" {
		platform = "ruby"
	}

	// Email default - handle both string and array types
	email := extractEmail(meta.Email)
	if email == "" {
		email = "ore@example.com"
	}

	// Homepage default
	homepage := meta.Homepage
	if homepage == "" {
		homepage = fmt.Sprintf("https://rubygems.org/gems/%s", spec.Name)
	}

	// Summary default
	summary := meta.Summary
	if summary == "" {
		summary = fmt.Sprintf("Gem %s", spec.Name)
	}

	// Description default
	description := meta.Description
	if description == "" {
		description = fmt.Sprintf("Gem %s version %s installed by Ore", spec.Name, spec.Version)
	}

	data := gemspecData{
		Name:         spec.Name,
		Version:      spec.Version,
		Platform:     platform,
		Authors:      authors,
		Email:        email,
		Homepage:     homepage,
		Licenses:     licenses,
		Summary:      summary,
		Description:  description,
		Dependencies: spec.Dependencies,
	}

	var buf bytes.Buffer
	if err := gemspecTmpl.Execute(&buf, data); err != nil {
		// Fallback to basic gemspec if template fails
		return fmt.Sprintf(`# -*- encoding: utf-8 -*-
# stub: %s %s ruby lib

Gem::Specification.new do |s|
  s.name = %q
  s.version = %q
end
`, spec.Name, spec.Version, spec.Name, spec.Version)
	}

	return buf.String()
}

func buildExecutionEnv(vendorDir string, specs []lockfile.GemSpec) ([]string, error) {
	if err := ensureDir(vendorDir); err != nil {
		return nil, err
	}

	libPaths := collectLibraryPaths(vendorDir, specs)
	if len(libPaths) == 0 {
		return nil, fmt.Errorf("no gem libraries found under %s; run `ore install` first", vendorDir)
	}

	env := os.Environ()
	env = setEnv(env, "GEM_HOME", vendorDir)
	env = setEnv(env, "GEM_PATH", vendorDir)
	env = prependPath(env, filepath.Join(vendorDir, "bin"))
	env = prependRubyLib(env, libPaths)
	return env, nil
}

func collectLibraryPaths(vendorDir string, specs []lockfile.GemSpec) []string {
	seen := make(map[string]struct{})
	var libs []string

	for _, spec := range specs {
		libDir := filepath.Join(vendorDir, "gems", spec.FullName(), "lib")
		if _, err := os.Stat(libDir); err != nil {
			continue
		}
		if _, ok := seen[libDir]; ok {
			continue
		}
		seen[libDir] = struct{}{}
		libs = append(libs, libDir)
	}

	return libs
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func prependPath(env []string, path string) []string {
	if path == "" {
		return env
	}
	current, _ := getEnvValue(env, "PATH")
	if current == "" {
		return setEnv(env, "PATH", path)
	}
	return setEnv(env, "PATH", fmt.Sprintf("%s%c%s", path, os.PathListSeparator, current))
}

func prependRubyLib(env []string, libs []string) []string {
	if len(libs) == 0 {
		return env
	}
	libValue := strings.Join(libs, string(os.PathListSeparator))
	if current, _ := getEnvValue(env, "RUBYLIB"); current != "" {
		libValue = libValue + string(os.PathListSeparator) + current
	}
	return setEnv(env, "RUBYLIB", libValue)
}

func getEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	if value, ok := os.LookupEnv(key); ok {
		return value, true
	}
	return "", false
}

// installGitGems installs gems from Git sources
func installGitGems(ctx context.Context, vendorDir string, gitSpecs []lockfile.GitGemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gitSpecs)}

	if err := ensureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	for _, spec := range gitSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		// Clone the git repo at the locked revision
		if err := cloneGitGem(spec, destDir); err != nil {
			return report, fmt.Errorf("failed to clone git gem %s: %w", spec.Name, err)
		}

		// Link binaries if any
		if err := linkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gemName)
		if err != nil {
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gemName, err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gemName, extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

	return report, nil
}

// cloneGitGem clones a git gem at the specified revision
func cloneGitGem(spec lockfile.GitGemSpec, destDir string) error {
	// Import the resolver package to use GitSource
	gitSource, err := resolver.NewGitSource(spec.Remote, spec.Branch, spec.Tag, spec.Revision)
	if err != nil {
		return fmt.Errorf("failed to create git source: %w", err)
	}

	// Clone at the locked revision
	if err := gitSource.CloneAtRevision(spec.Revision, destDir); err != nil {
		return fmt.Errorf("failed to clone at revision %s: %w", spec.Revision, err)
	}

	return nil
}

// installPathGems installs gems from local paths
func installPathGems(ctx context.Context, vendorDir string, pathSpecs []lockfile.PathGemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(pathSpecs)}

	if err := ensureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	for _, spec := range pathSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		// Copy the path gem to vendor
		if err := copyPathGem(spec, destDir); err != nil {
			return report, fmt.Errorf("failed to copy path gem %s: %w", spec.Name, err)
		}

		// Link binaries if any
		if err := linkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gemName)
		if err != nil {
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gemName, err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gemName, extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

	return report, nil
}

// copyPathGem copies a path gem to the vendor directory
func copyPathGem(spec lockfile.PathGemSpec, destDir string) error {
	pathSource, err := resolver.NewPathSource(spec.Remote)
	if err != nil {
		return fmt.Errorf("failed to create path source: %w", err)
	}

	if err := pathSource.CopyToVendor(destDir); err != nil {
		return fmt.Errorf("failed to copy to vendor: %w", err)
	}

	return nil
}

// Helper for tests: create a minimal .gem archive.
func createFakeGemArchive(dest string, files map[string][]byte, marshalData []byte) error {
	var metadataBuf bytes.Buffer
	metaGz := gzip.NewWriter(&metadataBuf)
	if marshalData == nil {
		marshalData = []byte("placeholder metadata")
	}
	if _, err := metaGz.Write(marshalData); err != nil {
		return err
	}
	if err := metaGz.Close(); err != nil {
		return err
	}

	var dataBuffer bytes.Buffer
	dataGz := gzip.NewWriter(&dataBuffer)
	dataTw := tar.NewWriter(dataGz)

	for path, content := range files {
		if err := dataTw.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0o755,
			Size: int64(len(content)),
		}); err != nil {
			return err
		}
		if _, err := dataTw.Write(content); err != nil {
			return err
		}
	}

	if err := dataTw.Close(); err != nil {
		return err
	}
	if err := dataGz.Close(); err != nil {
		return err
	}

	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	tw := tar.NewWriter(file)

	if err := tw.WriteHeader(&tar.Header{
		Name: "metadata.gz",
		Mode: 0o644,
		Size: int64(metadataBuf.Len()),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(metadataBuf.Bytes()); err != nil {
		return err
	}

	if err := tw.WriteHeader(&tar.Header{
		Name: "data.tar.gz",
		Mode: 0o644,
		Size: int64(dataBuffer.Len()),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(dataBuffer.Bytes()); err != nil {
		return err
	}

	return tw.Close()
}
