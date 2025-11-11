package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/compactindex"
)

// UpdateType represents the severity of a gem update
type UpdateType int

const (
	UpdatePatch   UpdateType = iota // 1.0.0 -> 1.0.1
	UpdateMinor                     // 1.0.0 -> 1.1.0
	UpdateMajor                     // 1.0.0 -> 2.0.0
	UpdateUnknown                   // Can't determine
)

func (u UpdateType) String() string {
	switch u {
	case UpdatePatch:
		return "PATCH"
	case UpdateMinor:
		return "MINOR"
	case UpdateMajor:
		return "MAJOR"
	default:
		return "UNKNOWN"
	}
}

// OutdatedGem represents a gem that has updates available
type OutdatedGem struct {
	Name           string
	CurrentVersion string
	LatestVersion  string
	Constraint     string
	UpdateType     UpdateType
	Groups         []string // Gem groups (default, development, test, etc.)
	Selected       bool     // Selection state for multi-select update
}

// versionCheckResult holds the result of checking a gem's latest version
type versionCheckResult struct {
	gemName       string
	latestVersion string
	err           error
}

// checkVersionsParallel fetches latest versions using the bulk versions file
// This is MUCH faster than individual gem info files - one HTTP call instead of N
func checkVersionsParallel(ctx context.Context, client *compactindex.Client, gemNames []string) map[string]versionCheckResult {
	results := make(map[string]versionCheckResult)

	// Fetch the versions file once (contains ALL gems)
	// This uses cache and only makes HTTP call if stale (>1 hour)
	allVersions, err := client.GetVersions(ctx)
	if err != nil {
		// If we can't get versions file, return error for all gems
		for _, name := range gemNames {
			results[name] = versionCheckResult{gemName: name, err: err}
		}
		return results
	}

	// Build a map for quick lookup
	versionMap := make(map[string]string)
	for _, entry := range allVersions {
		if len(entry.Versions) > 0 {
			// Versions are sorted oldest first (compact-index is append-only)
			// Iterate from the END to get latest non-yanked version
			for i := len(entry.Versions) - 1; i >= 0; i-- {
				v := entry.Versions[i]
				if !strings.HasPrefix(v, "-") {
					versionMap[entry.Name] = v
					break
				}
			}
		}
	}

	// Look up each gem we need
	for _, name := range gemNames {
		if latestVersion, ok := versionMap[name]; ok {
			results[name] = versionCheckResult{gemName: name, latestVersion: latestVersion}
		} else {
			results[name] = versionCheckResult{gemName: name, err: fmt.Errorf("gem not found in registry")}
		}
	}

	return results
}

// detectUpdateType determines if an update is major, minor, or patch
func detectUpdateType(current, latest string) UpdateType {
	// Parse semver: major.minor.patch
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	if len(currentParts) < 2 || len(latestParts) < 2 {
		return UpdateUnknown
	}

	currentMajor, err1 := strconv.Atoi(currentParts[0])
	latestMajor, err2 := strconv.Atoi(latestParts[0])
	if err1 != nil || err2 != nil {
		return UpdateUnknown
	}

	if latestMajor > currentMajor {
		return UpdateMajor
	}

	if len(currentParts) < 2 || len(latestParts) < 2 {
		return UpdateUnknown
	}

	currentMinor, err1 := strconv.Atoi(currentParts[1])
	latestMinor, err2 := strconv.Atoi(latestParts[1])
	if err1 != nil || err2 != nil {
		return UpdateUnknown
	}

	if latestMinor > currentMinor {
		return UpdateMinor
	}

	return UpdatePatch
}

// LoadOutdatedGems loads all outdated gems from Gemfile and lockfile
func LoadOutdatedGems(gemfilePath string) ([]OutdatedGem, error) {
	// Find the lockfile
	lockfilePath, err := findLockfilePath(gemfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find lockfile: %w", err)
	}

	// Parse Gemfile to get constraints
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Parse lockfile to get current versions
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Build map of gem name -> constraint and groups
	constraints := make(map[string]string)
	gemGroups := make(map[string][]string)
	for _, dep := range parsed.Dependencies {
		if len(dep.Constraints) > 0 {
			constraints[dep.Name] = dep.Constraints[0]
		}
		// Store groups, default to ["default"] if empty
		if len(dep.Groups) > 0 {
			gemGroups[dep.Name] = dep.Groups
		} else {
			gemGroups[dep.Name] = []string{"default"}
		}
	}

	// Determine source URL from Gemfile, fallback to rubygems.org
	sourceURL := "https://rubygems.org"
	for _, src := range parsed.Sources {
		if src.Type == "rubygems" && src.URL != "" {
			sourceURL = src.URL
			break // Use first rubygems source as default
		}
	}

	// Create compactindex client (uses Bundler cache)
	client, err := compactindex.NewClient(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create compactindex client: %w", err)
	}

	ctx := context.Background()

	// Collect gem names
	gemNames := make([]string, len(lock.GemSpecs))
	for i, spec := range lock.GemSpecs {
		gemNames[i] = spec.Name
	}

	// Check all versions
	results := checkVersionsParallel(ctx, client, gemNames)

	// Check if all results have errors (likely network issue)
	errorCount := 0
	for _, result := range results {
		if result.err != nil {
			errorCount++
		}
	}
	if errorCount == len(results) && len(results) > 0 {
		// All gems failed - likely a network or registry issue
		// Return the first error as representative
		for _, result := range results {
			if result.err != nil {
				return nil, fmt.Errorf("failed to check gem versions: %w", result.err)
			}
		}
	}

	// Build outdated gems list
	var outdated []OutdatedGem
	for _, spec := range lock.GemSpecs {
		result := results[spec.Name]

		if result.err != nil || result.latestVersion == "" {
			// Skip individual gem errors (might be yanked or not in registry)
			continue
		}

		// Compare versions
		if result.latestVersion != spec.Version {
			constraint := constraints[spec.Name]
			if constraint == "" {
				constraint = ""
			}

			// Get groups, default to ["default"] if not found
			groups := gemGroups[spec.Name]
			if len(groups) == 0 {
				groups = []string{"default"}
			}

			outdated = append(outdated, OutdatedGem{
				Name:           spec.Name,
				CurrentVersion: spec.Version,
				LatestVersion:  result.latestVersion,
				Constraint:     constraint,
				UpdateType:     detectUpdateType(spec.Version, result.latestVersion),
				Groups:         groups,
				Selected:       false, // Initially not selected
			})
		}
	}

	return outdated, nil
}
