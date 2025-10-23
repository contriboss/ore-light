package audit

import (
	"fmt"
	"strconv"
	"strings"
)

// MatchesVersion checks if a gem version matches a version constraint
// Supports Ruby gem version constraints: >=, <=, >, <, ~>, =, !=
func MatchesVersion(version string, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)

	// Handle multiple constraints separated by comma
	if strings.Contains(constraint, ",") {
		parts := strings.Split(constraint, ",")
		for _, part := range parts {
			matches, err := MatchesVersion(version, strings.TrimSpace(part))
			if err != nil {
				return false, err
			}
			if !matches {
				return false, nil
			}
		}
		return true, nil
	}

	// Parse constraint operator and version
	var op, constraintVer string
	if strings.HasPrefix(constraint, "~>") {
		op = "~>"
		constraintVer = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, ">=") {
		op = ">="
		constraintVer = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "<=") {
		op = "<="
		constraintVer = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "!=") {
		op = "!="
		constraintVer = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, ">") {
		op = ">"
		constraintVer = strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, "<") {
		op = "<"
		constraintVer = strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, "=") {
		op = "="
		constraintVer = strings.TrimSpace(constraint[1:])
	} else {
		// No operator means exact match
		op = "="
		constraintVer = constraint
	}

	cmp, err := compareVersions(version, constraintVer)
	if err != nil {
		return false, err
	}

	switch op {
	case "=":
		return cmp == 0, nil
	case "!=":
		return cmp != 0, nil
	case ">":
		return cmp > 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<":
		return cmp < 0, nil
	case "<=":
		return cmp <= 0, nil
	case "~>":
		return matchesPessimistic(version, constraintVer)
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// matchesPessimistic implements the ~> (pessimistic) operator
// ~> 1.2.3 means >= 1.2.3 and < 1.3.0
// ~> 1.2 means >= 1.2 and < 2.0
func matchesPessimistic(version, constraint string) (bool, error) {
	// Version must be >= constraint
	cmp, err := compareVersions(version, constraint)
	if err != nil {
		return false, err
	}
	if cmp < 0 {
		return false, nil
	}

	// Calculate upper bound
	parts := strings.Split(constraint, ".")
	if len(parts) < 2 {
		return false, fmt.Errorf("pessimistic constraint must have at least 2 parts: %s", constraint)
	}

	// Increment the second-to-last part
	upperParts := make([]string, len(parts)-1)
	copy(upperParts, parts[:len(parts)-1])

	lastIdx := len(upperParts) - 1
	lastPart, err := strconv.Atoi(upperParts[lastIdx])
	if err != nil {
		return false, fmt.Errorf("invalid version part: %s", upperParts[lastIdx])
	}
	upperParts[lastIdx] = strconv.Itoa(lastPart + 1)

	upperBound := strings.Join(upperParts, ".")

	// Version must be < upper bound
	cmp, err = compareVersions(version, upperBound)
	if err != nil {
		return false, err
	}

	return cmp < 0, nil
}

// compareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) (int, error) {
	// Split versions by dots
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		var err error

		if i < len(parts1) {
			p1, err = parseVersionPart(parts1[i])
			if err != nil {
				return 0, err
			}
		}

		if i < len(parts2) {
			p2, err = parseVersionPart(parts2[i])
			if err != nil {
				return 0, err
			}
		}

		if p1 < p2 {
			return -1, nil
		}
		if p1 > p2 {
			return 1, nil
		}
	}

	return 0, nil
}

// parseVersionPart parses a version part, handling prerelease suffixes
// e.g., "3" -> 3, "3.rc1" -> 3 (ignoring prerelease for now)
func parseVersionPart(part string) (int, error) {
	// Handle prerelease versions (alpha, beta, rc, pre)
	// For simplicity, we'll just take the numeric part
	for i, ch := range part {
		if ch < '0' || ch > '9' {
			if i == 0 {
				return 0, nil // Non-numeric prerelease
			}
			part = part[:i]
			break
		}
	}

	if part == "" {
		return 0, nil
	}

	return strconv.Atoi(part)
}

// IsVulnerable checks if a version is vulnerable according to advisory constraints
func IsVulnerable(version string, advisory Advisory) (bool, error) {
	// Check if version is in unaffected range
	for _, constraint := range advisory.UnaffectedVersions {
		matches, err := MatchesVersion(version, constraint)
		if err != nil {
			return false, err
		}
		if matches {
			return false, nil // Not vulnerable
		}
	}

	// Check if version is in patched range
	for _, constraint := range advisory.PatchedVersions {
		matches, err := MatchesVersion(version, constraint)
		if err != nil {
			return false, err
		}
		if matches {
			return false, nil // Patched, not vulnerable
		}
	}

	// If we have patched versions but version doesn't match any, it's vulnerable
	if len(advisory.PatchedVersions) > 0 {
		return true, nil
	}

	// No clear patched versions, assume safe
	return false, nil
}
