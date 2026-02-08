package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheck_RelativePathGem(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 1. Create a "gem" directory
	gemDir := filepath.Join(tmpDir, "libs", "my_gem")
	if err := os.MkdirAll(gemDir, 0755); err != nil {
		t.Fatal(err)
	}
	gemspecPath := filepath.Join(gemDir, "my_gem.gemspec")
	gemspecContent := `Gem::Specification.new do |s|
  s.name    = "my_gem"
  s.version = "0.1.0"
end`
	if err := os.WriteFile(gemspecPath, []byte(gemspecContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a subdirectory for the app
	appDir := filepath.Join(tmpDir, "app", "subdir")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 3. Create a lockfile in the subdirectory with a relative path to the gem
	// The path should be relative to the lockfile directory: ../../libs/my_gem
	lockfilePath := filepath.Join(appDir, "Gemfile.lock")
	lockfileContent := `PATH
  remote: ../../libs/my_gem
  specs:
    my_gem (0.1.0)

PLATFORMS
  ruby

DEPENDENCIES
  my_gem!
`
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Create a dummy vendor directory
	vendorDir := filepath.Join(appDir, "vendor")
	if err := os.MkdirAll(filepath.Join(vendorDir, "gems"), 0755); err != nil {
		t.Fatal(err)
	}

	// 5. Run check from the root directory (different from appDir)
	// ore check -lockfile app/subdir/Gemfile.lock -vendor app/subdir/vendor
	args := []string{"-lockfile", lockfilePath, "-vendor", vendorDir}

	// Capture output to verify the error message
	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	if err := RunCheck(args); err != nil {
		w.Close()
		os.Stdout = rescueStdout
		t.Fatalf("RunCheck failed for relative path gem: %v", err)
	}

	w.Close()
	os.Stdout = rescueStdout

	// 6. Test failure case: move the gem and verify it fails with the right message
	if err := os.Rename(gemDir, filepath.Join(tmpDir, "libs", "moved_gem")); err != nil {
		t.Fatal(err)
	}

	// Capture output again
	r, w, _ = os.Pipe()
	os.Stdout = w

	err := RunCheck(args)
	w.Close()
	os.Stdout = rescueStdout

	if err == nil {
		t.Fatal("RunCheck should have failed after moving the gem")
	}

	if !contains(err.Error(), "missing") {
		t.Errorf("Expected error to mention missing gems, got: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	expectedPart := "resolved to: " + filepath.Join(appDir, "../../libs/my_gem")
	if !strings.Contains(output, expectedPart) {
		t.Errorf("Expected output to contain %q, but got:\n%s", expectedPart, output)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
