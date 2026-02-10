package resolver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/ore-light/internal/config"
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
	// Version from gemspec
	version string
	// Cached repository reference
	repo *git.Repository
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

	// For git gems, prefer the gemspec version (fallback to 0.0.1)
	versionStr := g.version
	if versionStr == "" {
		versionStr = "0.0.1"
	}
	version, err := NewSemverVersion(versionStr)
	if err != nil {
		version, _ = NewSemverVersion("0.0.1")
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
	version, deps, err := g.parseGemspec(repoDir)
	if err != nil {
		return fmt.Errorf("failed to parse gemspec: %w", err)
	}
	g.version = version
	g.dependencies = deps

	return nil
}

// GetRevision returns the resolved git revision
func (g *GitSource) GetRevision() string {
	return g.resolvedRevision
}

// GetVersion returns the version parsed from the gemspec.
func (g *GitSource) GetVersion() string {
	return g.version
}

// cloneOrUpdate clones the repository or updates if it already exists
func (g *GitSource) cloneOrUpdate(repoDir string) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Repository exists, update it
		return g.updateRepo(repoDir)
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL: g.URL,
	}

	// Use shallow clone for branch/tag (not for raw SHA refs)
	if g.Branch != "" {
		cloneOpts.Depth = 1
		cloneOpts.SingleBranch = true
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(g.Branch)
	} else if g.Tag != "" {
		cloneOpts.Depth = 1
		cloneOpts.SingleBranch = true
		cloneOpts.ReferenceName = plumbing.NewTagReferenceName(g.Tag)
	}
	// ref (SHA) = full clone, no Depth set

	repo, err := git.PlainClone(repoDir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	g.repo = repo

	return nil
}

// updateRepo updates an existing repository
func (g *GitSource) updateRepo(repoDir string) error {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	g.repo = repo

	err = repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	return nil
}

// checkoutRef checks out the specified branch, tag, or ref
func (g *GitSource) checkoutRef(repoDir string) (string, error) {
	if g.repo == nil {
		repo, err := git.PlainOpen(repoDir)
		if err != nil {
			return "", fmt.Errorf("failed to open repository: %w", err)
		}
		g.repo = repo
	}

	wt, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	checkoutOpts := &git.CheckoutOptions{
		Force: true,
	}

	if g.Tag != "" {
		// Checkout tag
		ref, err := g.repo.Tag(g.Tag)
		if err != nil {
			// Try as lightweight tag reference
			tagRef, err := g.repo.Reference(plumbing.NewTagReferenceName(g.Tag), true)
			if err != nil {
				return "", fmt.Errorf("tag %s not found: %w", g.Tag, err)
			}
			checkoutOpts.Hash = tagRef.Hash()
		} else {
			checkoutOpts.Hash = ref.Hash()
		}
	} else if g.Branch != "" {
		// Checkout remote branch
		remoteRef, err := g.repo.Reference(plumbing.NewRemoteReferenceName("origin", g.Branch), true)
		if err != nil {
			return "", fmt.Errorf("branch origin/%s not found: %w", g.Branch, err)
		}
		checkoutOpts.Hash = remoteRef.Hash()
	} else if g.Ref != "" {
		// Checkout specific commit SHA
		checkoutOpts.Hash = plumbing.NewHash(g.Ref)
	} else {
		// Default: checkout HEAD
		headRef, err := g.repo.Head()
		if err != nil {
			// Try origin/HEAD or origin/main
			remoteRef, err := g.repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
			if err != nil {
				// Fallback to origin/main
				remoteRef, err = g.repo.Reference(plumbing.NewRemoteReferenceName("origin", "main"), true)
				if err != nil {
					// Fallback to origin/master
					remoteRef, err = g.repo.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true)
					if err != nil {
						return "", fmt.Errorf("no default branch found: %w", err)
					}
				}
			}
			checkoutOpts.Hash = remoteRef.Hash()
		} else {
			checkoutOpts.Hash = headRef.Hash()
		}
	}

	if err := wt.Checkout(checkoutOpts); err != nil {
		return "", fmt.Errorf("checkout failed: %w", err)
	}

	// Get HEAD SHA after checkout
	head, err := g.repo.Head()
	if err != nil {
		// If HEAD is detached, use the hash we checked out
		return checkoutOpts.Hash.String(), nil
	}
	return head.Hash().String(), nil
}

// parseGemspec parses the gemspec file to extract dependencies using tree-sitter
func (g *GitSource) parseGemspec(repoDir string) (string, []pubgrub.Term, error) {
	// Find the gemspec file
	gemspecPath, err := g.findGemspec(repoDir)
	if err != nil {
		return "", nil, err
	}

	// Read gemspec file
	content, err := os.ReadFile(gemspecPath)
	if err != nil {
		return "", []pubgrub.Term{}, nil // graceful fallback
	}

	// Parse with tree-sitter
	parser := gemfile.NewTreeSitterGemspecParser(content)
	gemspec, err := parser.ParseWithTreeSitter()
	if err != nil {
		// If tree-sitter parsing fails, return empty dependencies
		// This allows git gems without dependencies to work
		return "", []pubgrub.Term{}, nil
	}

	version := resolveGemspecVersion(gemspec.Version, filepath.Dir(gemspecPath))

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

	return version, terms, nil
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
	cacheHome, err := config.GetXDGCacheHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheHome, "ore", "git"), nil
}

// CloneAtRevision clones the repository at a specific revision to a destination directory.
// This is used during gem installation.
// The .git directory is preserved because Bundler requires it to verify the gem
// is checked out at the correct revision (see Bundler::Source::Git::GitProxy).
func (g *GitSource) CloneAtRevision(revision, destDir string) error {
	// First ensure the repo is in our cache
	repoDir := g.getRepoDir()
	if err := g.cloneOrUpdate(repoDir); err != nil {
		return err
	}

	// Clone from our cache to the destination directory
	// We use --shared to save disk space (objects are hard-linked from cache)
	// Note: --shared works because destDir is typically on the same filesystem as cache
	cloneCmd := exec.Command("git", "clone", "--shared", repoDir, destDir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		// Fallback to regular clone if --shared fails (e.g., cross-filesystem)
		cloneCmd = exec.Command("git", "clone", repoDir, destDir)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
		}
		_ = output // discard unused
	}

	// Checkout the specific revision
	checkoutCmd := exec.Command("git", "-C", destDir, "checkout", "--detach", revision)
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %w\n%s", err, string(output))
	}

	return nil
}
