package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/contriboss/gemfile-go/lockfile"
)

// Why shows why a gem is in the bundle by displaying dependency chains
func Why(gemName string) error {
	// Parse lockfile
	lock, err := lockfile.ParseFile("Gemfile.lock")
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile.lock: %w", err)
	}

	// Find the target gem
	var targetGem *lockfile.GemSpec
	for _, spec := range lock.GemSpecs {
		if spec.Name == gemName {
			targetGem = &spec
			break
		}
	}

	if targetGem == nil {
		return fmt.Errorf("gem %q not found in Gemfile.lock", gemName)
	}

	// Build reverse dependency map (gem -> gems that depend on it)
	reverseDeps := buildReverseDeps(lock.GemSpecs)

	// Find all paths from root gems to target
	paths := findDependencyPaths(lock, reverseDeps, gemName)

	// Display results
	displayWhyResults(gemName, targetGem, paths)

	return nil
}

// buildReverseDeps builds a map of gem -> list of gems that depend on it
func buildReverseDeps(specs []lockfile.GemSpec) map[string][]string {
	reverseDeps := make(map[string][]string)

	for _, spec := range specs {
		for _, dep := range spec.Dependencies {
			reverseDeps[dep.Name] = append(reverseDeps[dep.Name], spec.Name)
		}
	}

	// Sort for consistent output
	for gem := range reverseDeps {
		sort.Strings(reverseDeps[gem])
	}

	return reverseDeps
}

// DependencyPath represents a chain of dependencies leading to a gem
type DependencyPath struct {
	Chain []string // List of gem names from root to target
}

// findDependencyPaths finds all paths from root gems to the target gem
func findDependencyPaths(lock *lockfile.Lockfile, reverseDeps map[string][]string, targetGem string) []DependencyPath {
	// Find root gems from lockfile Dependencies section
	rootGems := make(map[string]bool)
	for _, dep := range lock.Dependencies {
		rootGems[dep.Name] = true
	}

	// Check if target is a root gem
	if rootGems[targetGem] {
		return []DependencyPath{{Chain: []string{targetGem}}}
	}

	var paths []DependencyPath
	visited := make(map[string]bool)

	// BFS from target backwards to find all roots that lead to it
	type node struct {
		gem  string
		path []string
	}

	queue := []node{{gem: targetGem, path: []string{targetGem}}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Get all gems that depend on current gem
		dependents := reverseDeps[current.gem]

		if len(dependents) == 0 {
			// No dependents - this shouldn't happen for non-root gems
			continue
		}

		for _, dependent := range dependents {
			// Build new path
			newPath := append([]string{dependent}, current.path...)

			// Check if this dependent is a root gem
			if rootGems[dependent] {
				paths = append(paths, DependencyPath{Chain: newPath})
			} else {
				// Continue searching backwards
				pathKey := strings.Join(newPath, "->")
				if !visited[pathKey] {
					visited[pathKey] = true
					queue = append(queue, node{gem: dependent, path: newPath})
				}
			}
		}
	}

	// Sort paths by length and then alphabetically
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i].Chain) != len(paths[j].Chain) {
			return len(paths[i].Chain) < len(paths[j].Chain)
		}
		return strings.Join(paths[i].Chain, " ") < strings.Join(paths[j].Chain, " ")
	})

	return paths
}

// displayWhyResults displays the dependency chain results
func displayWhyResults(gemName string, gem *lockfile.GemSpec, paths []DependencyPath) {
	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	gemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	arrowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	// Header
	source := gem.SourceURL
	if source == "" {
		source = "rubygems"
	}
	fmt.Printf("%s %s %s\n\n",
		headerStyle.Render(gemName),
		versionStyle.Render(gem.Version),
		versionStyle.Render(fmt.Sprintf("(%s)", source)),
	)

	if len(paths) == 0 {
		fmt.Println("No dependency paths found (this shouldn't happen)")
		return
	}

	// Display paths
	for _, path := range paths {
		parts := make([]string, len(path.Chain))
		for i, gemName := range path.Chain {
			if i == 0 {
				// Root gem - highlight it
				parts[i] = headerStyle.Render(gemName)
			} else {
				parts[i] = gemStyle.Render(gemName)
			}
		}
		arrow := arrowStyle.Render(" → ")
		fmt.Printf("  %s\n", strings.Join(parts, arrow))
	}

	// Summary
	fmt.Println()
	pathWord := "path"
	if len(paths) != 1 {
		pathWord = "paths"
	}
	fmt.Println(countStyle.Render(fmt.Sprintf("Found %d dependency %s", len(paths), pathWord)))
}
