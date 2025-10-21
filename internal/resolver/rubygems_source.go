package resolver

import (
	"fmt"
	"sync"

	rubygemsclient "github.com/contriboss/rubygems-client-go"
	"github.com/tinyrange/tinyrange/experimental/pubgrub"
)

// RubyGemsSource implements pubgrub.Source using the RubyGems.org API
type RubyGemsSource struct {
	client    *rubygemsclient.Client
	cache     map[string]map[string][]pubgrub.Term // cache[gemName][version] = dependencies
	mu        sync.RWMutex
	sourceURL string // The source URL this client queries
}

// NewRubyGemsSource creates a new RubyGems source for dependency resolution
func NewRubyGemsSource() *RubyGemsSource {
	return NewRubyGemsSourceWithURL("https://rubygems.org")
}

// NewRubyGemsSourceWithURL creates a RubyGems source for a specific gem server
func NewRubyGemsSourceWithURL(baseURL string) *RubyGemsSource {
	return &RubyGemsSource{
		client:    rubygemsclient.NewClientWithBaseURL(baseURL),
		cache:     make(map[string]map[string][]pubgrub.Term),
		sourceURL: baseURL,
	}
}

// SourceURL returns the URL of this gem source
func (s *RubyGemsSource) SourceURL() string {
	return s.sourceURL
}

// GetDependencies returns the dependencies for a specific package version
func (s *RubyGemsSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	gemName := string(name)
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

	// Fetch from RubyGems API
	info, err := s.client.GetGemInfo(gemName, versionStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get gem info for %s@%s: %w", gemName, versionStr, err)
	}

	// Convert runtime dependencies to PubGrub terms
	var terms []pubgrub.Term
	for _, dep := range info.Dependencies.Runtime {
		// Parse version constraint
		var condition pubgrub.Condition
		if dep.Requirements != "" && dep.Requirements != ">= 0" {
			semverCond, err := NewSemverCondition(dep.Requirements)
			if err != nil {
				// If we can't parse the constraint, use AnyVersion
				condition = &AnyVersionCondition{}
			} else {
				condition = semverCond
			}
		} else {
			condition = &AnyVersionCondition{}
		}

		term := pubgrub.NewTerm(pubgrub.Name(dep.Name), condition)
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

// GetVersions returns all available versions for a package
func (s *RubyGemsSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	gemName := string(name)

	versions, err := s.client.GetGemVersions(gemName)
	if err != nil {
		return nil, fmt.Errorf("failed to get versions for %s: %w", gemName, err)
	}

	// Convert to SemverVersions
	semverVersions := make([]pubgrub.Version, 0, len(versions))
	for _, v := range versions {
		semverVer, err := NewSemverVersion(v)
		if err != nil {
			// Skip versions that can't be parsed
			continue
		}
		semverVersions = append(semverVersions, semverVer)
	}

	return semverVersions, nil
}
