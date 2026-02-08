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

func TestGemfileFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a custom Gemfile in a subdirectory
	customDir := filepath.Join(tmpDir, "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	gemfilePath := filepath.Join(customDir, "MyGemfile")
	gemfileContent := "source 'https://rubygems.org'\ngem 'rake'\n"
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("InstallWithGemfileFlag", func(t *testing.T) {
		// Mock callbacks for RunInstall
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
			InstallPathGems: func(ctx context.Context, vendorDir, rubyScope string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
				return InstallReport{}, nil
			},
		}

		// Run install with -gemfile flag
		// It should automatically generate MyGemfile.lock
		args := []string{"-gemfile", gemfilePath}
		if err := RunInstall(args, callbacks); err != nil {
			t.Fatalf("RunInstall failed: %v", err)
		}

		// Verify lockfile was created
		lockfilePath := gemfilePath + ".lock"
		if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
			t.Errorf("Expected lockfile to be created at %s", lockfilePath)
		}
	})

	t.Run("AddWithGemfileFlag", func(t *testing.T) {
		// Run add with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "rspec"}
		if err := RunAdd(args); err != nil {
			t.Fatalf("RunAdd failed: %v", err)
		}

		// Verify Gemfile content
		content, err := os.ReadFile(gemfilePath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "rspec") {
			t.Errorf("Expected MyGemfile to contain 'rspec', got:\n%s", string(content))
		}
	})

	t.Run("RemoveWithGemfileFlag", func(t *testing.T) {
		// Run remove with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "rake"}
		if err := RunRemove(args); err != nil {
			t.Fatalf("RunRemove failed: %v", err)
		}

		// Verify Gemfile content
		content, err := os.ReadFile(gemfilePath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "gem 'rake'") {
			t.Errorf("Expected MyGemfile NOT to contain 'rake', got:\n%s", string(content))
		}
	})

	t.Run("CheckWithGemfileFlag", func(t *testing.T) {
		// Update lockfile to match Gemfile state after Add and Remove
		// Gemfile should now contain 'rspec' and NOT 'rake'
		if err := RunLock([]string{"-gemfile", gemfilePath}); err != nil {
			t.Fatalf("RunLock failed: %v", err)
		}

		// Create a vendor directory with the expected structure
		vendorDir := filepath.Join(tmpDir, "vendor_check")
		gemsDir := filepath.Join(vendorDir, "gems")
		if err := os.MkdirAll(gemsDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Parse the newly created lockfile to see what versions were resolved
		parsed, err := loadLockfile(gemfilePath + ".lock")
		if err != nil {
			t.Fatalf("failed to load lockfile: %v", err)
		}

		// Create dummy gem directories for all gems in the lockfile
		for _, spec := range parsed.GemSpecs {
			if err := os.MkdirAll(filepath.Join(gemsDir, spec.FullName()), 0755); err != nil {
				t.Fatal(err)
			}
		}

		// Run check with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "-vendor", vendorDir}
		if err := RunCheck(args); err != nil {
			t.Fatalf("RunCheck failed: %v", err)
		}
	})

	t.Run("ExecWithGemfileFlag", func(t *testing.T) {
		// Mock buildEnv for RunExec
		buildEnv := func(vendorDir string, specs []lockfile.GemSpec) ([]string, error) {
			return os.Environ(), nil
		}

		// Run exec with -gemfile flag
		// We use 'true' as the command because it exists on most systems and returns 0
		args := []string{"-gemfile", gemfilePath, "--", "true"}
		if err := RunExec(args, buildEnv); err != nil {
			t.Fatalf("RunExec failed: %v", err)
		}
	})

	t.Run("UpdateWithGemfileFlag", func(t *testing.T) {
		// Run update with -gemfile flag
		args := []string{"-gemfile", gemfilePath}
		if err := RunUpdate(args); err != nil {
			t.Fatalf("RunUpdate failed: %v", err)
		}
	})

	t.Run("AuditWithGemfileFlag", func(t *testing.T) {
		// Run audit with -gemfile flag
		// Note: this will likely fail because advisory database is missing,
		// but we want to check if it parses the flags and finds the lockfile
		args := []string{"-gemfile", gemfilePath}
		err := RunAudit(args)
		if err == nil {
			// Unexpected success? Maybe db exists in environment
		} else if strings.Contains(err.Error(), "failed to parse lockfile") || strings.Contains(err.Error(), "failed to find lockfile") {
			t.Fatalf("RunAudit failed to find/parse lockfile: %v", err)
		}
	})

	t.Run("TreeWithGemfileFlag", func(t *testing.T) {
		// Run tree with -gemfile flag
		args := []string{"-gemfile", gemfilePath}
		if err := RunTree(args); err != nil {
			t.Fatalf("RunTree failed: %v", err)
		}
	})

	t.Run("CleanWithGemfileFlag", func(t *testing.T) {
		// Run clean with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "-dry-run"}
		if err := RunClean(args); err != nil {
			t.Fatalf("RunClean failed: %v", err)
		}
	})
	t.Run("ShowWithGemfileFlag", func(t *testing.T) {
		// Run show with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "--paths"}
		if err := RunShow(args); err != nil {
			t.Fatalf("RunShow failed: %v", err)
		}
	})

	t.Run("WhyWithGemfileFlag", func(t *testing.T) {
		// Run why with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "rspec"}
		if err := RunWhy(args); err != nil {
			t.Fatalf("RunWhy failed: %v", err)
		}
	})

	t.Run("OutdatedWithGemfileFlag", func(t *testing.T) {
		// Run outdated with -gemfile flag
		args := []string{"-gemfile", gemfilePath, "--plain"}
		// Note: LoadOutdatedGems might fail without actual network or registry mock,
		// but we want to check flag parsing and lockfile derivation.
		err := RunOutdated(args)
		if err != nil {
			// Require either success or an error that clearly comes from the version-check network path
			if !strings.Contains(err.Error(), "failed to check") {
				t.Fatalf("RunOutdated failed with unexpected error (likely lockfile/flag issue): %v", err)
			}
		}
	})

	t.Run("PlatformWithGemfileFlag", func(t *testing.T) {
		// Run platform with -gemfile flag
		args := []string{"-gemfile", gemfilePath}
		if err := RunPlatform(args); err != nil {
			t.Fatalf("RunPlatform failed: %v", err)
		}
	})
}

// mockDownloadManager implements DownloadManager for testing
type mockDownloadManager struct {
	cacheDir string
}

func (m *mockDownloadManager) CheckSourceHealth(ctx context.Context) {}
func (m *mockDownloadManager) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (DownloadReport, error) {
	return DownloadReport{Skipped: len(gems)}, nil
}
func (m *mockDownloadManager) CacheDir() string {
	return m.cacheDir
}
