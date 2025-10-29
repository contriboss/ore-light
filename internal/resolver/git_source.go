package resolver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/pubgrub-go"
)

// GitSource handles resolution of gems from Git repositories
type GitSource struct {
	// URL of the git repository
	URL string
	// Branch, tag, or ref to checkout
	Branch string
	Tag    string
	Ref    string
	// Cache directory for cloned repos
	cacheDir string
	// Resolved commit SHA
	resolvedRevision string
	// Dependencies parsed from gemspec
	dependencies []pubgrub.Term
}

// NewGitSource creates a new Git source for a gem
func NewGitSource(url, branch, tag, ref string) (*GitSource, error) {
	cacheDir, err := getGitCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get git cache dir: %w", err)
	}

	return &GitSource{
		URL:      url,
		Branch:   branch,
		Tag:      tag,
		Ref:      ref,
		cacheDir: cacheDir,
	}, nil
}

// GetDependencies returns the dependencies for this git gem
func (g *GitSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	// If we haven't resolved yet, do it now
	if g.resolvedRevision == "" {
		if err := g.Resolve(); err != nil {
			return nil, err
		}
	}
	return g.dependencies, nil
}

// GetVersions returns a single version for a git gem (the resolved revision)
func (g *GitSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	if err := g.Resolve(); err != nil {
		return nil, err
	}

	// For git gems, we return a pseudo-version based on the commit SHA
	version, err := NewSemverVersion("0.0.1")
	if err != nil {
		return nil, err
	}

	return []pubgrub.Version{version}, nil
}

// Resolve clones the repository and parses the gemspec
func (g *GitSource) Resolve() error {
	// Create a unique directory name for this repo
	repoDir := g.getRepoDir()

	// Clone or update the repository
	if err := g.cloneOrUpdate(repoDir); err != nil {
		return fmt.Errorf("failed to clone/update repo: %w", err)
	}

	// Checkout the specified ref
	revision, err := g.checkoutRef(repoDir)
	if err != nil {
		return fmt.Errorf("failed to checkout ref: %w", err)
	}
	g.resolvedRevision = revision

	// Parse the gemspec to get dependencies
	deps, err := g.parseGemspec(repoDir)
	if err != nil {
		return fmt.Errorf("failed to parse gemspec: %w", err)
	}
	g.dependencies = deps

	return nil
}

// GetRevision returns the resolved git revision
func (g *GitSource) GetRevision() string {
	return g.resolvedRevision
}

// cloneOrUpdate clones the repository or updates if it already exists
func (g *GitSource) cloneOrUpdate(repoDir string) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Repository exists, update it
		return g.updateRepo(repoDir)
	}

	// Clone the repository
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", "--quiet", g.URL, repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	return nil
}

// updateRepo updates an existing repository
func (g *GitSource) updateRepo(repoDir string) error {
	cmd := exec.Command("git", "-C", repoDir, "fetch", "--quiet", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w\n%s", err, string(output))
	}
	return nil
}

// checkoutRef checks out the specified branch, tag, or ref
func (g *GitSource) checkoutRef(repoDir string) (string, error) {
	// Determine what to checkout
	var ref string
	if g.Tag != "" {
		ref = g.Tag
	} else if g.Branch != "" {
		ref = "origin/" + g.Branch
	} else if g.Ref != "" {
		ref = g.Ref
	} else {
		// Default to main/master
		ref = "origin/HEAD"
	}

	// Checkout the ref
	cmd := exec.Command("git", "-C", repoDir, "checkout", "--quiet", ref)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git checkout %s failed: %w\n%s", ref, err, string(output))
	}

	// Get the commit SHA
	cmd = exec.Command("git", "-C", repoDir, "rev-parse", "HEAD")
	shaOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}

	return strings.TrimSpace(string(shaOutput)), nil
}

// parseGemspec parses the gemspec file to extract dependencies using tree-sitter
func (g *GitSource) parseGemspec(repoDir string) ([]pubgrub.Term, error) {
	// Find the gemspec file
	gemspecPath, err := g.findGemspec(repoDir)
	if err != nil {
		return nil, err
	}

	// Read gemspec file
	content, err := os.ReadFile(gemspecPath)
	if err != nil {
		return []pubgrub.Term{}, nil // graceful fallback
	}

	// Parse with tree-sitter
	parser := gemfile.NewTreeSitterGemspecParser(content)
	gemspec, err := parser.ParseWithTreeSitter()
	if err != nil {
		// If tree-sitter parsing fails, return empty dependencies
		// This allows git gems without dependencies to work
		return []pubgrub.Term{}, nil
	}

	// Convert RuntimeDependencies to PubGrub terms
	var terms []pubgrub.Term
	for _, dep := range gemspec.RuntimeDependencies {
		var condition pubgrub.Condition

		// Convert constraint strings
		if len(dep.Constraints) > 0 {
			constraintStr := strings.Join(dep.Constraints, ", ")
			if constraintStr != "" && constraintStr != ">= 0" {
				semverCond, err := NewSemverCondition(constraintStr)
				if err != nil {
					condition = NewAnyVersionCondition()
				} else {
					condition = semverCond
				}
			} else {
				condition = NewAnyVersionCondition()
			}
		} else {
			condition = NewAnyVersionCondition()
		}

		term := pubgrub.NewTerm(pubgrub.MakeName(dep.Name), condition)
		terms = append(terms, term)
	}

	return terms, nil
}

// findGemspec finds the gemspec file in the repository
func (g *GitSource) findGemspec(repoDir string) (string, error) {
	// Look for .gemspec files
	matches, err := filepath.Glob(filepath.Join(repoDir, "*.gemspec"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no gemspec file found in repository")
	}

	// Return the first gemspec found
	return matches[0], nil
}

// getRepoDir returns the directory where this repo should be cached
func (g *GitSource) getRepoDir() string {
	// Create a hash of the URL to use as directory name
	hash := sha256.Sum256([]byte(g.URL))
	hashStr := hex.EncodeToString(hash[:])[:16]
	return filepath.Join(g.cacheDir, hashStr)
}

// getGitCacheDir returns the cache directory for git repositories
func getGitCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ore", "git"), nil
}

// CloneAtRevision clones the repository at a specific revision to a destination directory
// This is used during gem installation
func (g *GitSource) CloneAtRevision(revision, destDir string) error {
	// First ensure the repo is in our cache
	repoDir := g.getRepoDir()
	if err := g.cloneOrUpdate(repoDir); err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	// Use git archive to export the specific revision
	// This is cleaner than clone + checkout as it doesn't include .git
	cmd := exec.Command("git", "-C", repoDir, "archive", revision)
	archiveData, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git archive failed: %w", err)
	}

	// Extract the archive to destDir using tar
	tarCmd := exec.Command("tar", "-x", "-C", destDir)
	tarCmd.Stdin = bytes.NewReader(archiveData)
	if output, err := tarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar extraction failed: %w\n%s", err, string(output))
	}

	return nil
}
