package registry

import (
	"testing"
)

func TestProtocolNames(t *testing.T) {
	tests := []struct {
		name     string
		protocol ProtocolName
		expected string
	}{
		{
			name:     "rubygems_protocol",
			protocol: ProtocolRubygems,
			expected: "rubygems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.protocol) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.protocol))
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		protocolName  ProtocolName
		expectedProto ProtocolName
	}{
		{
			name:          "rubygems_explicit",
			baseURL:       "https://rubygems.org/api/v1",
			protocolName:  ProtocolRubygems,
			expectedProto: ProtocolRubygems,
		},
		{
			name:          "default_to_rubygems",
			baseURL:       "https://example.com",
			protocolName:  "unknown",
			expectedProto: ProtocolRubygems,
		},
		{
			name:          "empty_baseurl_rubygems",
			baseURL:       "",
			protocolName:  ProtocolRubygems,
			expectedProto: ProtocolRubygems,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL, tt.protocolName)
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			if client == nil {
				t.Fatal("NewClient() returned nil client")
			}

			if client.ProtocolName() != tt.expectedProto {
				t.Errorf("expected protocol %q, got %q", tt.expectedProto, client.ProtocolName())
			}
		})
	}
}

func TestRubygemsProtocol_Interface(t *testing.T) {
	// Verify RubygemsProtocol implements Protocol interface
	var _ Protocol = (*RubygemsProtocol)(nil)
}

func TestRubygemsProtocol_Name(t *testing.T) {
	protocol := newRubygemsProtocol("https://rubygems.org/api/v1")

	if protocol.Name() != ProtocolRubygems {
		t.Errorf("expected protocol name %q, got %q", ProtocolRubygems, protocol.Name())
	}
}

func TestRubygemsProtocol_BaseURL(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "explicit_url",
			inputURL:    "https://custom.gem.server/api/v1",
			expectedURL: "https://custom.gem.server/api/v1",
		},
		{
			name:        "empty_defaults_to_rubygems",
			inputURL:    "",
			expectedURL: "https://rubygems.org/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protocol := newRubygemsProtocol(tt.inputURL)

			if protocol.BaseURL() != tt.expectedURL {
				t.Errorf("expected BaseURL %q, got %q", tt.expectedURL, protocol.BaseURL())
			}
		})
	}
}

func TestClient_ProtocolDelegation(t *testing.T) {
	tests := []struct {
		name         string
		protocolName ProtocolName
		expectedName ProtocolName
	}{
		{
			name:         "rubygems_protocol",
			protocolName: ProtocolRubygems,
			expectedName: ProtocolRubygems,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("https://example.com", tt.protocolName)
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			if client.ProtocolName() != tt.expectedName {
				t.Errorf("expected protocol name %q, got %q", tt.expectedName, client.ProtocolName())
			}
		})
	}
}

func TestClient_GetBaseURL(t *testing.T) {
	baseURL := "https://custom.gem.server/api/v1"
	client, err := NewClient(baseURL, ProtocolRubygems)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	if client.GetBaseURL() != baseURL {
		t.Errorf("expected base URL %q, got %q", baseURL, client.GetBaseURL())
	}
}
