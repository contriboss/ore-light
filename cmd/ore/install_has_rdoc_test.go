package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
)

func TestInstallAnsiDropsHasRdoc(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	vendorDir := filepath.Join(tmpDir, "vendor")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	spec := lockfile.GemSpec{
		Name:    "ansi",
		Version: "1.5.0",
	}

	metadataYAML := []byte(`---
name: ansi
version: !ruby/object:Gem::Version
  version: 1.5.0
summary: ANSI colorizer
has_rdoc: true
require_paths:
  - lib
`)

	files := map[string][]byte{
		"lib/ansi.rb": []byte("# ansi"),
	}

	gemPath := filepath.Join(cacheDir, spec.FullName()+".gem")
	if err := createFakeGemArchive(gemPath, files, metadataYAML); err != nil {
		t.Fatalf("failed to create fake gem: %v", err)
	}

	ctx := context.Background()
	extConfig := &extensions.BuildConfig{SkipExtensions: true}
	report, err := installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{spec}, false, false, extConfig)
	if err != nil {
		t.Fatalf("installFromCache failed: %v", err)
	}
	if report.Installed != 1 {
		t.Fatalf("expected 1 gem installed, got %d (skipped: %d)", report.Installed, report.Skipped)
	}

	specPath := filepath.Join(vendorDir, "specifications", spec.FullName()+".gemspec")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("expected gemspec to exist: %v", err)
	}
	if strings.Contains(string(data), "has_rdoc") {
		t.Fatalf("expected gemspec to omit has_rdoc, got: %s", data)
	}
}
