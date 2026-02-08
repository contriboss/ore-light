package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppraisalStyleGemfile(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a dummy gem structure
	// mygem/
	//   mygem.gemspec
	//   lib/
	//     mygem.rb
	//   gemfiles/
	//     appraisal.gemfile

	gemDir := filepath.Join(tmpDir, "mygem")
	if err := os.MkdirAll(filepath.Join(gemDir, "lib"), 0755); err != nil {
		t.Fatal(err)
	}

	gemspecContent := `
Gem::Specification.new do |s|
  s.name        = "mygem"
  s.version     = "0.1.0"
  s.summary     = "A test gem"
  s.authors     = ["Test"]
  s.files       = ["lib/mygem.rb"]
  s.add_runtime_dependency "rake", ">= 0"
  s.add_development_dependency "rspec", ">= 0"
end
`
	if err := os.WriteFile(filepath.Join(gemDir, "mygem.gemspec"), []byte(gemspecContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(gemDir, "lib", "mygem.rb"), []byte("module Mygem; end"), 0644); err != nil {
		t.Fatal(err)
	}

	gemfilesDir := filepath.Join(gemDir, "gemfiles")
	if err := os.MkdirAll(gemfilesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Create the appraisal gemfile referencing the gemspec with relative path
	gemfileContent := `
source "https://rubygems.org"
gemspec :path => "../", :development_group => :test
`
	gemfilePath := filepath.Join(gemfilesDir, "appraisal.gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Test loadOrGenerateLockfile with the appraisal gemfile
	lockfilePath := gemfilePath + ".lock"

	// We need to be in some directory, but loadOrGenerateLockfile should handle paths correctly
	// Actually, loadOrGenerateLockfile uses detectGemfileFromLock which checks for file existence

	t.Run("GenerateLockfile", func(t *testing.T) {
		parsed, err := loadOrGenerateLockfile(lockfilePath, false)
		if err != nil {
			t.Fatalf("loadOrGenerateLockfile failed: %v", err)
		}

		if parsed == nil {
			t.Fatal("Expected parsed lockfile, got nil")
		}

		// Check if rake (dependency from gemspec) is in the lockfile
		foundRake := false
		for _, spec := range parsed.GemSpecs {
			if spec.Name == "rake" {
				foundRake = true
				break
			}
		}

		if !foundRake {
			t.Errorf("Expected to find 'rake' in lockfile (from gemspec), but it was missing")
		}

		// Check if rspec (dev dependency from gemspec) is in the lockfile
		foundRspec := false
		for _, spec := range parsed.GemSpecs {
			if spec.Name == "rspec" {
				foundRspec = true
				break
			}
		}

		if !foundRspec {
			t.Errorf("Expected to find 'rspec' in lockfile (from gemspec dev dep), but it was missing")
		}
	})

	t.Run("VerifyLockfilePath", func(t *testing.T) {
		if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
			t.Errorf("Expected lockfile to be created at %s", lockfilePath)
		}
	})
}
