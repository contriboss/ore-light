package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
)

func TestGemsRbGemsLockedInteraction(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create gems.rb
	gemfilePath := filepath.Join(tmpDir, "gems.rb")
	gemfileContent := "source 'https://rubygems.org'\ngem 'rake'\n"
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Test 'install' with gems.rb
	t.Run("InstallGemsRb", func(t *testing.T) {
		callbacks := InstallCallbacks{
			GetDownloadManager: func(workers int) (DownloadManager, error) {
				return &mockDownloadManager{cacheDir: t.TempDir()}, nil
			},
			GetDefaultVendorDir: func() string {
				return filepath.Join(tmpDir, "vendor")
			},
			InstallFromCache: func(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
				return InstallReport{Installed: len(gems)}, nil
			},
			InstallGitGems: func(ctx context.Context, vendorDir, rubyScope string, gitSpecs []lockfile.GitGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
				return InstallReport{}, nil
			},
			InstallPathGems: func(ctx context.Context, vendorDir, rubyScope string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig, lockfileDir string) (InstallReport, error) {
				return InstallReport{}, nil
			},
		}

		// ore install -gemfile gems.rb
		args := []string{"-gemfile", "gems.rb"}
		if err := RunInstall(args, callbacks); err != nil {
			t.Fatalf("RunInstall failed: %v", err)
		}

		// It should have created gems.locked, NOT gems.rb.lock
		if _, err := os.Stat("gems.locked"); os.IsNotExist(err) {
			t.Errorf("Expected gems.locked to be created")
		}
		if _, err := os.Stat("gems.rb.lock"); err == nil {
			t.Errorf("gems.rb.lock should NOT have been created")
		}
	})

	// 2. Test 'check' with gems.rb
	t.Run("CheckGemsRb", func(t *testing.T) {
		// Create a minimal vendor dir to avoid unrelated failures
		vendorDir := filepath.Join(tmpDir, "vendor")
		if err := os.MkdirAll(filepath.Join(vendorDir, "gems"), 0755); err != nil {
			t.Fatal(err)
		}
		// gems.locked exists from previous step
		args := []string{"-gemfile", "gems.rb", "-vendor", vendorDir}
		if err := RunCheck(args); err != nil {
			// Check if the error is about a missing lockfile or wrong lockfile
			if strings.Contains(err.Error(), "gems.rb.lock") {
				t.Errorf("RunCheck tried to look for gems.rb.lock instead of gems.locked: %v", err)
			}
		}
	})

	// 3. Test 'exec' with gems.rb
	t.Run("ExecGemsRb", func(t *testing.T) {
		vendorDir := filepath.Join(tmpDir, "vendor")
		buildEnv := func(vDir string, specs []lockfile.GemSpec) ([]string, error) {
			return os.Environ(), nil
		}
		args := []string{"-gemfile", "gems.rb", "-vendor", vendorDir, "--", "true"}
		if err := RunExec(args, buildEnv); err != nil {
			if strings.Contains(err.Error(), "gems.rb.lock") {
				t.Errorf("RunExec tried to look for gems.rb.lock instead of gems.locked: %v", err)
			} else {
				t.Fatalf("RunExec failed: %v", err)
			}
		}
	})

	// 4. Test 'show' with gems.rb
	t.Run("ShowGemsRb", func(t *testing.T) {
		vendorDir := filepath.Join(tmpDir, "vendor")
		args := []string{"-gemfile", "gems.rb", "-vendor", vendorDir, "rake"}
		err := RunShow(args)
		// We expect "gem rake is in lockfile but not installed" or success, NOT "failed to find lockfile"
		if err != nil && (strings.Contains(err.Error(), "gems.rb.lock") || strings.Contains(err.Error(), "gems.rb.lock.lock")) {
			t.Errorf("RunShow tried to look for wrong lockfile: %v", err)
		}
	})

	// 5. Test 'why' with gems.rb
	t.Run("WhyGemsRb", func(t *testing.T) {
		args := []string{"-gemfile", "gems.rb", "rake"}
		if err := RunWhy(args); err != nil {
			t.Fatalf("RunWhy failed: %v", err)
		}
	})

	// 6. Test 'tree' with gems.rb
	t.Run("TreeGemsRb", func(t *testing.T) {
		args := []string{"-gemfile", "gems.rb"}
		if err := RunTree(args); err != nil {
			t.Fatalf("RunTree failed: %v", err)
		}
	})

	// 7. Test 'audit' with gems.rb
	t.Run("AuditGemsRb", func(t *testing.T) {
		args := []string{"-gemfile", "gems.rb"}
		err := RunAudit(args)
		// Advisory DB might be missing, that's fine.
		// We want to ensure it doesn't fail with "failed to parse lockfile" due to wrong path.
		if err != nil && strings.Contains(err.Error(), "gems.rb.lock") {
			t.Fatalf("RunAudit failed with wrong lockfile path: %v", err)
		}
	})

	// 8. Test 'clean' with gems.rb
	t.Run("CleanGemsRb", func(t *testing.T) {
		args := []string{"-gemfile", "gems.rb", "-dry-run"}
		if err := RunClean(args); err != nil {
			t.Fatalf("RunClean failed: %v", err)
		}
	})
}
