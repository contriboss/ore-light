package geminstall

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/contriboss/gemfile-go/lockfile"
	"gopkg.in/yaml.v3"
)

const (
	// DEFAULT_RUBYGEMS_VERSION is the RubyGems version to write in gemspec files
	// Update this to match the current stable RubyGems release (should match cmd/ore/main.go)
	DEFAULT_RUBYGEMS_VERSION = "3.6.4"
)

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
	Extensions  []string     `yaml:"extensions"` // Native C extensions
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

// ParseExtensionsFromMetadata extracts the extensions list from gem metadata YAML
func ParseExtensionsFromMetadata(metadataYAML []byte) []string {
	cleanedYAML := stripRubyYAMLTags(metadataYAML)

	var gemMeta gemMetadata
	if err := yaml.Unmarshal(cleanedYAML, &gemMeta); err != nil {
		return nil
	}

	return gemMeta.Extensions
}

// WriteGemSpecification writes a gemspec file for the given gem
func WriteGemSpecification(vendorDir string, spec lockfile.GemSpec, metadataYAML []byte) error {
	specDir := filepath.Join(vendorDir, "specifications")
	if err := EnsureDir(specDir); err != nil {
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
  s.rubygems_version = "{{.RubygemsVersion}}"
  s.summary = {{printf "%q" .Summary}}
  s.description = {{printf "%q" .Description}}
{{- if .Extensions}}
  s.extensions = [{{range $i, $e := .Extensions}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}}]
{{- end}}
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
	Name            string
	Version         string
	Platform        string
	Authors         []string
	Email           string
	Homepage        string
	Licenses        []string
	Summary         string
	Description     string
	Dependencies    []lockfile.Dependency
	RubygemsVersion string
	Extensions      []string // Native C extensions
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

	// Extensions - use from metadata if available, otherwise from spec
	extensions := meta.Extensions
	if len(extensions) == 0 && len(spec.Extensions) > 0 {
		extensions = spec.Extensions
	}

	data := gemspecData{
		Name:            spec.Name,
		Version:         spec.Version,
		Platform:        platform,
		Authors:         authors,
		Email:           email,
		Homepage:        homepage,
		Licenses:        licenses,
		Summary:         summary,
		Description:     description,
		Dependencies:    spec.Dependencies,
		RubygemsVersion: DEFAULT_RUBYGEMS_VERSION,
		Extensions:      extensions,
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
