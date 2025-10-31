package registry

import (
	"context"

	rubygems "github.com/contriboss/rubygems-client-go"
)

// RubygemsProtocol adapts rubygems-client-go to the Protocol interface.
// It provides access to the legacy rubygems.org API.
type RubygemsProtocol struct {
	client  *rubygems.Client
	baseURL string
}

// newRubygemsProtocol creates a new Rubygems protocol adapter
func newRubygemsProtocol(baseURL string) *RubygemsProtocol {
	if baseURL == "" {
		baseURL = "https://rubygems.org/api/v1"
	}

	client := rubygems.NewClientWithBaseURL(baseURL)

	return &RubygemsProtocol{
		client:  client,
		baseURL: baseURL,
	}
}

// Name returns the protocol identifier
func (p *RubygemsProtocol) Name() ProtocolName {
	return ProtocolRubygems
}

// BaseURL returns the registry base URL
func (p *RubygemsProtocol) BaseURL() string {
	return p.baseURL
}

// GetGemInfo retrieves gem metadata from rubygems.org API
func (p *RubygemsProtocol) GetGemInfo(ctx context.Context, name, version string) (*GemInfo, error) {
	// Call rubygems-client-go
	info, err := p.client.GetGemInfo(name, version)
	if err != nil {
		return nil, err
	}

	// Adapt rubygems-client-go types to registry types
	runtimeDeps := make([]Dependency, len(info.Dependencies.Runtime))
	for i, dep := range info.Dependencies.Runtime {
		runtimeDeps[i] = Dependency{
			Name:         dep.Name,
			Requirements: dep.Requirements,
		}
	}

	devDeps := make([]Dependency, len(info.Dependencies.Development))
	for i, dep := range info.Dependencies.Development {
		devDeps[i] = Dependency{
			Name:         dep.Name,
			Requirements: dep.Requirements,
		}
	}

	return &GemInfo{
		Name:    info.Name,
		Version: info.Version,
		Dependencies: DependencyCategories{
			Runtime:     runtimeDeps,
			Development: devDeps,
		},
	}, nil
}

// GetGemVersions retrieves all available versions for a gem
func (p *RubygemsProtocol) GetGemVersions(ctx context.Context, name string) ([]string, error) {
	// rubygems-client-go already returns []string, no adaptation needed
	return p.client.GetGemVersions(name)
}
