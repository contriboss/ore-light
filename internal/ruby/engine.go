package ruby

import (
	"os"
	"os/exec"
	"strings"
)

// Engine represents a Ruby implementation
type Engine struct {
	Name    string // mri, jruby, truffleruby, mruby
	Version string // e.g., "3.4.0", "9.4.0.0" (for JRuby)
}

// EngineType constants for common Ruby engines
const (
	EngineMRI         = "mri"         // Standard CRuby/MRI
	EngineJRuby       = "jruby"       // JRuby (Java)
	EngineTruffleRuby = "truffleruby" // TruffleRuby (GraalVM)
	EngineMRuby       = "mruby"       // mruby (embedded)
)

// DetectEngine detects the current Ruby engine and version
// Priority:
// 1. RUBY_ENGINE environment variable
// 2. Running `ruby -e "puts RUBY_ENGINE"` command
// 3. Default to "mri" if detection fails
func DetectEngine() Engine {
	// Try environment variable first (fast)
	if engine := os.Getenv("RUBY_ENGINE"); engine != "" {
		version := os.Getenv("RUBY_VERSION")
		if version == "" {
			version = detectVersionFromCommand()
		}
		return Engine{
			Name:    normalizeEngineName(engine),
			Version: version,
		}
	}

	// Try running Ruby command
	return detectEngineFromCommand()
}

// detectEngineFromCommand runs ruby commands to detect engine
func detectEngineFromCommand() Engine {
	engine := Engine{
		Name:    EngineMRI, // default
		Version: "",
	}

	// Detect engine name
	cmd := exec.Command("ruby", "-e", "puts RUBY_ENGINE")
	output, err := cmd.Output()
	if err == nil {
		engine.Name = normalizeEngineName(strings.TrimSpace(string(output)))
	}

	// Detect version
	engine.Version = detectVersionFromCommand()

	return engine
}

// detectVersionFromCommand gets Ruby version
func detectVersionFromCommand() string {
	cmd := exec.Command("ruby", "-e", "puts RUBY_VERSION")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// normalizeEngineName normalizes engine name variations
func normalizeEngineName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Handle variations
	switch {
	case name == "ruby" || name == "cruby":
		return EngineMRI
	case strings.HasPrefix(name, "jruby"):
		return EngineJRuby
	case strings.HasPrefix(name, "truffleruby"):
		return EngineTruffleRuby
	case strings.HasPrefix(name, "mruby"):
		return EngineMRuby
	default:
		return name
	}
}

// SupportsNativeExtensions returns true if the engine can build C extensions
func (e Engine) SupportsNativeExtensions() bool {
	switch e.Name {
	case EngineMRI:
		return true // MRI fully supports C extensions
	case EngineJRuby:
		return false // JRuby doesn't support C extensions well
	case EngineTruffleRuby:
		return true // TruffleRuby supports C extensions via LLVM
	case EngineMRuby:
		return false // mruby is embedded, no extensions
	default:
		// Unknown engine, assume it doesn't support extensions
		return false
	}
}

// PlatformSuffix returns the platform suffix for this engine
// e.g., "java" for JRuby, "" for MRI
func (e Engine) PlatformSuffix() string {
	switch e.Name {
	case EngineJRuby:
		return "java"
	case EngineTruffleRuby:
		// TruffleRuby uses regular platform suffixes
		return ""
	default:
		return ""
	}
}

// String returns a human-readable representation
func (e Engine) String() string {
	if e.Version != "" {
		return e.Name + " " + e.Version
	}
	return e.Name
}

// ParseEngineFromString parses engine from string like "jruby" or "mri 3.4.0"
func ParseEngineFromString(s string) Engine {
	parts := strings.Fields(s)
	engine := Engine{
		Name: EngineMRI, // default
	}

	if len(parts) > 0 {
		engine.Name = normalizeEngineName(parts[0])
	}
	if len(parts) > 1 {
		engine.Version = parts[1]
	}

	return engine
}

// DetectEngineFromPlatform detects engine from platform string
// e.g., "java" → jruby, "x86_64-darwin" → mri
func DetectEngineFromPlatform(platform string) string {
	platform = strings.ToLower(platform)

	if platform == "java" {
		return EngineJRuby
	}

	// Default to MRI for all other platforms
	return EngineMRI
}
