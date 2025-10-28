package resolver

import (
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

// IsCompatible checks if a gem is compatible with the current Ruby engine
func (ec *EngineCompatibility) IsCompatible(gem lockfile.GemSpec) bool {
	// Check platform compatibility first
	if !ec.isPlatformCompatible(gem.Platform) {
		return false
	}

	// For pure Ruby gems (no extensions), always compatible
	if len(gem.Extensions) == 0 && gem.Platform == "" {
		return true
	}

	// For gems with native extensions, check if engine supports them
	if len(gem.Extensions) > 0 {
		return ec.engine.SupportsNativeExtensions()
	}

	// For platform-specific gems, check platform match
	if gem.Platform != "" {
		return ec.isPlatformCompatible(gem.Platform)
	}

	return true
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
		if ec.engine.Name == ruby.EngineJRuby && gem.Platform != "java" {
			return "requires platform " + gem.Platform + " but using JRuby (java platform)"
		}
		if ec.engine.Name == ruby.EngineMRI && gem.Platform == "java" {
			return "requires Java platform but using MRI"
		}
		return "incompatible platform: " + gem.Platform
	}

	// Check native extensions
	if len(gem.Extensions) > 0 && !ec.engine.SupportsNativeExtensions() {
		return "has native extensions but " + ec.engine.Name + " doesn't support them"
	}

	return "incompatible with " + ec.engine.Name
}
