package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/contriboss/gemfile-go/lockfile"
)

// TreeNode represents a gem in the dependency tree
type TreeNode struct {
	Gem      lockfile.GemSpec
	Children []*TreeNode
	Visited  bool
}

// buildDependencyTree builds a hierarchical tree from lockfile specs
func buildDependencyTree(specs []lockfile.GemSpec) map[string]*TreeNode {
	// Create a map of all gems by name
	gemMap := make(map[string]lockfile.GemSpec)
	for _, spec := range specs {
		gemMap[spec.Name] = spec
	}

	// Create tree nodes
	nodeMap := make(map[string]*TreeNode)
	for _, spec := range specs {
		nodeMap[spec.Name] = &TreeNode{
			Gem:      spec,
			Children: []*TreeNode{},
		}
	}

	// Build parent-child relationships
	for name, node := range nodeMap {
		for _, dep := range node.Gem.Dependencies {
			if childNode, exists := nodeMap[dep.Name]; exists {
				nodeMap[name].Children = append(nodeMap[name].Children, childNode)
			}
		}
		// Sort children by name for consistent output
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Gem.Name < node.Children[j].Gem.Name
		})
	}

	return nodeMap
}

// findRootGems identifies gems that are direct dependencies (have groups)
func findRootGems(specs []lockfile.GemSpec) []lockfile.GemSpec {
	var roots []lockfile.GemSpec
	for _, spec := range specs {
		if len(spec.Groups) > 0 {
			roots = append(roots, spec)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Name < roots[j].Name
	})
	return roots
}

// Styles for tree rendering
var (
	gemNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")). // Cyan
			Bold(true)

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Gray

	platformStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")). // Orange
			Italic(true)

	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("135")) // Purple

	treeCharStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark gray
)

// renderTree renders the dependency tree with Unicode box-drawing characters
func renderTree(node *TreeNode, prefix string, isLast bool, visited map[string]bool) {
	if node.Visited || visited[node.Gem.Name] {
		// Already shown this gem, indicate circular/shared dependency
		connector := "├──"
		if isLast {
			connector = "└──"
		}
		fmt.Printf("%s%s %s %s %s\n",
			prefix,
			treeCharStyle.Render(connector),
			gemNameStyle.Render(node.Gem.Name),
			versionStyle.Render(node.Gem.Version),
			versionStyle.Render("(already shown)"),
		)
		return
	}

	visited[node.Gem.Name] = true

	// Build the current line
	connector := "├──"
	extension := "│  "
	if isLast {
		connector = "└──"
		extension = "   "
	}

	// Gem info
	gemInfo := fmt.Sprintf("%s %s",
		gemNameStyle.Render(node.Gem.Name),
		versionStyle.Render(node.Gem.Version),
	)

	// Add platform if present
	if node.Gem.Platform != "" {
		gemInfo += " " + platformStyle.Render(fmt.Sprintf("[%s]", node.Gem.Platform))
	}

	// Add groups if present
	if len(node.Gem.Groups) > 0 {
		gemInfo += " " + groupStyle.Render(fmt.Sprintf("(%s)", strings.Join(node.Gem.Groups, ", ")))
	}

	fmt.Printf("%s%s %s\n",
		prefix,
		treeCharStyle.Render(connector),
		gemInfo,
	)

	// Render children
	newPrefix := prefix + treeCharStyle.Render(extension)
	for i, child := range node.Children {
		renderTree(child, newPrefix, i == len(node.Children)-1, visited)
	}
}

// printDependencyTree prints the entire dependency tree
func printDependencyTree(specs []lockfile.GemSpec) {
	nodeMap := buildDependencyTree(specs)
	rootGems := findRootGems(specs)

	if len(rootGems) == 0 {
		fmt.Println("No root gems found (gems with groups)")
		return
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Underline(true)

	fmt.Println(headerStyle.Render("Dependency Tree"))
	fmt.Println()

	// Render each root gem
	for i, root := range rootGems {
		if node, exists := nodeMap[root.Name]; exists {
			isLast := i == len(rootGems)-1

			// Root level formatting
			gemInfo := fmt.Sprintf("%s %s",
				gemNameStyle.Render(root.Name),
				versionStyle.Render(root.Version),
			)

			if root.Platform != "" {
				gemInfo += " " + platformStyle.Render(fmt.Sprintf("[%s]", root.Platform))
			}

			if len(root.Groups) > 0 {
				gemInfo += " " + groupStyle.Render(fmt.Sprintf("(%s)", strings.Join(root.Groups, ", ")))
			}

			fmt.Printf("%s\n", gemInfo)

			// Render children
			childVisited := make(map[string]bool)
			for j, child := range node.Children {
				renderTree(child, "", j == len(node.Children)-1, childVisited)
			}

			if !isLast {
				fmt.Println()
			}
		}
	}

	// Summary
	fmt.Println()
	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	uniqueGems := len(nodeMap)
	fmt.Println(summaryStyle.Render(fmt.Sprintf("Total: %d gems", uniqueGems)))
}

// renderTreePlain renders without colors for non-TTY
func renderTreePlain(node *TreeNode, prefix string, isLast bool, visited map[string]bool) {
	if visited[node.Gem.Name] {
		connector := "├──"
		if isLast {
			connector = "└──"
		}
		fmt.Printf("%s%s %s %s (already shown)\n",
			prefix, connector, node.Gem.Name, node.Gem.Version)
		return
	}

	visited[node.Gem.Name] = true

	connector := "├──"
	extension := "│  "
	if isLast {
		connector = "└──"
		extension = "   "
	}

	gemInfo := fmt.Sprintf("%s %s", node.Gem.Name, node.Gem.Version)
	if node.Gem.Platform != "" {
		gemInfo += fmt.Sprintf(" [%s]", node.Gem.Platform)
	}
	if len(node.Gem.Groups) > 0 {
		gemInfo += fmt.Sprintf(" (%s)", strings.Join(node.Gem.Groups, ", "))
	}

	fmt.Printf("%s%s %s\n", prefix, connector, gemInfo)

	newPrefix := prefix + extension
	for i, child := range node.Children {
		renderTreePlain(child, newPrefix, i == len(node.Children)-1, visited)
	}
}

// printDependencyTreePlain prints tree without colors
func printDependencyTreePlain(specs []lockfile.GemSpec) {
	nodeMap := buildDependencyTree(specs)
	rootGems := findRootGems(specs)

	if len(rootGems) == 0 {
		fmt.Println("No root gems found")
		return
	}

	fmt.Println("Dependency Tree")
	fmt.Println()

	for i, root := range rootGems {
		if node, exists := nodeMap[root.Name]; exists {
			gemInfo := fmt.Sprintf("%s %s", root.Name, root.Version)
			if root.Platform != "" {
				gemInfo += fmt.Sprintf(" [%s]", root.Platform)
			}
			if len(root.Groups) > 0 {
				gemInfo += fmt.Sprintf(" (%s)", strings.Join(root.Groups, ", "))
			}

			fmt.Printf("%s\n", gemInfo)

			childVisited := make(map[string]bool)
			for j, child := range node.Children {
				renderTreePlain(child, "", j == len(node.Children)-1, childVisited)
			}

			if i < len(rootGems)-1 {
				fmt.Println()
			}
		}
	}

	fmt.Printf("\nTotal: %d gems\n", len(nodeMap))
}

// isTTY checks if stdout is a terminal
func isTTY() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
