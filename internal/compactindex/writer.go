package compactindex

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteVersionsFile writes a Bundler-compatible versions file.
//
// Format:
//
//	created_at: 2024-04-01T00:00:05Z
//	---
//	gemname version,version,... checksum
func WriteVersionsFile(path string, entries []VersionsEntry) error {
	// Create temp file in the same directory
	tempPath := path + ".tmp"
	tmpFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tempPath) // Clean up on error
	}()

	// Write header
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if _, err := fmt.Fprintf(tmpFile, "created_at: %s\n", timestamp); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write separator
	if _, err := fmt.Fprintln(tmpFile, "---"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	// Sort entries by name for consistency
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	// Write entries
	for _, entry := range entries {
		versionsStr := strings.Join(entry.Versions, ",")
		line := fmt.Sprintf("%s %s %s\n", entry.Name, versionsStr, entry.Checksum)

		if _, err := tmpFile.WriteString(line); err != nil {
			return fmt.Errorf("failed to write entry: %w", err)
		}
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// WriteInfoFile writes a Bundler-compatible info file.
//
// Format:
//
//	---
//	version[-platform] dep:constraint,dep:constraint|req:value,req:value
func WriteInfoFile(path string, versions []VersionInfo) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create temp file in the same directory
	tempPath := path + ".tmp"
	tmpFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tempPath) // Clean up on error
	}()

	// Write separator
	if _, err := fmt.Fprintln(tmpFile, "---"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	// Write version entries
	for _, v := range versions {
		line := formatVersionLine(v)
		if _, err := fmt.Fprintln(tmpFile, line); err != nil {
			return fmt.Errorf("failed to write version line: %w", err)
		}
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// formatVersionLine formats a VersionInfo into a compact index line.
func formatVersionLine(v VersionInfo) string {
	var parts []string

	// Version (with platform if present)
	versionStr := v.Version
	if v.Platform != "" {
		versionStr = fmt.Sprintf("%s-%s", v.Version, v.Platform)
	}
	parts = append(parts, versionStr)

	// Build dependencies and requirements sections
	var sections []string

	// Dependencies: name:constraint,name:constraint,...
	if len(v.Dependencies) > 0 {
		var deps []string
		for name, constraint := range v.Dependencies {
			// Replace commas with & for multiple constraints
			constraint = strings.ReplaceAll(constraint, ",", "&")
			deps = append(deps, fmt.Sprintf("%s:%s", name, constraint))
		}
		sort.Strings(deps) // Sort for consistency
		sections = append(sections, strings.Join(deps, ","))
	}

	// Requirements: key:value,key:value,...
	if len(v.Requirements) > 0 {
		var reqs []string
		for key, value := range v.Requirements {
			reqs = append(reqs, fmt.Sprintf("%s:%s", key, value))
		}
		sort.Strings(reqs) // Sort for consistency

		if len(sections) == 0 {
			// No dependencies, so add empty section before pipe
			sections = append(sections, "")
		}
		sections = append(sections, strings.Join(reqs, ","))
	}

	// Join sections
	if len(sections) > 0 {
		parts = append(parts, strings.Join(sections, "|"))
	}

	return strings.Join(parts, " ")
}

// ComputeInfoFileChecksum computes the MD5 checksum of an info file.
// This is used as the checksum in the versions file.
func ComputeInfoFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
