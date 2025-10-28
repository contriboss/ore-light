package resolver

import (
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/ruby"
)

// EngineCompatibility checks if gems are compatible with the Ruby engine
type EngineCompatibility struct {
	engine ruby.Engine
}

// NewEngineCompatibility creates a new engine compatibility checker
func NewEngineCompatibility(engine ruby.Engine) *EngineCompatibility {
	return &EngineCompatibility{
		engine: engine,
	}
}

// hasNativeCExtension checks if a gem has native C extensions (extconf.rb)
// Returns true if the gem has C extensions, false for JRuby jar extensions
func hasNativeCExtension(gem lockfile.GemSpec) bool {
	if len(gem.Extensions) == 0 {
		return false
	}

	// Check for typical C extension build scripts
	for _, ext := range gem.Extensions {
		basename := filepath.Base(ext)
		if basename == "extconf.rb" || basename == "mkrf_conf.rb" || basename == "configure" {
			return true
		}
	}

	return false
}

// IsCompatible checks if a gem is compatible with the current Ruby engine
func (ec *EngineCompatibility) IsCompatible(gem lockfile.GemSpec) bool {
	// Check platform compatibility first (e.g., java platform for JRuby)
	if !ec.isPlatformCompatible(gem.Platform) {
		return false
	}

	// For pure Ruby gems (no extensions), always compatible
	if len(gem.Extensions) == 0 {
		return true
	}

	// For gems with extensions, distinguish C extensions from JRuby jar extensions
	if hasNativeCExtension(gem) {
		// This is a C extension (has extconf.rb) - check if engine supports it
		return ec.engine.SupportsNativeExtensions()
	}

	// Extensions present but no extconf.rb - likely JRuby jar extensions
	// These are compatible with JRuby, but for other engines we fall back to the
	// engine's native extension support as a conservative proxy
	if ec.engine.Name == ruby.EngineJRuby {
		return true // JRuby can handle jar extensions
	}

	// For other engines, check if they support extensions in general
	return ec.engine.SupportsNativeExtensions()
}

// isPlatformCompatible checks if a platform is compatible with the engine
func (ec *EngineCompatibility) isPlatformCompatible(platform string) bool {
	if platform == "" || platform == "ruby" {
		// Pure Ruby, always compatible
		return true
	}

	// Get engine-specific platform suffix
	enginePlatform := ec.engine.PlatformSuffix()

	// For JRuby, check for "java" platform
	if ec.engine.Name == ruby.EngineJRuby {
		return platform == "java" || strings.HasSuffix(platform, "-java")
	}

	// For TruffleRuby and MRI, check if it's NOT java
	if ec.engine.Name == ruby.EngineMRI || ec.engine.Name == ruby.EngineTruffleRuby {
		// These engines don't support Java platform gems
		if platform == "java" || strings.HasSuffix(platform, "-java") {
			return false
		}
		return true
	}

	// For unknown engines, check if platforms match
	return enginePlatform == "" || platform == enginePlatform
}

// FilterGems filters a list of gems to only include those compatible with the engine
func (ec *EngineCompatibility) FilterGems(gems []lockfile.GemSpec) (compatible []lockfile.GemSpec, skipped []lockfile.GemSpec) {
	for _, gem := range gems {
		if ec.IsCompatible(gem) {
			compatible = append(compatible, gem)
		} else {
			skipped = append(skipped, gem)
		}
	}
	return compatible, skipped
}

// GetIncompatibilityReason returns a human-readable reason why a gem is incompatible
func (ec *EngineCompatibility) GetIncompatibilityReason(gem lockfile.GemSpec) string {
	if ec.IsCompatible(gem) {
		return ""
	}

	// Check platform first
	if gem.Platform != "" && !ec.isPlatformCompatible(gem.Platform) {
		if ec.engine.Name == ruby.EngineJRuby && !strings.HasSuffix(gem.Platform, "-java") && gem.Platform != "java" {
			return "requires platform " + gem.Platform + " but using JRuby (java platform)"
		}
		if (ec.engine.Name == ruby.EngineMRI || ec.engine.Name == ruby.EngineTruffleRuby) && (gem.Platform == "java" || strings.HasSuffix(gem.Platform, "-java")) {
			return "requires Java platform but using " + ec.engine.Name
		}
		return "incompatible platform: " + gem.Platform
	}

	// Check for native C extensions
	if hasNativeCExtension(gem) && !ec.engine.SupportsNativeExtensions() {
		return "has native C extensions but " + ec.engine.Name + " doesn't support them"
	}

	return "incompatible with " + ec.engine.Name
}
