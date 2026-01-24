package geminstall

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/ruby"
	"gopkg.in/yaml.v3"
)

// gemMetadata represents extracted metadata from YAML
type gemMetadata struct {
	Name                     string            `yaml:"name"`
	Version                  versionField      `yaml:"version"`
	Authors                  []string          `yaml:"authors"`
	Author                   string            `yaml:"author"`
	Email                    interface{}       `yaml:"email"` // Can be string or []string
	Homepage                 string            `yaml:"homepage"`
	Summary                  string            `yaml:"summary"`
	Description              string            `yaml:"description"`
	Licenses                 []string          `yaml:"licenses"`
	License                  string            `yaml:"license"`
	Platform                 string            `yaml:"platform"`
	Bindir                   string            `yaml:"bindir"`
	CertChain                []string          `yaml:"cert_chain"`
	Date                     string            `yaml:"date"` // 2025-08-20 00:00:00.000000000 Z
	Executables              []string          `yaml:"executables"`
	Extensions               []string          `yaml:"extensions"` // Native C extensions
	ExtraRdocFiles           []string          `yaml:"extra_rdoc_files"`
	Files                    []string          `yaml:"files"`
	Metadata                 map[string]string `yaml:"metadata"`
	PostInstallMessage       string            `yaml:"post_install_message"`
	RdocOptions              []string          `yaml:"rdoc_options"`
	RequirePaths             []string          `yaml:"require_paths"`
	RequiredRubyVersion      requirementField  `yaml:"required_ruby_version"`
	RequiredRubygemsVersion  requirementField  `yaml:"required_rubygems_version"`
	Requirements             []string          `yaml:"requirements"`
	RubygemsVersion          string            `yaml:"rubygems_version"`
	SigningKey               string            `yaml:"signing_key"`
	SpecificationVersion     int               `yaml:"specification_version"`
	TestFiles                []string          `yaml:"test_files"`
}

// requirementField handles Gem::Requirement which is a struct with a list of requirements
type requirementField struct {
	Requirements [][]interface{} `yaml:"requirements"`
}

// UnmarshalYAML for requirementField
func (r *requirementField) UnmarshalYAML(node *yaml.Node) error {
	// A requirement might be single string (simple scalar) or object
	var simple string
	if err := node.Decode(&simple); err == nil && simple != "" {
		// Just a version string? Treat as >= string
		r.Requirements = [][]interface{}{{">=", simple}}
		return nil
	}

	var obj struct {
		Requirements [][]interface{} `yaml:"requirements"`
	}
	if err := node.Decode(&obj); err == nil {
		r.Requirements = obj.Requirements
		return nil
	}
	return nil
}

// String converting requirements back to valid Ruby code params for Gem::Requirement.new
// e.g. '">= 2.3.0"' or '">= 2.3.0", "< 3.0"'
func (r requirementField) ToRuby() string {
	if len(r.Requirements) == 0 {
		return ">= 0"
	}
	var parts []string
	for _, req := range r.Requirements {
		if len(req) == 2 {
			op, opOk := req[0].(string)
			// ver might be a map {version: "2.3.0"} or string
			var verStr string
			if s, ok := req[1].(string); ok {
				verStr = s
			} else if m, ok := req[1].(map[string]interface{}); ok {
				if v, ok := m["version"].(string); ok {
					verStr = v
				}
			}

			if opOk && verStr != "" {
				parts = append(parts, fmt.Sprintf("%q", op+" "+verStr))
			}
		}
	}
	if len(parts) == 0 {
		return ">= 0"
	}
	// Gem::Requirement.new([req1, req2]) works
	if len(parts) == 1 {
		return parts[0]
	}
	return "[" + getJoin(parts, ", ") + "]"
}

func getJoin(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	res := items[0]
	for _, item := range items[1:] {
		res += sep + item
	}
	return res
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

	// Debug: log cleaned YAML if BUNDLE_VERBOSE is set
	if os.Getenv("BUNDLE_VERBOSE") != "" {
		fmt.Fprintf(os.Stderr, "=== Cleaned YAML ===\n%s\n=== End ===\n", string(result))
	}

	return result
}

// ParseExtensionsFromMetadata extracts the extensions list from gem metadata YAML
// Returns the extensions list and any parsing error
func ParseExtensionsFromMetadata(metadataYAML []byte) ([]string, error) {
	cleanedYAML := stripRubyYAMLTags(metadataYAML)

	var gemMeta gemMetadata
	if err := yaml.Unmarshal(cleanedYAML, &gemMeta); err != nil {
		return nil, fmt.Errorf("failed to parse gem metadata: %w", err)
	}

	return gemMeta.Extensions, nil
}

// GemspecIsValid checks if a gemspec file contains the installed_by_version field
// which is required for Bundler to recognize the gem as properly installed.
func GemspecIsValid(specPath string) bool {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return false
	}
	return bytes.Contains(content, []byte("installed_by_version"))
}

// WriteGemSpecification writes a gemspec file for the given gem using a generic map approach
func WriteGemSpecification(vendorDir string, spec lockfile.GemSpec, metadataYAML []byte) error {
	specDir := filepath.Join(vendorDir, "specifications")
	if err := EnsureDir(specDir); err != nil {
		return err
	}

	cleanedYAML := stripRubyYAMLTags(metadataYAML)

	// Unmarshal into generic map
	var rawData map[string]interface{}
	if err := yaml.Unmarshal(cleanedYAML, &rawData); err != nil {
		// Log error if verbose
		if os.Getenv("BUNDLE_VERBOSE") != "" {
			fmt.Fprintf(os.Stderr, "YAML parse error for %s: %v. Falling back to basic gemspec.\n", spec.FullName(), err)
		}
		return writeBasicGemspec(vendorDir, spec)
	}

	// Apply filtering and normalization
	filterAndNormalize(rawData)

	// Ensure crucial fields are present
	if _, ok := rawData["name"]; !ok {
		rawData["name"] = spec.Name
	}
	if _, ok := rawData["version"]; !ok {
		rawData["version"] = spec.Version
	}

	// Generate Ruby code
	rubyCode, err := generateGenericGemspec(rawData)
	if err != nil {
		return fmt.Errorf("failed to generate gemspec ruby for %s: %w", spec.FullName(), err)
	}

	specPath := filepath.Join(specDir, fmt.Sprintf("%s.gemspec", spec.FullName()))
	if err := os.WriteFile(specPath, []byte(rubyCode), 0o644); err != nil {
		return fmt.Errorf("failed to write gemspec for %s: %w", spec.FullName(), err)
	}

	return nil
}

func writeBasicGemspec(vendorDir string, spec lockfile.GemSpec) error {
	specDir := filepath.Join(vendorDir, "specifications")
	specPath := filepath.Join(specDir, fmt.Sprintf("%s.gemspec", spec.FullName()))

	rubyCode := fmt.Sprintf(`# -*- encoding: utf-8 -*-
# stub: %s %s ruby lib

Gem::Specification.new do |s|
  s.name = %q
  s.version = %q
  s.installed_by_version = %q
end
`, spec.Name, spec.Version, spec.Name, spec.Version, ruby.DefaultRubyGemsVersion)

	return os.WriteFile(specPath, []byte(rubyCode), 0o644)
}

func filterAndNormalize(data map[string]interface{}) {
	// Fields to remove entirely
	removals := []string{"date", "specification_version", "test_files", "rubyforge_project"}
	for _, key := range removals {
		delete(data, key)
	}
}

func generateGenericGemspec(data map[string]interface{}) (string, error) {
	var buf bytes.Buffer

	// Extract header info
	name, _ := data["name"].(string)
	version := extractVersionString(data["version"])
	platform, _ := data["platform"].(string)
	if platform == "" || platform == "ruby" {
		platform = "ruby"
	}

	// Calculate require_paths for stub header
	var reqPaths []string
	if rp, ok := data["require_paths"].([]interface{}); ok {
		for _, p := range rp {
			if s, ok := p.(string); ok {
				reqPaths = append(reqPaths, s)
			}
		}
	} else {
		reqPaths = []string{"lib"}
	}

	// Write Stub Header
	fmt.Fprintf(&buf, "# -*- encoding: utf-8 -*-\n")
	fmt.Fprintf(&buf, "# stub: %s %s %s %s\n\n", name, version, platform, strings.Join(reqPaths, " "))

	fmt.Fprintf(&buf, "Gem::Specification.new do |s|\n")

	// Defined preferred order for Bundler parity
	preferredOrder := []string{
		"name",
		"version",
		"installed_by_version",
		"authors",
		"email",
		"description",
		"summary",
		"homepage",
		"licenses",
		"bindir",
		"executables",
		"require_paths",
	}
	preferredMap := make(map[string]int)
	for i, k := range preferredOrder {
		preferredMap[k] = i
	}

	// Sort keys for deterministic output with preference
	var keys []string
	for k := range data {
		if k == "dependencies" {
			continue // Handled at end
		}
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		pi, oki := preferredMap[ki]
		pj, okj := preferredMap[kj]

		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return ki < kj
	})

	for _, k := range keys {
		val := data[k]

		var rubyVal string
		var err error

		switch k {
		case "version":
			v := extractVersionString(val)
			rubyVal = fmt.Sprintf("%q.freeze", v)
		case "platform":
			if val == "ruby" || val == nil {
				continue
			}
			rubyVal = fmt.Sprintf("%q.freeze", val)
		case "required_ruby_version", "required_rubygems_version":
			rubyVal = formatRequirement(val)
			if rubyVal != "" {
				fmt.Fprintf(&buf, "  s.%s = Gem::Requirement.new(%s)\n", k, rubyVal)
				continue
			}
			continue
		case "metadata":
			rubyVal = formatMap(val)
		case "installed_by_version":
			if vStr, ok := val.(string); ok {
				rubyVal = fmt.Sprintf("%q.freeze", vStr)
			} else {
				rubyVal, _ = formatValue(val)
			}
		default:
			rubyVal, err = formatValue(val)
			if err != nil {
				continue
			}
			if rubyVal == "" {
				continue
			}
		}

		fmt.Fprintf(&buf, "  s.%s = %s\n", k, rubyVal)
	}

	// Always add installed_by_version if not present
	if _, ok := data["installed_by_version"]; !ok {
		fmt.Fprintf(&buf, "  s.installed_by_version = %q.freeze\n", ruby.DefaultRubyGemsVersion)
	}

	// Dependencies last
	if deps, ok := data["dependencies"]; ok {
		writeDependencies(&buf, deps)
	}

	fmt.Fprintf(&buf, "end\n")
	return buf.String(), nil
}

func formatValue(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q.freeze", val), nil
	case []interface{}:
		var parts []string
		for _, item := range val {
			s, err := formatValue(item)
			if err == nil && s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) == 0 {
			return "[].freeze", nil
		}
		return "[" + strings.Join(parts, ", ") + "].freeze", nil
	case bool:
		if val {
			return "true", nil
		}
		return "false", nil
	case int, int64, float64:
		return fmt.Sprintf("%v", val), nil
	case nil:
		return "nil", nil
	default:
		if m, ok := val.(map[string]interface{}); ok {
			return formatMap(m), nil
		}
		return "", fmt.Errorf("unsupported type %T", v)
	}
}

func formatMap(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "{}"
	}
	var parts []string
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		valStr, err := formatValue(m[k])
		if err == nil {
			parts = append(parts, fmt.Sprintf("%q.freeze => %s", k, valStr))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func extractVersionString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if m, ok := v.(map[string]interface{}); ok {
		if ver, ok := m["version"]; ok {
			if s, ok := ver.(string); ok {
				return s
			}
		}
	}
	return "0.0.0"
}

func formatRequirement(v interface{}) string {
	var reqs []interface{}

	if m, ok := v.(map[string]interface{}); ok {
		if r, ok := m["requirements"]; ok {
			if list, ok := r.([]interface{}); ok {
				reqs = list
			}
		}
	} else if list, ok := v.([]interface{}); ok {
		reqs = list
	}

	if len(reqs) == 0 {
		return ""
	}

	var rubyReqs []string

	for _, reqItem := range reqs {
		if pair, ok := reqItem.([]interface{}); ok && len(pair) == 2 {
			op, _ := pair[0].(string)
			verStr := extractVersionString(pair[1])

			if op != "" && verStr != "" {
				rubyReqs = append(rubyReqs, fmt.Sprintf("%q.freeze", op+" "+verStr))
			}
		}
	}

	if len(rubyReqs) == 0 {
		return ""
	}
	if len(rubyReqs) == 1 {
		return rubyReqs[0]
	}
	return "[" + strings.Join(rubyReqs, ", ") + "]"
}

func writeDependencies(buf *bytes.Buffer, v interface{}) {
	deps, ok := v.([]interface{})
	if !ok {
		return
	}

	if len(deps) > 0 {
		fmt.Fprintf(buf, "\n")
	}

	sort.Slice(deps, func(i, j int) bool {
		d1, ok1 := deps[i].(map[string]interface{})
		d2, ok2 := deps[j].(map[string]interface{})
		if !ok1 || !ok2 {
			return false
		}
		n1, _ := d1["name"].(string)
		n2, _ := d2["name"].(string)
		return n1 < n2
	})

	for _, item := range deps {
		dep, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := dep["name"].(string)
		typeStr, _ := dep["type"].(string)

		method := "add_dependency"
		if typeStr == ":development" {
			method = "add_development_dependency"
		}

		var reqStr string
		if reqObj, ok := dep["requirement"]; ok {
			reqStr = formatRequirement(reqObj)
		} else if reqObj, ok := dep["version_requirements"]; ok {
			reqStr = formatRequirement(reqObj)
		}

		if reqStr == "" {
			reqStr = "\">= 0.freeze\""
		}

		fmt.Fprintf(buf, "  s.%s(%q.freeze, %s)\n", method, name, reqStr)
	}
}
