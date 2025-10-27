package resolver

import (
	"fmt"
	"sync"

	"github.com/contriboss/pubgrub-go"
	rubygemsclient "github.com/contriboss/rubygems-client-go"
)

// RubyGemsSource implements pubgrub.Source using the RubyGems.org API
//
// Ruby developers: This is a concurrent-safe cache using Go's sync.RWMutex.
// Unlike Ruby's Thread::Mutex, RWMutex allows multiple readers OR one writer.
// It's like ActiveSupport's Concurrent::Map but built into the language.
// The struct is thread-safe by design - not by accident!
type RubyGemsSource struct {
	client    *rubygemsclient.Client
	cache     map[string]map[string][]pubgrub.Term // Nested map requires careful locking
	mu        sync.RWMutex                         // RWMutex = Read-Write mutex for performance
	sourceURL string                               // The source URL this client queries
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
				condition = NewAnyVersionCondition()
			} else {
				condition = semverCond
			}
		} else {
			condition = NewAnyVersionCondition()
		}

		term := pubgrub.NewTerm(pubgrub.MakeName(dep.Name), condition)
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
	gemName := name.Value()

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
