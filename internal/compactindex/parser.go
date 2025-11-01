package compactindex

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// VersionsEntry represents a single line in the versions file.
type VersionsEntry struct {
	Name     string   // Gem name
	Versions []string // List of versions (may include yanked with - prefix)
	Checksum string   // MD5 checksum of the info file
}

// VersionInfo represents a single version entry in an info file.
type VersionInfo struct {
	Version      string            // Version number
	Platform     string            // Platform (empty for all platforms)
	Dependencies map[string]string // name -> constraint
	Requirements map[string]string // key -> value (checksum, ruby, rubygems)
}

// ParseVersionsFile parses a Bundler compact index versions file.
//
// Format:
//
//	created_at: 2024-04-01T00:00:05Z
//	---
//	gemname [-]version[,version,...] checksum
//
// Returns a slice of VersionsEntry structs.
func ParseVersionsFile(path string) ([]VersionsEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open versions file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var entries []VersionsEntry
	scanner := bufio.NewScanner(file)
	headerPassed := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Wait for separator
		if !headerPassed {
			if strings.HasPrefix(line, "---") {
				headerPassed = true
			}
			continue
		}

		// Parse entry: name versions checksum
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue // Skip malformed lines
		}

		name := parts[0]
		versionsStr := parts[1]
		checksum := parts[2]

		// Split versions by comma
		versions := strings.Split(versionsStr, ",")

		entries = append(entries, VersionsEntry{
			Name:     name,
			Versions: versions,
			Checksum: checksum,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading versions file: %w", err)
	}

	return entries, nil
}

// ParseInfoFile parses a Bundler compact index info file.
//
// Format:
//
//	---
//	version[-platform] dep:constraint,dep:constraint|req:value,req:value
//
// Returns a slice of VersionInfo structs.
func ParseInfoFile(path string) ([]VersionInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open info file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var versions []VersionInfo
	scanner := bufio.NewScanner(file)
	headerPassed := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Wait for separator
		if !headerPassed {
			if strings.HasPrefix(line, "---") {
				headerPassed = true
			}
			continue
		}

		// Parse version line
		versionInfo, err := parseVersionLine(line)
		if err != nil {
			// Skip malformed lines
			continue
		}

		versions = append(versions, versionInfo)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading info file: %w", err)
	}

	return versions, nil
}

// parseVersionLine parses a single version line from an info file.
//
// Format: version[-platform] [dep:constraint,...]|[req:value,...]
func parseVersionLine(line string) (VersionInfo, error) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 1 {
		return VersionInfo{}, fmt.Errorf("invalid version line: %q", line)
	}

	// Parse version and platform
	versionPlatform := parts[0]
	version := versionPlatform
	platform := ""

	if dashIdx := strings.Index(versionPlatform, "-"); dashIdx != -1 {
		version = versionPlatform[:dashIdx]
		platform = versionPlatform[dashIdx+1:]
	}

	info := VersionInfo{
		Version:      version,
		Platform:     platform,
		Dependencies: make(map[string]string),
		Requirements: make(map[string]string),
	}

	// Parse dependencies and requirements (if present)
	if len(parts) == 2 {
		depsAndReqs := parts[1]

		// Split by pipe: deps|reqs
		sections := strings.Split(depsAndReqs, "|")

		// Parse dependencies (before pipe)
		if len(sections) > 0 && sections[0] != "" {
			parseDependencies(sections[0], info.Dependencies)
		}

		// Parse requirements (after pipe)
		if len(sections) > 1 {
			parseRequirements(sections[1], info.Requirements)
		}
	}

	return info, nil
}

// parseDependencies parses the dependencies section.
//
// Format: name:constraint,name:constraint,...
func parseDependencies(depsStr string, deps map[string]string) {
	if depsStr == "" {
		return
	}

	depPairs := strings.Split(depsStr, ",")
	for _, pair := range depPairs {
		// Split by first colon
		colonIdx := strings.Index(pair, ":")
		if colonIdx == -1 {
			continue
		}

		name := pair[:colonIdx]
		constraint := pair[colonIdx+1:]

		// Handle multiple constraints separated by &
		deps[name] = strings.ReplaceAll(constraint, "&", ",")
	}
}

// parseRequirements parses the requirements section.
//
// Format: key:value,key:value,...
func parseRequirements(reqsStr string, reqs map[string]string) {
	if reqsStr == "" {
		return
	}

	reqPairs := strings.Split(reqsStr, ",")
	for _, pair := range reqPairs {
		// Split by first colon
		colonIdx := strings.Index(pair, ":")
		if colonIdx == -1 {
			continue
		}

		key := pair[:colonIdx]
		value := pair[colonIdx+1:]
		reqs[key] = value
	}
}
