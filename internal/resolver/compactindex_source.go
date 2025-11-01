package resolver

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/contriboss/ore-light/internal/compactindex"
	"github.com/contriboss/pubgrub-go"
)

// CompactIndexSource implements pubgrub.Source using Bundler's compact index cache.
type CompactIndexSource struct {
	client      *compactindex.Client
	cache       map[string]map[string][]pubgrub.Term // gem -> version -> dependencies
	versions    map[string][]pubgrub.Version         // gem -> versions (cached)
	mu          sync.RWMutex
	sourceURL   string
	versionPins map[string]string
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
		sourceURL:   baseURL,
		versionPins: nil,
	}, nil
}

// SetVersionPins sets version pins for selective updates.
func (s *CompactIndexSource) SetVersionPins(pins map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versionPins = pins
}

// SourceURL returns the URL of this gem source.
func (s *CompactIndexSource) SourceURL() string {
	return s.sourceURL
}

// GetVersions returns all available versions for a package.
func (s *CompactIndexSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	gemName := name.Value()

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
	ctx := context.Background()
	infoList, err := s.client.GetGemInfo(ctx, gemName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gem info for %s: %w", gemName, err)
	}

	// Convert to SemverVersions
	semverVersions := make([]pubgrub.Version, 0, len(infoList))
	for _, info := range infoList {
		// Skip platform-specific versions for now
		if info.Platform != "" {
			continue
		}

		semverVer, err := NewSemverVersion(info.Version)
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

// GetDependencies returns the dependencies for a specific package version.
func (s *CompactIndexSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	gemName := name.Value()
	versionStr := version.String()

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
	ctx := context.Background()
	infoList, err := s.client.GetGemInfo(ctx, gemName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gem info for %s: %w", gemName, err)
	}

	// Find the specific version
	var versionInfo *compactindex.VersionInfo
	for i := range infoList {
		if infoList[i].Version == versionStr && infoList[i].Platform == "" {
			versionInfo = &infoList[i]
			break
		}
	}

	if versionInfo == nil {
		return nil, fmt.Errorf("version %s not found for gem %s", versionStr, gemName)
	}

	// Convert dependencies to PubGrub terms
	var terms []pubgrub.Term
	for depName, constraint := range versionInfo.Dependencies {
		// Parse version constraint
		var condition pubgrub.Condition
		if constraint != "" && constraint != ">= 0" {
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

	// Cache the result
	s.mu.Lock()
	if _, ok := s.cache[gemName]; !ok {
		s.cache[gemName] = make(map[string][]pubgrub.Term)
	}
	s.cache[gemName][versionStr] = terms
	s.mu.Unlock()

	return terms, nil
}
