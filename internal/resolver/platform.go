package resolver

import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

// darwinVersionedPlatformRegex matches Darwin platforms with version suffixes (e.g., arm64-darwin-23).
var darwinVersionedPlatformRegex = regexp.MustCompile(`^(.*-darwin)-?\d+$`)

// PlatformTarget represents a platform with both its original form (for lockfile output)
// and normalized form (for matching against gem platform availability).
type PlatformTarget struct {
	Original   string // e.g., "x86_64-linux-musl"
	Normalized string // e.g., "x86_64-linux"
}

// BuildPlatformTargets builds a deduplicated, sorted list of platform targets
// from the given platform strings.
func BuildPlatformTargets(platforms []string) []PlatformTarget {
	set := make(map[string]PlatformTarget)
	for _, platform := range platforms {
		platform = strings.TrimSpace(platform)
		if platform == "" {
			continue
		}
		normalized := normalizePlatformForIndex(platform)
		if normalized == "" {
			continue
		}
		set[platform] = PlatformTarget{Original: platform, Normalized: normalized}
	}

	targets := make([]PlatformTarget, 0, len(set))
	for _, target := range set {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Original < targets[j].Original
	})
	return targets
}

// PlatformScore returns a score for ranking platforms when no specific target is requested.
// Higher scores indicate more preferred platforms.
// - Plain linux (no libc suffix) is preferred (score 3)
// - linux-gnu is second choice (score 2)
// - linux-musl is third choice (score 1)
// - Non-empty non-linux platforms score 1
// - Empty platforms (ruby) score 0
func PlatformScore(platform string) int {
	p := strings.ToLower(strings.TrimSpace(platform))
	if strings.Contains(p, "linux") {
		switch {
		case strings.Contains(p, "linux-musl"):
			return 1
		case strings.Contains(p, "linux-gnu"):
			return 2
		default:
			return 3 // prefer plain linux if available
		}
	}
	if p != "" {
		return 1
	}
	return 0
}

// PlatformScoreWithTarget returns a score for ranking platforms when a specific
// target platform is requested. Higher scores indicate better matches.
//
// When target is linux-musl:
//   - linux-musl scores 3 (exact match)
//   - linux-gnu scores 1 (avoid if possible)
//   - other linux scores 2 (fallback)
//
// When target is linux-gnu:
//   - linux-gnu scores 3 (exact match)
//   - linux-musl scores 1 (avoid if possible)
//   - other linux scores 2 (fallback)
//
// For non-linux targets, falls back to PlatformScore.
func PlatformScoreWithTarget(target, platform string) int {
	target = strings.ToLower(strings.TrimSpace(target))
	p := strings.ToLower(strings.TrimSpace(platform))

	if strings.Contains(target, "linux-musl") {
		switch {
		case strings.Contains(p, "linux-musl"):
			return 3
		case strings.Contains(p, "linux-gnu"):
			return 1
		default:
			return 2
		}
	}
	if strings.Contains(target, "linux-gnu") {
		switch {
		case strings.Contains(p, "linux-gnu"):
			return 3
		case strings.Contains(p, "linux-musl"):
			return 1
		default:
			return 2
		}
	}
	return PlatformScore(platform)
}

// PlatformMatchesRequirement checks if an actual platform satisfies a required platform.
//
// Matching rules:
// - Empty requirement matches only empty actual platform
// - "ruby" requirement matches empty actual platform
// - Darwin versioned platforms (e.g., arm64-darwin-23) match normalized form
// - Otherwise requires exact match or normalized match
func PlatformMatchesRequirement(actualPlatform, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return strings.TrimSpace(actualPlatform) == ""
	}
	if required == "ruby" {
		return strings.TrimSpace(actualPlatform) == ""
	}

	norm := normalizePlatformForIndex(required)
	if norm == "" {
		return false
	}

	actualNorm := normalizePlatformForIndex(actualPlatform)

	// If required is already normalized, check normalized match
	if required == norm {
		return actualNorm == norm
	}

	// Handle Darwin versioned platforms (e.g., arm64-darwin-23)
	if darwinVersionedPlatformRegex.MatchString(required) {
		return actualNorm == norm
	}

	// Exact match required
	return actualPlatform == required
}

// PlatformRequirementSatisfied checks if any platform in the set satisfies the requirement.
func PlatformRequirementSatisfied(platforms map[string]bool, hasRuby bool, required string) bool {
	if required == "" {
		return false
	}
	if required == "ruby" {
		return hasRuby
	}

	norm := normalizePlatformForIndex(required)
	if norm == "" {
		return false
	}

	// If required differs from normalized form
	if required != norm {
		// Handle Darwin versioned platforms
		if darwinVersionedPlatformRegex.MatchString(required) {
			for p := range platforms {
				if normalizePlatformForIndex(p) == norm {
					return true
				}
			}
			return false
		}
		return platforms[required]
	}

	// Check if any platform normalizes to the required form
	for p := range platforms {
		if normalizePlatformForIndex(p) == norm {
			return true
		}
	}
	return false
}

// VersionSupportsPlatforms checks if a version supports all required platforms.
// If no platforms are required, only ruby (empty platform) versions are accepted.
func VersionSupportsPlatforms(hasRuby bool, platforms map[string]bool, required []string) bool {
	if len(required) == 0 {
		return hasRuby
	}
	if hasRuby {
		return true
	}
	for _, platform := range required {
		if platform == "" {
			continue
		}
		if !PlatformRequirementSatisfied(platforms, hasRuby, platform) {
			return false
		}
	}
	return true
}

// DeduplicatePlatforms returns a sorted, deduplicated list of platforms.
func DeduplicatePlatforms(platforms []string) []string {
	set := make(map[string]struct{})
	for _, p := range platforms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		set[p] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for p := range set {
		result = append(result, p)
	}
	slices.Sort(result)
	return result
}
