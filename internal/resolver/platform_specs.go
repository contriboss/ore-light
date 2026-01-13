package resolver

import (
	"sort"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/compactindex"
	"github.com/contriboss/pubgrub-go"
)

func buildPlatformSpecs(compactSource *CompactIndexSource, baseSpecs []lockfile.GemSpec, platforms []string, constraintsByGem map[string][]pubgrub.Condition, gemSources map[string]string, gemGroups map[string][]string, versionPins map[string]string, baseVersions map[string]string, existingPlatformVersions map[string]map[string]string) ([]lockfile.GemSpec, map[string]bool) {
	if compactSource == nil {
		return nil, nil
	}

	targetPlatforms := normalizePlatformTargets(platforms)
	if len(targetPlatforms) == 0 {
		return nil, nil
	}

	specs := make([]lockfile.GemSpec, 0)
	gemsWithPlatformSpecs := make(map[string]bool)

	for _, base := range baseSpecs {
		gemName := base.Name
		infoList, err := compactSource.getGemInfo(gemName)
		if err != nil {
			continue
		}

		perPlatform := make([]lockfile.GemSpec, 0, len(targetPlatforms))
		allPlatformsResolved := true

		for _, platform := range targetPlatforms {
			pinned := ""
			if versionPins != nil && versionPins[gemName] != "" {
				pinned = versionPins[gemName]
			} else if baseVersions != nil && baseVersions[gemName] != "" {
				pinned = baseVersions[gemName]
			} else if existingPlatformVersions != nil && existingPlatformVersions[gemName] != nil {
				pinned = existingPlatformVersions[gemName][platform]
			}

			allowFallback := pinned == ""
			version, deps, actualPlatform := selectBestPlatformVersion(infoList, platform, constraintsByGem[gemName], pinned, allowFallback)
			if version == "" || actualPlatform == "" {
				allPlatformsResolved = false
				break
			}

			lockDeps := dependenciesFromCompactMap(deps)
			perPlatform = append(perPlatform, lockfile.GemSpec{
				Name:         gemName,
				Version:      version,
				Platform:     actualPlatform,
				Dependencies: lockDeps,
				SourceURL:    gemSources[gemName],
				Groups:       gemGroups[gemName],
			})
		}

		if allPlatformsResolved && len(perPlatform) > 0 {
			specs = append(specs, perPlatform...)
			gemsWithPlatformSpecs[gemName] = true
		}
	}

	return specs, gemsWithPlatformSpecs
}

func normalizePlatformTargets(platforms []string) []string {
	set := make(map[string]struct{})
	for _, platform := range platforms {
		normalized := normalizePlatformForIndex(platform)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}

	targets := make([]string, 0, len(set))
	for platform := range set {
		targets = append(targets, platform)
	}
	sort.Strings(targets)
	return targets
}

func loadExistingPlatformVersions(lockfilePath string) map[string]map[string]string {
	existing := make(map[string]map[string]string)

	lf, err := lockfile.ParseFile(lockfilePath)
	if err != nil || lf == nil {
		return existing
	}

	for _, spec := range lf.GemSpecs {
		if spec.Platform == "" {
			continue
		}
		platform := normalizePlatformForIndex(spec.Platform)
		if platform == "" {
			continue
		}
		if existing[spec.Name] == nil {
			existing[spec.Name] = make(map[string]string)
		}
		existing[spec.Name][platform] = spec.Version
	}

	return existing
}

func selectBestPlatformVersion(infoList []compactindex.VersionInfo, platform string, conditions []pubgrub.Condition, pinnedVersion string, allowFallback bool) (string, map[string]string, string) {
	var bestInfo *compactindex.VersionInfo
	var bestVersion *SemverVersion
	allowPrerelease := allowPrereleases()

	platformScore := func(p string) int {
		p = strings.ToLower(p)
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

	trySelect := func(requirePinned bool) {
		for i := range infoList {
			info := &infoList[i]
			if normalizePlatformForIndex(info.Platform) != platform {
				continue
			}
			if requirePinned && pinnedVersion != "" && info.Version != pinnedVersion {
				continue
			}
			if !allowPrerelease && isPrereleaseVersion(info.Version) {
				continue
			}

			version, err := NewSemverVersion(info.Version)
			if err != nil {
				continue
			}

			if !satisfiesAllConditions(version, conditions) {
				continue
			}

			if bestVersion == nil {
				bestVersion = version
				bestInfo = info
				continue
			}
			cmp := version.Sort(bestVersion)
			if cmp > 0 {
				bestVersion = version
				bestInfo = info
				continue
			}
			if cmp == 0 && bestInfo != nil {
				if platformScore(info.Platform) > platformScore(bestInfo.Platform) {
					bestVersion = version
					bestInfo = info
				}
			}
		}
	}

	if pinnedVersion != "" {
		trySelect(true)
	}
	if bestInfo == nil && allowFallback {
		trySelect(false)
	}

	if bestInfo == nil {
		return "", nil, ""
	}

	return bestInfo.Version, bestInfo.Dependencies, bestInfo.Platform
}

func satisfiesAllConditions(version pubgrub.Version, conditions []pubgrub.Condition) bool {
	for _, condition := range conditions {
		if condition == nil {
			continue
		}
		if !condition.Satisfies(version) {
			return false
		}
	}
	return true
}
