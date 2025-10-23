package audit

import (
	"fmt"

	"github.com/contriboss/gemfile-go/lockfile"
)

// Vulnerability represents a found security vulnerability
type Vulnerability struct {
	Gem      lockfile.GemSpec
	Advisory Advisory
}

// Scanner scans gems for security vulnerabilities
type Scanner struct {
	Database *Database
}

// NewScanner creates a new vulnerability scanner
func NewScanner(db *Database) *Scanner {
	return &Scanner{Database: db}
}

// Scan checks all gems for vulnerabilities
func (s *Scanner) Scan(gems []lockfile.GemSpec) ([]Vulnerability, error) {
	if !s.Database.Exists() {
		return nil, fmt.Errorf("advisory database not found; run `ore audit update` first")
	}

	var vulnerabilities []Vulnerability

	for _, gem := range gems {
		// Load advisories for this gem
		advisories, err := s.Database.LoadAdvisories(gem.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to load advisories for %s: %w", gem.Name, err)
		}

		// Check each advisory
		for _, advisory := range advisories {
			vulnerable, err := IsVulnerable(gem.Version, advisory)
			if err != nil {
				// Log warning but continue
				fmt.Printf("Warning: failed to check %s %s against %s: %v\n",
					gem.Name, gem.Version, advisory.ID(), err)
				continue
			}

			if vulnerable {
				vulnerabilities = append(vulnerabilities, Vulnerability{
					Gem:      gem,
					Advisory: advisory,
				})
			}
		}
	}

	return vulnerabilities, nil
}

// ScanResult contains the results of a security scan
type ScanResult struct {
	Vulnerabilities []Vulnerability
	ScannedGems     int
	VulnerableGems  map[string]bool // Set of vulnerable gem names
}

// ScanWithReport runs a scan and returns a detailed report
func (s *Scanner) ScanWithReport(gems []lockfile.GemSpec) (*ScanResult, error) {
	vulnerabilities, err := s.Scan(gems)
	if err != nil {
		return nil, err
	}

	// Build set of vulnerable gem names
	vulnGems := make(map[string]bool)
	for _, vuln := range vulnerabilities {
		vulnGems[vuln.Gem.Name] = true
	}

	return &ScanResult{
		Vulnerabilities: vulnerabilities,
		ScannedGems:     len(gems),
		VulnerableGems:  vulnGems,
	}, nil
}

// HasVulnerabilities returns true if any vulnerabilities were found
func (r *ScanResult) HasVulnerabilities() bool {
	return len(r.Vulnerabilities) > 0
}

// VulnerabilityCount returns the number of vulnerabilities found
func (r *ScanResult) VulnerabilityCount() int {
	return len(r.Vulnerabilities)
}

// VulnerableGemCount returns the number of unique gems with vulnerabilities
func (r *ScanResult) VulnerableGemCount() int {
	return len(r.VulnerableGems)
}
