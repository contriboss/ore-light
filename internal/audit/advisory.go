package audit

import (
	"time"
)

// Advisory represents a security vulnerability advisory for a gem
type Advisory struct {
	Gem                string    `yaml:"gem"`
	Framework          string    `yaml:"framework,omitempty"`
	CVE                string    `yaml:"cve,omitempty"`
	GHSA               string    `yaml:"ghsa,omitempty"`
	URL                string    `yaml:"url"`
	Title              string    `yaml:"title"`
	Date               time.Time `yaml:"date"`
	Description        string    `yaml:"description"`
	UnaffectedVersions []string  `yaml:"unaffected_versions,omitempty"`
	PatchedVersions    []string  `yaml:"patched_versions"`
	Criticality        string    `yaml:"criticality,omitempty"`
}

// ID returns the advisory identifier (CVE or GHSA)
func (a *Advisory) ID() string {
	if a.CVE != "" {
		return "CVE-" + a.CVE
	}
	if a.GHSA != "" {
		return "GHSA-" + a.GHSA
	}
	return "UNKNOWN"
}

// Severity returns the criticality level with a default
func (a *Advisory) Severity() string {
	if a.Criticality != "" {
		return a.Criticality
	}
	return "Unknown"
}
