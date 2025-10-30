package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SearchResult represents a gem search result from RubyGems API
type SearchResult struct {
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	Info             string   `json:"info"`
	Downloads        int64    `json:"downloads"`
	VersionDownloads int64    `json:"version_downloads"`
	Authors          string   `json:"authors"`
	Licenses         []string `json:"licenses"`
	HomepageURI      string   `json:"homepage_uri"`
	ProjectURI       string   `json:"project_uri"`
	Source           string   `json:"-"` // Not from API, we add this
}

// Search searches for gems across all configured gem sources
func Search(query string, limit int, sources []string) error {
	if len(sources) == 0 {
		// Default to rubygems.org if no sources configured
		sources = []string{"https://rubygems.org"}
	}

	allResults := make([]SearchResult, 0)
	seen := make(map[string]bool) // Deduplicate by gem name

	// Search each source
	for _, source := range sources {
		results, err := searchSource(source, query)
		if err != nil {
			// Don't fail completely if one source fails, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to search %s: %v\n", source, err)
			continue
		}

		// Add results, deduplicating by name
		for _, result := range results {
			if !seen[result.Name] {
				result.Source = source
				allResults = append(allResults, result)
				seen[result.Name] = true
			}
		}
	}

	// Display results
	displaySearchResults(allResults, query, limit)

	return nil
}

// searchSource searches a single gem source
func searchSource(sourceURL, query string) ([]SearchResult, error) {
	// Build API URL
	apiURL := fmt.Sprintf("%s/api/v1/search.json?query=%s",
		strings.TrimSuffix(sourceURL, "/"),
		url.QueryEscape(query))

	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse JSON response
	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to parse results: %w", err)
	}

	return results, nil
}

// displaySearchResults displays the search results with formatted output
func displaySearchResults(results []SearchResult, query string, limit int) {
	if len(results) == 0 {
		fmt.Printf("No gems found matching %q\n", query)
		return
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	downloadsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	// Header
	fmt.Printf("%s %q\n\n", headerStyle.Render("Gems matching"), query)

	// Limit results
	displayCount := limit
	if displayCount > len(results) {
		displayCount = len(results)
	}

	// Display each result
	for i := 0; i < displayCount; i++ {
		gem := results[i]

		// Name and version
		fmt.Printf("%s %s",
			nameStyle.Render(gem.Name),
			versionStyle.Render(fmt.Sprintf("(%s)", gem.Version)),
		)

		// Downloads
		if gem.Downloads > 0 {
			fmt.Printf(" - %s",
				downloadsStyle.Render(formatDownloads(gem.Downloads)),
			)
		}

		fmt.Println()

		// Description
		if gem.Info != "" {
			// Truncate long descriptions
			desc := gem.Info
			if len(desc) > 200 {
				desc = desc[:197] + "..."
			}
			// Clean up whitespace
			desc = strings.ReplaceAll(desc, "\n", " ")
			desc = strings.Join(strings.Fields(desc), " ")

			fmt.Printf("  %s\n", descStyle.Render(desc))
		}

		// Authors
		if gem.Authors != "" {
			authors := gem.Authors
			if len(authors) > 80 {
				authors = authors[:77] + "..."
			}
			fmt.Printf("  %s\n", versionStyle.Render("by "+authors))
		}

		// Source
		if gem.Source != "" {
			fmt.Printf("  %s\n", versionStyle.Render("source: "+gem.Source))
		}

		// URL
		if gem.HomepageURI != "" {
			fmt.Printf("  %s\n", versionStyle.Render(gem.HomepageURI))
		}

		fmt.Println()
	}

	// Summary
	if len(results) > displayCount {
		fmt.Println(countStyle.Render(fmt.Sprintf("Showing %d of %d results", displayCount, len(results))))
	} else {
		fmt.Println(countStyle.Render(fmt.Sprintf("Found %d results", len(results))))
	}
}

// formatDownloads formats download count with human-readable suffixes
func formatDownloads(downloads int64) string {
	if downloads >= 1000000000 {
		return fmt.Sprintf("%.1fB downloads", float64(downloads)/1000000000)
	}
	if downloads >= 1000000 {
		return fmt.Sprintf("%.1fM downloads", float64(downloads)/1000000)
	}
	if downloads >= 1000 {
		return fmt.Sprintf("%.1fK downloads", float64(downloads)/1000)
	}
	return fmt.Sprintf("%d downloads", downloads)
}
