package resolver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/compactindex"
	"github.com/contriboss/pubgrub-go"
)

// CompactIndexSource implements pubgrub.Source using Bundler's compact index cache.
type CompactIndexSource struct {
	client      *compactindex.Client
	cache       map[string]map[string][]pubgrub.Term // gem -> version -> dependencies
	versions    map[string][]pubgrub.Version         // gem -> versions (cached)
	infoCache   map[string][]compactindex.VersionInfo
	mu          sync.RWMutex
	sourceURL   string
	versionPins map[string]string
	overrides   map[string]overrideSpec
	platforms   []string
}

type overrideSpec struct {
	version string
	deps    []pubgrub.Term
}

type availability struct {
	ruby      bool
	platforms map[string]bool
}

// NewCompactIndexSource creates a new compact index source.
// This writes to Bundler's cache: ~/.bundle/cache/compact_index/
func NewCompactIndexSource(baseURL string) (*CompactIndexSource, error) {
	client, err := compactindex.NewClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create compact index client: %w", err)
	}

	return &CompactIndexSource{
		client:      client,
		cache:       make(map[string]map[string][]pubgrub.Term),
		versions:    make(map[string][]pubgrub.Version),
		infoCache:   make(map[string][]compactindex.VersionInfo),
		sourceURL:   baseURL,
		versionPins: nil,
		overrides:   make(map[string]overrideSpec),
	}, nil
}

func (s *CompactIndexSource) getGemInfo(gemName string) ([]compactindex.VersionInfo, error) {
	s.mu.RLock()
	if info, ok := s.infoCache[gemName]; ok {
		s.mu.RUnlock()
		return info, nil
	}
	s.mu.RUnlock()

	ctx := context.Background()
	infoList, err := s.client.GetGemInfo(ctx, gemName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gem info for %s: %w", gemName, err)
	}

	s.mu.Lock()
	s.infoCache[gemName] = infoList
	s.mu.Unlock()

	return infoList, nil
}

// SetVersionPins sets version pins for selective updates.
func (s *CompactIndexSource) SetVersionPins(pins map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versionPins = pins
}

// SetOverrides sets resolver overrides (used for git/path gems).
func (s *CompactIndexSource) SetOverrides(overrides map[string]overrideSpec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides = overrides
	s.cache = make(map[string]map[string][]pubgrub.Term)
	s.versions = make(map[string][]pubgrub.Version)
}

// SetRequiredPlatforms sets the list of platforms that must be supported by selected versions.
func (s *CompactIndexSource) SetRequiredPlatforms(platforms []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := make(map[string]struct{})
	for _, p := range platforms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		set[p] = struct{}{}
	}
	s.platforms = make([]string, 0, len(set))
	for p := range set {
		s.platforms = append(s.platforms, p)
	}
	slices.Sort(s.platforms)
	// Clear cached versions to ensure platform filtering is applied.
	s.versions = make(map[string][]pubgrub.Version)
}

// SourceURL returns the URL of this gem source.
func (s *CompactIndexSource) SourceURL() string {
	return s.sourceURL
}

// GetVersions returns all available versions for a package.
func (s *CompactIndexSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	gemName := name.Value()

	s.mu.RLock()
	if override, ok := s.overrides[gemName]; ok {
		s.mu.RUnlock()
		semverVer, err := NewSemverVersion(override.version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse override version %s for %s: %w", override.version, gemName, err)
		}
		return []pubgrub.Version{semverVer}, nil
	}
	s.mu.RUnlock()

	// Check if this gem is pinned
	s.mu.RLock()
	pinnedVersion := s.versionPins[gemName]
	s.mu.RUnlock()

	if pinnedVersion != "" {
		semverVer, err := NewSemverVersion(pinnedVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pinned version %s for %s: %w", pinnedVersion, gemName, err)
		}
		return []pubgrub.Version{semverVer}, nil
	}

	// Check cache
	s.mu.RLock()
	if versions, ok := s.versions[gemName]; ok {
		s.mu.RUnlock()
		return versions, nil
	}
	s.mu.RUnlock()

	// Fetch from compact index
	infoList, err := s.getGemInfo(gemName)
	if err != nil {
		return nil, err
	}

	requiredPlatforms := s.platforms
	allowPrerelease := allowPrereleases()

	available := make(map[string]*availability)
	for i := range infoList {
		info := &infoList[i]
		if !allowPrerelease && isPrereleaseVersion(info.Version) {
			continue
		}

		entry := available[info.Version]
		if entry == nil {
			entry = &availability{platforms: make(map[string]bool)}
			available[info.Version] = entry
		}

		platform := strings.TrimSpace(info.Platform)
		if platform == "" {
			entry.ruby = true
		} else {
			entry.platforms[platform] = true
		}
	}

	// Convert to SemverVersions
	semverVersions := make([]pubgrub.Version, 0, len(available))
	for versionStr, entry := range available {
		if !versionSupportsPlatforms(entry, requiredPlatforms) {
			continue
		}

		semverVer, err := NewSemverVersion(versionStr)
		if err != nil {
			// Skip versions that can't be parsed
			continue
		}
		semverVersions = append(semverVersions, semverVer)
	}

	// Sort versions low→high
	slices.SortFunc(semverVersions, func(a, b pubgrub.Version) int {
		return a.Sort(b)
	})

	// Cache the result
	s.mu.Lock()
	s.versions[gemName] = semverVersions
	s.mu.Unlock()

	return semverVersions, nil
}

func isPrereleaseVersion(version string) bool {
	for _, r := range version {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// GetDependencies returns the dependencies for a specific package version.
func (s *CompactIndexSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	gemName := name.Value()
	versionStr := version.String()

	s.mu.RLock()
	if override, ok := s.overrides[gemName]; ok {
		s.mu.RUnlock()
		if override.version == "" || override.version == versionStr {
			return override.deps, nil
		}
	}
	s.mu.RUnlock()

	// Check cache first
	s.mu.RLock()
	if versions, ok := s.cache[gemName]; ok {
		if deps, ok := versions[versionStr]; ok {
			s.mu.RUnlock()
			return deps, nil
		}
	}
	s.mu.RUnlock()

	// Fetch from compact index
	infoList, err := s.getGemInfo(gemName)
	if err != nil {
		return nil, err
	}

	// Prefer platform-specific dependencies if all required platforms are available.
	var versionInfo *compactindex.VersionInfo
	if len(s.platforms) > 0 {
		versionInfo = selectPlatformInfo(infoList, versionStr, s.platforms)
	}

	// Fallback to ruby platform dependencies.
	if versionInfo == nil {
		for i := range infoList {
			if infoList[i].Version == versionStr && infoList[i].Platform == "" {
				versionInfo = &infoList[i]
				break
			}
		}
	}

	if versionInfo == nil {
		return nil, fmt.Errorf("version %s not found for gem %s", versionStr, gemName)
	}

	terms := termsFromDependenciesMap(versionInfo.Dependencies)

	// Cache the result
	s.mu.Lock()
	if _, ok := s.cache[gemName]; !ok {
		s.cache[gemName] = make(map[string][]pubgrub.Term)
	}
	s.cache[gemName][versionStr] = terms
	s.mu.Unlock()

	return terms, nil
}

// GetDependenciesForPlatform returns dependencies for a specific version/platform tuple.
func (s *CompactIndexSource) GetDependenciesForPlatform(name pubgrub.Name, version, platform string) ([]pubgrub.Term, error) {
	gemName := name.Value()
	versionStr := strings.TrimSpace(version)
	platform = strings.TrimSpace(platform)

	s.mu.RLock()
	if override, ok := s.overrides[gemName]; ok {
		s.mu.RUnlock()
		if override.version == "" || override.version == versionStr {
			return override.deps, nil
		}
	}
	s.mu.RUnlock()

	infoList, err := s.getGemInfo(gemName)
	if err != nil {
		return nil, err
	}

	var versionInfo *compactindex.VersionInfo
	for i := range infoList {
		info := &infoList[i]
		if info.Version != versionStr {
			continue
		}
		if !platformMatchesRequirement(info.Platform, platform) {
			continue
		}
		if versionInfo == nil || platformScoreWithTarget(platform, info.Platform) > platformScoreWithTarget(platform, versionInfo.Platform) {
			versionInfo = info
		}
	}

	if versionInfo == nil {
		return nil, fmt.Errorf("version %s (%s) not found for gem %s", versionStr, platform, gemName)
	}

	return termsFromDependenciesMap(versionInfo.Dependencies), nil
}

// GetDependenciesMap returns the raw dependency map for a specific version/platform tuple.
func (s *CompactIndexSource) GetDependenciesMap(name, version, platform string) (map[string]string, error) {
	infoList, err := s.getGemInfo(name)
	if err != nil {
		return nil, err
	}

	platform = strings.TrimSpace(platform)
	for i := range infoList {
		info := &infoList[i]
		if info.Version != version {
			continue
		}
		if platform == "" {
			if info.Platform == "" {
				return info.Dependencies, nil
			}
			continue
		}
		if platformMatchesRequirement(info.Platform, platform) {
			return info.Dependencies, nil
		}
	}

	return nil, fmt.Errorf("version %s (%s) not found for gem %s", version, platform, name)
}

// GetChecksum returns the checksum for a gem version/platform if present in the compact index.
func (s *CompactIndexSource) GetChecksum(name, version, platform string) (lockfile.Checksum, bool) {
	infoList, err := s.getGemInfo(name)
	if err != nil {
		return lockfile.Checksum{}, false
	}

	checksum, ok := compactindex.FindChecksum(infoList, version, platform)
	if !ok {
		return lockfile.Checksum{}, false
	}

	parsed, err := lockfile.ParseChecksum(checksum)
	if err != nil && !strings.Contains(checksum, "=") {
		parsed, err = lockfile.ParseChecksum(lockfile.DefaultAlgorithm + "=" + checksum)
	}
	if err != nil {
		return lockfile.Checksum{}, false
	}

	return parsed, true
}

func termsFromDependenciesMap(deps map[string]string) []pubgrub.Term {
	if len(deps) == 0 {
		return nil
	}

	terms := make([]pubgrub.Term, 0, len(deps))
	for depName, constraint := range deps {
		var condition pubgrub.Condition
		if constraint != "" && constraint != ">= 0" {
			// Convert compact index format (& for AND) to standard format (, for AND)
			constraint = strings.ReplaceAll(constraint, "&", ",")
			semverCond, err := NewSemverCondition(constraint)
			if err != nil {
				// If we can't parse the constraint, use AnyVersion
				condition = NewAnyVersionCondition()
			} else {
				condition = semverCond
			}
		} else {
			condition = NewAnyVersionCondition()
		}

		term := pubgrub.NewTerm(pubgrub.MakeName(depName), condition)
		terms = append(terms, term)
	}

	return terms
}

func selectPlatformInfo(infoList []compactindex.VersionInfo, version string, required []string) *compactindex.VersionInfo {
	if len(required) == 0 {
		return nil
	}

	requiredSet := make(map[string]bool, len(required))
	for _, p := range required {
		if p == "" {
			continue
		}
		requiredSet[p] = true
	}
	if len(requiredSet) == 0 {
		return nil
	}

	bestForReq := make(map[string]*compactindex.VersionInfo, len(requiredSet))
	for req := range requiredSet {
		var best *compactindex.VersionInfo
		for i := range infoList {
			info := &infoList[i]
			if info.Version != version {
				continue
			}
			if !platformMatchesRequirement(info.Platform, req) {
				continue
			}
			if best == nil || platformScoreWithTarget(req, info.Platform) > platformScoreWithTarget(req, best.Platform) {
				best = info
			}
		}
		if best == nil {
			return nil
		}
		bestForReq[req] = best
	}

	var best *compactindex.VersionInfo
	for _, info := range bestForReq {
		if best == nil || platformScore(info.Platform) > platformScore(best.Platform) {
			best = info
		}
	}
	return best
}

func platformScore(platform string) int {
	p := strings.ToLower(strings.TrimSpace(platform))
	if strings.Contains(p, "linux") {
		switch {
		case strings.Contains(p, "linux-musl"):
			return 1
		case strings.Contains(p, "linux-gnu"):
			return 2
		default:
			return 3
		}
	}
	if p != "" {
		return 1
	}
	return 0
}

func versionSupportsPlatforms(entry *availability, required []string) bool {
	if entry == nil {
		return false
	}
	if len(required) == 0 {
		return entry.ruby
	}
	if entry.ruby {
		return true
	}
	for _, platform := range required {
		if platform == "" {
			continue
		}
		if !platformRequirementSatisfied(entry, platform) {
			return false
		}
	}
	return true
}

func platformMatchesRequirement(actualPlatform, required string) bool {
	if required == "" {
		return false
	}
	if required == "ruby" {
		return strings.TrimSpace(actualPlatform) == ""
	}
	norm := normalizePlatformForIndex(required)
	if norm == "" {
		return false
	}
	actualNorm := normalizePlatformForIndex(actualPlatform)
	if required == norm {
		return actualNorm == norm
	}
	return actualPlatform == required
}

func platformRequirementSatisfied(entry *availability, required string) bool {
	if required == "" {
		return false
	}
	if required == "ruby" {
		return entry.ruby
	}
	norm := normalizePlatformForIndex(required)
	if norm == "" {
		return false
	}
	if required != norm {
		return entry.platforms[required]
	}
	for p := range entry.platforms {
		if normalizePlatformForIndex(p) == norm {
			return true
		}
	}
	return false
}

func platformScoreWithTarget(target, platform string) int {
	target = strings.ToLower(strings.TrimSpace(target))
	p := strings.ToLower(strings.TrimSpace(platform))
	if strings.Contains(target, "linux-musl") {
		switch {
		case strings.Contains(p, "linux-musl"):
			return 3
		default:
			return 2
		case strings.Contains(p, "linux-gnu"):
			return 1
		}
	}
	if strings.Contains(target, "linux-gnu") {
		switch {
		case strings.Contains(p, "linux-gnu"):
			return 3
		default:
			return 2
		case strings.Contains(p, "linux-musl"):
			return 1
		}
	}
	return platformScore(platform)
}
