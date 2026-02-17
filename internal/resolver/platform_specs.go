package resolver

import (
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/compactindex"
	"github.com/contriboss/pubgrub-go"
)

func buildPlatformSpecs(compactSource *CompactIndexSource, baseSpecs []lockfile.GemSpec, platforms []string, constraintsByGem map[string][]pubgrub.Condition, gemSources map[string]string, gemGroups map[string][]string, versionPins map[string]string, baseVersions map[string]string, existingPlatformVersions map[string]map[string]string) ([]lockfile.GemSpec, map[string]bool) {
	if compactSource == nil {
		return nil, nil
	}

	targetPlatforms := BuildPlatformTargets(platforms)
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
		seenPlatforms := make(map[string]bool)
		allPlatformsResolved := true

		for _, target := range targetPlatforms {
			platform := target.Normalized
			pinned := ""
			if versionPins != nil && versionPins[gemName] != "" {
				pinned = versionPins[gemName]
			} else if baseVersions != nil && baseVersions[gemName] != "" {
				pinned = baseVersions[gemName]
			} else if existingPlatformVersions != nil && existingPlatformVersions[gemName] != nil {
				pinned = existingPlatformVersions[gemName][target.Original]
			}

			allowFallback := pinned == ""
			version, deps, actualPlatform := selectBestPlatformVersion(infoList, platform, target.Original, constraintsByGem[gemName], pinned, allowFallback)
			if version == "" || actualPlatform == "" {
				allPlatformsResolved = false
				break
			}
			if seenPlatforms[actualPlatform] {
				continue
			}
			seenPlatforms[actualPlatform] = true

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

func loadExistingPlatformVersions(lockfilePath string) map[string]map[string]string {
	existing := make(map[string]map[string]string)

	lf, err := lockfile.ParseLockfileIfPresent(lockfilePath)
	if err != nil || lf == nil {
		return existing
	}

	for _, spec := range lf.GemSpecs {
		if spec.Platform == "" {
			continue
		}
		if existing[spec.Name] == nil {
			existing[spec.Name] = make(map[string]string)
		}
		existing[spec.Name][spec.Platform] = spec.Version
	}

	return existing
}

func loadExistingRubySpecs(lockfilePath string) map[string]bool {
	existing := make(map[string]bool)

	lf, err := lockfile.ParseLockfileIfPresent(lockfilePath)
	if err != nil || lf == nil {
		return existing
	}

	for _, spec := range lf.GemSpecs {
		if spec.Platform != "" {
			continue
		}
		existing[spec.Name] = true
	}

	return existing
}

func selectBestPlatformVersion(infoList []compactindex.VersionInfo, platform string, desiredPlatform string, conditions []pubgrub.Condition, pinnedVersion string, allowFallback bool) (string, map[string]string, string) {
	var bestInfo *compactindex.VersionInfo
	var bestVersion *SemverVersion
	allowPrerelease := allowPrereleases()

	trySelect := func(requirePinned bool, exactOnly bool) {
		for i := range infoList {
			info := &infoList[i]
			if normalizePlatformForIndex(info.Platform) != platform {
				continue
			}
			if exactOnly && desiredPlatform != "" && info.Platform != desiredPlatform {
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
				if PlatformScoreWithTarget(desiredPlatform, info.Platform) > PlatformScoreWithTarget(desiredPlatform, bestInfo.Platform) {
					bestVersion = version
					bestInfo = info
				}
			}
		}
	}

	if pinnedVersion != "" {
		trySelect(true, true)
		if bestInfo == nil {
			// Allow platform fallback while keeping the pinned version.
			trySelect(true, false)
		}
	}
	// Fallback: try without pinned version, first exact platform match, then any matching platform.
	if bestInfo == nil && allowFallback {
		trySelect(false, true) // exact platform match
		if bestInfo == nil {
			trySelect(false, false) // any matching platform
		}
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
