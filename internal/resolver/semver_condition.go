package resolver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/contriboss/pubgrub-go"
)

var constraintRegex = regexp.MustCompile(`^(>=|<=|>|<|!=|==|=)?\s*(.+?)\s*$`)

// SemverCondition implements pubgrub.Condition using RubyGems-style version semantics.
type SemverCondition struct {
	requirements []gemRequirement
	original     string
}

// NewSemverCondition creates a condition from a Ruby-style constraint string.
func NewSemverCondition(constraintString string) (*SemverCondition, error) {
	trimmed := strings.TrimSpace(constraintString)
	if trimmed == "" || trimmed == ">= 0" {
		return &SemverCondition{
			requirements: nil,
			original:     constraintString,
		}, nil
	}

	var requirements []gemRequirement

	parts := strings.Split(trimmed, ",")
	for _, part := range parts {
		clause := strings.TrimSpace(part)
		if clause == "" {
			continue
		}

		if strings.HasPrefix(clause, "~>") {
			versionStr := strings.TrimSpace(clause[2:])
			if versionStr == "" {
				return nil, fmt.Errorf("invalid constraint %q", clause)
			}

			lower, err := NewSemverVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid ~> constraint %q: %w", clause, err)
			}

			upper, err := lower.pessimisticUpperBound()
			if err != nil {
				return nil, fmt.Errorf("invalid ~> constraint %q: %w", clause, err)
			}

			requirements = append(requirements,
				gemRequirement{op: ">=", version: lower},
				gemRequirement{op: "<", version: upper},
			)
			continue
		}

		matches := constraintRegex.FindStringSubmatch(clause)
		if matches == nil {
			return nil, fmt.Errorf("invalid constraint %q", clause)
		}

		op := normalizeOp(matches[1])
		versionStr := strings.TrimSpace(matches[2])
		if versionStr == "" {
			return nil, fmt.Errorf("invalid constraint %q", clause)
		}

		version, err := NewSemverVersion(versionStr)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q in constraint %q: %w", versionStr, clause, err)
		}

		requirements = append(requirements, gemRequirement{op: op, version: version})
	}

	return &SemverCondition{
		requirements: requirements,
		original:     constraintString,
	}, nil
}

// Satisfies checks if a version satisfies this condition.
func (c *SemverCondition) Satisfies(ver pubgrub.Version) bool {
	target, err := ensureSemverVersion(ver)
	if err != nil {
		return false
	}

	for _, req := range c.requirements {
		if !req.satisfiedBy(target) {
			return false
		}
	}

	return true
}

// String returns a string representation of the condition
func (c *SemverCondition) String() string {
	if strings.TrimSpace(c.original) == "" {
		return ">= 0"
	}
	return c.original
}

// ToVersionSet converts this condition to a VersionSet for CDCL solver support.
// This uses the exported helpers in pubgrub-go to build intervals directly with
// our SemverVersion type, avoiding ParseVersionRange which creates SemanticVersion.
func (c *SemverCondition) ToVersionSet() pubgrub.VersionSet {
	// If no requirements, return full set (any version)
	if len(c.requirements) == 0 {
		return pubgrub.FullVersionSet()
	}

	// Start with full set and intersect each requirement
	result := pubgrub.FullVersionSet()

	for _, req := range c.requirements {
		// Convert each requirement to a VersionSet interval using our SemverVersion
		var interval pubgrub.VersionSet

		switch req.op {
		case "=", "==":
			// Exact version: [version, version]
			interval = pubgrub.NewVersionRangeSet(req.version, true, req.version, true)

		case "!=":
			// Not equal: complement of singleton
			singleton := pubgrub.NewVersionRangeSet(req.version, true, req.version, true)
			interval = singleton.Complement()

		case ">":
			// Greater than (exclusive): (version, +∞)
			interval = pubgrub.NewLowerBoundVersionSet(req.version, false)

		case ">=":
			// Greater than or equal (inclusive): [version, +∞)
			interval = pubgrub.NewLowerBoundVersionSet(req.version, true)

		case "<":
			// Less than (exclusive): (-∞, version)
			interval = pubgrub.NewUpperBoundVersionSet(req.version, false)

		case "<=":
			// Less than or equal (inclusive): (-∞, version]
			interval = pubgrub.NewUpperBoundVersionSet(req.version, true)

		default:
			// Unknown operator, accept any version
			interval = pubgrub.FullVersionSet()
		}

		// Intersect this requirement with the result (AND logic)
		result = result.Intersection(interval)

		// If we've narrowed down to nothing, no point continuing
		if result.IsEmpty() {
			return result
		}
	}

	return result
}

type gemRequirement struct {
	op      string
	version *SemverVersion
}

func (r gemRequirement) satisfiedBy(version *SemverVersion) bool {
	comparison := version.compare(r.version)

	switch r.op {
	case "=", "==":
		return comparison == 0
	case "!=":
		return comparison != 0
	case ">":
		return comparison > 0
	case "<":
		return comparison < 0
	case ">=":
		return comparison >= 0
	case "<=":
		return comparison <= 0
	default:
		return true
	}
}

func normalizeOp(op string) string {
	if op == "" {
		return "="
	}
	if op == "==" {
		return "="
	}
	return op
}

// SemverVersion implements pubgrub.Version using RubyGems comparison rules.
type SemverVersion struct {
	original string
	segments []versionSegment
}

type versionSegment struct {
	numeric bool
	num     int64
	str     string
}

// NewSemverVersion creates a SemverVersion from a version string
func NewSemverVersion(versionString string) (*SemverVersion, error) {
	normalized := strings.TrimSpace(versionString)
	if normalized == "" {
		normalized = "0"
	}

	segments := parseSegments(normalized)
	if len(segments) == 0 {
		return nil, fmt.Errorf("invalid version %q", versionString)
	}

	return &SemverVersion{
		original: normalized,
		segments: segments,
	}, nil
}

func newSemverVersionFromSegments(segments []versionSegment) *SemverVersion {
	copied := make([]versionSegment, len(segments))
	copy(copied, segments)
	copied = trimTrailingZeros(copied)

	return &SemverVersion{
		original: segmentsToString(copied),
		segments: copied,
	}
}

// String returns the version string
func (v *SemverVersion) String() string {
	if v == nil || v.original == "" {
		return "0"
	}
	return v.original
}

// Sort compares this version with another version
// Returns: -1 if this < other, 0 if this == other, 1 if this > other
func (v *SemverVersion) Sort(other pubgrub.Version) int {
	if v == nil {
		if other == nil {
			return 0
		}
		return -1
	}

	otherVersion, err := ensureSemverVersion(other)
	if err != nil {
		switch {
		case v.String() < other.String():
			return -1
		case v.String() > other.String():
			return 1
		default:
			return 0
		}
	}

	return v.compare(otherVersion)
}

func (v *SemverVersion) compare(other *SemverVersion) int {
	maxLen := len(v.segments)
	if len(other.segments) > maxLen {
		maxLen = len(other.segments)
	}

	for i := 0; i < maxLen; i++ {
		left := segmentAt(v.segments, i)
		right := segmentAt(other.segments, i)

		if left.equal(right) {
			continue
		}

		if left.numeric && right.numeric {
			if left.num < right.num {
				return -1
			}
			return 1
		}

		if left.numeric {
			return 1
		}

		if right.numeric {
			return -1
		}

		if left.str < right.str {
			return -1
		}
		if left.str > right.str {
			return 1
		}
	}

	return 0
}

func (v *SemverVersion) pessimisticUpperBound() (*SemverVersion, error) {
	if len(v.segments) == 0 {
		return NewSemverVersion("0")
	}

	pivot := len(v.segments) - 2
	if pivot < 0 {
		pivot = 0
	}

	if !v.segments[pivot].numeric {
		return nil, fmt.Errorf("cannot apply ~> to non-numeric segment in %s", v.original)
	}

	newSegments := make([]versionSegment, pivot+1)
	copy(newSegments, v.segments[:pivot+1])
	newSegments[pivot].num++

	return newSemverVersionFromSegments(newSegments), nil
}

// AnyVersionCondition accepts any version (for gems with no constraints)
type AnyVersionCondition struct{}

func (c *AnyVersionCondition) Satisfies(ver pubgrub.Version) bool {
	return true
}

func (c *AnyVersionCondition) String() string {
	return ">= 0"
}

// NewAnyVersionCondition creates a condition that matches any version
// Uses pubgrub-go's FullVersionSet for compatibility
func NewAnyVersionCondition() pubgrub.Condition {
	return pubgrub.NewVersionSetCondition(pubgrub.FullVersionSet())
}

func ensureSemverVersion(ver pubgrub.Version) (*SemverVersion, error) {
	if ver == nil {
		return nil, fmt.Errorf("nil version")
	}

	if existing, ok := ver.(*SemverVersion); ok {
		return existing, nil
	}

	return NewSemverVersion(ver.String())
}

func parseSegments(version string) []versionSegment {
	normalized := strings.ReplaceAll(version, "-", ".")
	normalized = strings.ReplaceAll(normalized, "_", ".")

	rawParts := strings.Split(normalized, ".")
	segments := make([]versionSegment, 0, len(rawParts))

	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			segments = append(segments, versionSegment{numeric: true, num: 0})
			continue
		}

		if num, err := strconv.ParseInt(part, 10, 64); err == nil {
			segments = append(segments, versionSegment{numeric: true, num: num})
			continue
		}

		segments = append(segments, versionSegment{
			numeric: false,
			str:     strings.ToLower(part),
		})
	}

	return trimTrailingZeros(segments)
}

func trimTrailingZeros(segments []versionSegment) []versionSegment {
	i := len(segments) - 1
	for i >= 0 {
		seg := segments[i]
		if seg.numeric && seg.num == 0 {
			i--
			continue
		}
		break
	}

	if i < 0 {
		return []versionSegment{{numeric: true, num: 0}}
	}

	return segments[:i+1]
}

func segmentAt(segments []versionSegment, index int) versionSegment {
	if index >= 0 && index < len(segments) {
		return segments[index]
	}
	return versionSegment{numeric: true, num: 0}
}

func (s versionSegment) equal(other versionSegment) bool {
	if s.numeric != other.numeric {
		return false
	}

	if s.numeric {
		return s.num == other.num
	}

	return s.str == other.str
}

func segmentsToString(segments []versionSegment) string {
	parts := make([]string, len(segments))
	for i, seg := range segments {
		if seg.numeric {
			parts[i] = strconv.FormatInt(seg.num, 10)
		} else {
			parts[i] = seg.str
		}
	}
	return strings.Join(parts, ".")
}
