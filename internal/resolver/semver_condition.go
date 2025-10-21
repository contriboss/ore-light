package resolver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/tinyrange/tinyrange/experimental/pubgrub"
)

// SemverCondition implements pubgrub.Condition using semver constraints
type SemverCondition struct {
	constraint *semver.Constraints
	original   string
}

// NewSemverCondition creates a condition from a semver constraint string
// Examples: "~2.1.0", ">=1.0.0 <2.0.0", "^1.2.3"
func NewSemverCondition(constraintString string) (*SemverCondition, error) {
	// Convert Ruby-style constraints to semver format
	converted := convertRubyConstraint(constraintString)

	constraint, err := semver.NewConstraint(converted)
	if err != nil {
		return nil, fmt.Errorf("invalid constraint %q: %w", constraintString, err)
	}

	return &SemverCondition{
		constraint: constraint,
		original:   constraintString,
	}, nil
}

// Satisfies checks if a version satisfies this condition
func (c *SemverCondition) Satisfies(ver pubgrub.Version) bool {
	version, err := semver.NewVersion(ver.String())
	if err != nil {
		return false
	}
	return c.constraint.Check(version)
}

// String returns a string representation of the condition
func (c *SemverCondition) String() string {
	return c.original
}

// convertRubyConstraint converts Ruby constraint syntax to semver constraint syntax
func convertRubyConstraint(rubyConstraint string) string {
	// Ruby's ~> operator is more permissive than semver's ~:
	// Ruby "~> 3.0" means >= 3.0.0, < 4.0.0 (allows 3.x.x)
	// Semver "~3.0" means >= 3.0.0, < 3.1.0 (only allows 3.0.x)
	//
	// Ruby "~> 3.0.1" means >= 3.0.1, < 3.1.0 (same as semver ~3.0.1)

	converted := rubyConstraint

	// Handle ~> operator
	converted = convertTildeArrow(converted)

	return converted
}

func convertTildeArrow(constraint string) string {
	// Check for ~> operator
	if len(constraint) < 3 || constraint[:2] != "~>" {
		return constraint
	}

	// Extract version after ~>
	versionStr := strings.TrimSpace(constraint[2:])

	// Count the number of version components (major.minor.patch)
	parts := strings.Split(versionStr, ".")

	if len(parts) == 2 {
		// Ruby "~> 3.0" -> semver ">= 3.0.0, < 4.0.0"
		// This allows all 3.x.x versions
		major := parts[0]
		nextMajor := incrementVersion(major)
		return fmt.Sprintf(">= %s.0, < %s.0", versionStr, nextMajor)
	}

	// For 3+ components, standard semver ~ works
	// Ruby "~> 3.0.1" -> semver "~3.0.1"
	return "~" + versionStr
}

func incrementVersion(version string) string {
	// Simple integer increment for major version
	if v, err := strconv.Atoi(strings.TrimSpace(version)); err == nil {
		return strconv.Itoa(v + 1)
	}
	return version
}

// SemverVersion wraps a semver.Version to implement pubgrub.Version
type SemverVersion struct {
	version *semver.Version
}

// NewSemverVersion creates a SemverVersion from a version string
func NewSemverVersion(versionString string) (*SemverVersion, error) {
	version, err := semver.NewVersion(versionString)
	if err != nil {
		return nil, err
	}
	return &SemverVersion{version: version}, nil
}

// String returns the version string
func (v *SemverVersion) String() string {
	if v.version == nil {
		return "0.0.0"
	}
	return v.version.String()
}

// Sort compares this version with another version
// Returns: -1 if this < other, 0 if this == other, 1 if this > other
func (v *SemverVersion) Sort(other pubgrub.Version) int {
	if v.version == nil {
		// Handle nil version case
		if other == nil {
			return 0
		}
		return -1
	}

	otherVersion, err := semver.NewVersion(other.String())
	if err != nil {
		// Fallback to string comparison if parsing fails
		if v.String() < other.String() {
			return -1
		} else if v.String() > other.String() {
			return 1
		}
		return 0
	}

	return v.version.Compare(otherVersion)
}

// AnyVersionCondition accepts any version (for gems with no constraints)
type AnyVersionCondition struct{}

func (c *AnyVersionCondition) Satisfies(ver pubgrub.Version) bool {
	return true
}

func (c *AnyVersionCondition) String() string {
	return ">= 0"
}
