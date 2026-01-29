package geminstall

import (
	"bufio"
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
	Name                    string            `yaml:"name"`
	Version                 versionField      `yaml:"version"`
	Authors                 []string          `yaml:"authors"`
	Author                  string            `yaml:"author"`
	Email                   interface{}       `yaml:"email"` // Can be string or []string
	Homepage                string            `yaml:"homepage"`
	Summary                 string            `yaml:"summary"`
	Description             string            `yaml:"description"`
	Licenses                []string          `yaml:"licenses"`
	License                 string            `yaml:"license"`
	Platform                string            `yaml:"platform"`
	Bindir                  string            `yaml:"bindir"`
	CertChain               []string          `yaml:"cert_chain"`
	Date                    string            `yaml:"date"` // 2025-08-20 00:00:00.000000000 Z
	Executables             []string          `yaml:"executables"`
	Extensions              []string          `yaml:"extensions"` // Native C extensions
	ExtraRdocFiles          []string          `yaml:"extra_rdoc_files"`
	Files                   []string          `yaml:"files"`
	Metadata                map[string]string `yaml:"metadata"`
	PostInstallMessage      string            `yaml:"post_install_message"`
	RdocOptions             []string          `yaml:"rdoc_options"`
	RequirePaths            []string          `yaml:"require_paths"`
	RequiredRubyVersion     requirementField  `yaml:"required_ruby_version"`
	RequiredRubygemsVersion requirementField  `yaml:"required_rubygems_version"`
	Requirements            []string          `yaml:"requirements"`
	RubygemsVersion         string            `yaml:"rubygems_version"`
	SigningKey              string            `yaml:"signing_key"`
	SpecificationVersion    int               `yaml:"specification_version"`
	TestFiles               []string          `yaml:"test_files"`
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
		// Was: r.Requirements = [][]interface{}{{">=", simple}}
		// This creates ONE requirement: [">=", "version"]
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
// and its stub platform matches the lockfile spec (Bundler materializes by platform).
func GemspecIsValid(specPath string, spec lockfile.GemSpec) bool {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return false
	}
	if !bytes.Contains(content, []byte("installed_by_version")) {
		return false
	}
	return stubPlatformMatches(content, spec)
}

func stubPlatformMatches(content []byte, spec lockfile.GemSpec) bool {
	expected := spec.Platform
	if expected == "" {
		expected = "ruby"
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "# stub: ") {
			continue
		}
		rest := strings.TrimPrefix(line, "# stub: ")
		parts := strings.SplitN(rest, " ", 4)
		if len(parts) < 3 {
			return false
		}
		return parts[2] == expected
	}

	return false
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
	normalizePlatformForLockfile(spec, rawData)

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

	platform := spec.Platform
	if platform == "" {
		platform = "ruby"
	}

	platformLine := ""
	if platform != "" && platform != "ruby" {
		platformLine = fmt.Sprintf("  s.platform = %q\n", platform)
	}

	rubyCode := fmt.Sprintf(`# -*- encoding: utf-8 -*-
# stub: %s %s %s lib

Gem::Specification.new do |s|
  s.name = %q
  s.version = %q
%s
  s.installed_by_version = %q
end
`, spec.Name, spec.Version, platform, spec.Name, spec.Version, platformLine, ruby.DefaultRubyGemsVersion)

	return os.WriteFile(specPath, []byte(rubyCode), 0o644)
}

func filterAndNormalize(data map[string]interface{}) {
	// Fields to remove entirely
	removals := []string{
		"default_executable",
		"has_rdoc",
		"rubyforge_project",
		"specification_version",
		"test_files",
	}
	for _, key := range removals {
		delete(data, key)
	}

	// Remove empty list fields to cleaner output (Bundler parity)
	// Bundler often omits empty arrays like executables, extensions, requirements
	// if they are empty.
	emptyChecks := []string{"executables", "extensions", "requirements", "extra_rdoc_files", "rdoc_options"}
	for _, key := range emptyChecks {
		if list, ok := data[key].([]interface{}); ok && len(list) == 0 {
			delete(data, key)
		} else if list, ok := data[key].([]string); ok && len(list) == 0 {
			// In case it was parsed as []string
			delete(data, key)
		}
	}

	// Use date from metadata if available, otherwise static.
	// Bundler preserves the release date of the gem from metadata.
	// Format is typically YYYY-MM-DD.
	if dateVal, ok := data["date"].(string); ok && dateVal != "" {
		// Clean up date format if needed.
		// YAML often gives "2025-08-20 00:00:00.000000000 Z".
		// We want "2025-08-20".
		if len(dateVal) >= 10 {
			data["date"] = dateVal[:10]
		}
	} else {
		// Fallback if missing
		data["date"] = "1980-01-02"
	}
}

func normalizePlatformForLockfile(spec lockfile.GemSpec, data map[string]interface{}) {
	if spec.Platform != "" && spec.Platform != "ruby" {
		data["platform"] = spec.Platform
		return
	}

	if platform, ok := data["platform"].(string); ok && platform != "" && platform != "ruby" {
		data["platform"] = "ruby"
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
	// Note: Bundler puts executables/extensions even if empty?
	// The diff shows Ore + s.executables = [] + s.extensions = [] + s.requirements = []
	// Bundler (Left) does NOT have these lines.
	// So we SHOULD remove them if empty.
	preferredOrder := []string{
		"name",
		"version",
		"required_rubygems_version",
		"metadata",
		"require_paths",
		"authors",
		"bindir",
		"cert_chain",
		"date",
		"description",
		"email",
		"executables",
		"extensions",
		"extra_rdoc_files",
		"files",
		"homepage",
		"licenses",
		"rdoc_options",
		"required_ruby_version",
		"requirements",
		"rubygems_version",
		"signing_key",
		"summary",
		"test_files",
		"specification_version",
		"installed_by_version",
		"post_install_message",
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
		case "required_ruby_version":
			rubyVal = formatRequirement(val)
			if rubyVal != "" {
				fmt.Fprintf(&buf, "  s.%s = Gem::Requirement.new(%s)\n", k, rubyVal)
				continue
			}
			continue
		case "required_rubygems_version":
			rubyVal = formatRequirement(val)

			if rubyVal != "" {
				fmt.Fprintf(&buf, "  s.%s = Gem::Requirement.new(%s) if s.respond_to? :required_rubygems_version=\n", k, rubyVal)
				continue
			}
			continue
		case "metadata":
			rubyVal = formatMap(val)
			if rubyVal != "" {
				fmt.Fprintf(&buf, "  s.%s = %s if s.respond_to? :metadata=\n", k, rubyVal)
				continue
			}
			continue // Don't write empty
		case "specification_version":
			rubyVal = "4"
		case "date":
			// Bundler output: s.date = "2025-08-20" (no freeze)
			rubyVal = fmt.Sprintf("%q", val)
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

	// Always add specification_version if not present/written
	// (Note: we deleted it in filterAndNormalize, so it won't be in keys loop unless we accidentally put it back.
	// But let's verify if `keys` loop handles it. If keys doesn't contain "specification_version", we write it here.)
	if _, ok := data["installed_by_version"]; !ok {
		fmt.Fprintf(&buf, "\n  s.installed_by_version = %q.freeze\n", ruby.DefaultRubyGemsVersion)
	}

	if _, ok := data["specification_version"]; !ok {
		// Bundler puts it AFTER installed_by_version
		fmt.Fprintf(&buf, "\n  s.specification_version = 4\n")
	}

	if deps, ok := data["dependencies"]; ok {
		writeDependencies(&buf, deps)
	}

	fmt.Fprintf(&buf, "end\n")
	return buf.String(), nil
}

func quoteRubyString(s string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' {
			b.WriteString(`\"`)
		} else if r == '\\' {
			b.WriteString(`\\`)
		} else if r == '#' {
			b.WriteString("#") // No special escaping unless needed, usually safe as is
		} else if r == '\n' {
			b.WriteString(`\n`)
		} else if r == '\r' {
			b.WriteString(`\r`)
		} else if r == '\t' {
			b.WriteString(`\t`)
		} else if r < 32 || r > 126 {
			fmt.Fprintf(&b, "\\u{%X}", r)
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func formatValue(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return quoteRubyString(val) + ".freeze", nil
	case []interface{}:
		var parts []string
		for _, item := range val {
			s, err := formatValue(item)
			if err == nil && s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) == 0 {
			return "[]", nil
		}
		// Bundler parity: arrays are NOT frozen, but elements ARE.
		// e.g. ["lib".freeze]
		return "[" + strings.Join(parts, ", ") + "]", nil
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
		// Keys and values in s.metadata are NOT frozen in Bundler output usually.
		// formatValue freezes everything by default.
		// Let's manually format strings here without freeze if possible?
		// But formatMap is generic.
		// However, s.metadata is the main map we output.
		// Let's assume map values should NOT be frozen?
		// Or maybe just metadata.
		// Bundler's gemspec template usually freezes strings.
		// But `s.metadata` might be special.
		// Let's modify formatMap to invoke a non-freezing formatter or just strip .freeze.
		// Or update formatValue to take an option?
		// Simpler: Just rely on the user's report that Bundler output didn't have frozen values in metadata.
		// The diff showed: `s.metadata = { "key" => "value" }` vs `... { "key" => "value".freeze }`.

		valStr, err := formatValue(m[k])
		if err == nil {
			// Keys are NOT frozen in Bundler output
			keyStr, _ := formatValue(k)
			keyStr = strings.TrimSuffix(keyStr, ".freeze") // hackily remove freeze for key

			// Values: If it's a string, formatValue added .freeze.
			// Let's remove it for map values to match observed Bundler behavior.
			// This might be risky if there are maps where values MUST be frozen,
			// but gemspecs rarely use maps other than metadata.
			if strings.HasSuffix(valStr, ".freeze") {
				valStr = strings.TrimSuffix(valStr, ".freeze")
			}
			parts = append(parts, fmt.Sprintf("%s => %s", keyStr, valStr))
		}
	}
	// Bundler puts spaces inside the braces: { "key" => "val", ... }
	if len(parts) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(parts, ", ") + " }"
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
				// Requirements: `["~> 1.1".freeze, ...]`
				// Using plain string quoting with freeze
				rubyReqs = append(rubyReqs, fmt.Sprintf("%q.freeze", op+" "+verStr))
			}
		}
	}

	if len(rubyReqs) == 0 {
		return ""
	}

	// Default to single string if one requirement, matching Bundler's behavior for Gem::Requirement.new
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

	// We need stable sort to preserve original order within types
	sort.SliceStable(deps, func(i, j int) bool {
		d1, _ := deps[i].(map[string]interface{})
		d2, _ := deps[j].(map[string]interface{})
		t1 := fmt.Sprintf("%v", d1["type"])
		t2 := fmt.Sprintf("%v", d2["type"])

		isDev1 := strings.Contains(t1, "development")
		isDev2 := strings.Contains(t2, "development")

		if isDev1 != isDev2 {
			// Runtime (false) < Development (true)
			return !isDev1
		}
		// If same type, maintain original order
		return false
	})

	for _, item := range deps {
		dep, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := dep["name"].(string)
		typeStr, _ := dep["type"].(string)

		method := "add_runtime_dependency"
		if strings.Contains(typeStr, "development") {
			method = "add_development_dependency"
		} else {
			method = "add_runtime_dependency" // Bundler default output explicit? NO, it uses add_runtime_dependency
		}
		// Actually Bundler uses add_dependency for runtime?
		// Diff showed:
		// - s.add_runtime_dependency(%q<version_gem>.freeze...
		// + s.add_dependency("version_gem".freeze...
		// So Bundler used `add_runtime_dependency`. Ore used `add_dependency`.
		// Parity requirement: use `add_runtime_dependency`.

		var reqStr string
		if reqObj, ok := dep["requirement"]; ok {
			reqStr = formatRequirement(reqObj)
		} else if reqObj, ok := dep["version_requirements"]; ok {
			reqStr = formatRequirement(reqObj)
		}

		if reqStr == "" {
			reqStr = "Gem::Requirement.new(\">= 0\".freeze)"
		} else {
			// Bundler parity: add_dependency ALWAYS uses array notation, even for single items.
			// formatRequirement returns a single string if there's only one item (e.g. ">= 1.0".freeze).
			// We must wrap it in brackets if it's not already wrapped.
			if !strings.HasPrefix(reqStr, "[") {
				reqStr = "[" + reqStr + "]"
			}
		}

		// Bundler uses %q<name>.freeze
		fmt.Fprintf(buf, "  s.%s(%%q<%s>.freeze, %s)\n", method, name, reqStr)
	}
}
