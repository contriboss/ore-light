//go:build mage

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Build compiles the ore binary into ./bin/ore.
func Build() error {
	fmt.Println("🔨 Building ore…")

	if err := os.MkdirAll("bin", 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	ldflags := buildLdflags()

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", filepath.Join("bin", "ore"), "./cmd/ore")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Test runs the Go test suite.
func Test() error {
	fmt.Println("🧪 Running tests…")

	cmd := exec.Command("go", "test", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Clean removes build artifacts.
func Clean() error {
	fmt.Println("🧹 Cleaning build artifacts…")

	if err := os.RemoveAll("bin"); err != nil {
		return fmt.Errorf("failed to remove bin directory: %w", err)
	}

	return nil
}

// Install copies the built ore binary into /usr/local/bin (or ORE_INSTALL_PREFIX).
func Install() error {
	fmt.Println("📦 Installing ore…")

	if err := Build(); err != nil {
		return err
	}

	prefix := os.Getenv("ORE_INSTALL_PREFIX")
	if prefix == "" {
		if home := os.Getenv("HOME"); home != "" {
			prefix = filepath.Join(home, ".local", "bin")
		} else {
			prefix = "/usr/local/bin"
		}
	}

	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return fmt.Errorf("failed to create install prefix %s: %w", prefix, err)
	}

	src := filepath.Join("bin", "ore")
	dst := filepath.Join(prefix, "ore")

	if err := copyFile(src, dst); err != nil {
		return err
	}

	if err := os.Chmod(dst, 0o755); err != nil {
		return fmt.Errorf("failed to mark %s executable: %w", dst, err)
	}

	fmt.Printf("✅ ore installed to %s\n", dst)
	return nil
}

// Fmt runs gofmt on all Go files
func Fmt() error {
	fmt.Println("Formatting code...")
	return sh.Run("go", "fmt", "./...")
}

// Vet runs go vet on all Go files
func Vet() error {
	fmt.Println("Vetting code...")
	return sh.Run("go", "vet", "./...")
}

// Bench runs benchmarks
func Bench() error {
	fmt.Println("Running benchmarks...")
	return sh.Run("go", "test", "-bench=.", "./...")
}

// Deps downloads dependencies
func Deps() error {
	fmt.Println("Downloading dependencies...")
	return sh.Run("go", "mod", "download")
}

// Tidy tidies go.mod
func Tidy() error {
	fmt.Println("Tidying go.mod...")
	return sh.Run("go", "mod", "tidy")
}

// CI runs all checks for continuous integration
func CI() error {
	mg.SerialDeps(Deps, Fmt, Vet, Test)
	fmt.Println("All CI checks passed!")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dst, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}

	return nil
}

func buildLdflags() string {
	version := buildVersion()
	commit := gitCommit()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	return fmt.Sprintf("-s -w -X main.version=%s -X main.buildCommit=%s -X main.buildTime=%s", version, commit, timestamp)
}

func buildVersion() string {
	data, err := os.ReadFile("VERSION")
	if err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	return "dev"
}

func gitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
