package registry

import "context"

// ProtocolName represents supported registry protocols
type ProtocolName string

const (
	// ProtocolRubygems represents the rubygems.org protocol
	ProtocolRubygems ProtocolName = "rubygems"
)

// Protocol defines the interface for gem registry protocols.
type Protocol interface {
	// GetGemInfo retrieves metadata for a specific gem version
	GetGemInfo(ctx context.Context, name, version string) (*GemInfo, error)

	// GetGemVersions retrieves all available versions for a gem
	GetGemVersions(ctx context.Context, name string) ([]string, error)

	// Name returns the protocol identifier
	Name() ProtocolName

	// BaseURL returns the registry base URL
	BaseURL() string
}

// DependencyCategories represents the dependency structure
type DependencyCategories struct {
	Development []Dependency
	Runtime     []Dependency
}

// Dependency represents a gem dependency
type Dependency struct {
	Name         string
	Requirements string
}

// GemInfo represents unified gem metadata across protocols
type GemInfo struct {
	Name         string
	Version      string
	Dependencies DependencyCategories
}

// Client provides protocol-agnostic access to gem registries.
// It wraps protocol-specific implementations and delegates operations
// to the appropriate protocol handler.
type Client struct {
	protocol Protocol
}

// NewClient creates a registry client with the specified protocol.
// Defaults to Rubygems protocol if protocolName is unknown.
func NewClient(baseURL string, protocolName ProtocolName) (*Client, error) {
	var protocol Protocol

	switch protocolName {
	case ProtocolRubygems:
		fallthrough
	default:
		protocol = newRubygemsProtocol(baseURL)
	}

	return &Client{protocol: protocol}, nil
}

// GetGemInfo retrieves gem metadata using the configured protocol
func (c *Client) GetGemInfo(ctx context.Context, name, version string) (*GemInfo, error) {
	return c.protocol.GetGemInfo(ctx, name, version)
}

// GetGemVersions retrieves all versions for a gem using the configured protocol
func (c *Client) GetGemVersions(ctx context.Context, name string) ([]string, error) {
	return c.protocol.GetGemVersions(ctx, name)
}

// ProtocolName returns the name of the protocol being used
func (c *Client) ProtocolName() ProtocolName {
	return c.protocol.Name()
}

// GetBaseURL returns the base URL of the registry
func (c *Client) GetBaseURL() string {
	return c.protocol.BaseURL()
}
