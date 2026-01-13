package resolver

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type rubyVersionInfo struct {
	Version  string
	String   string
	Major    int
	Minor    int
	Tiny     int
	Patch    int
	hasMajor bool
	hasMinor bool
	hasTiny  bool
	hasPatch bool
}

func (i rubyVersionInfo) computed() string {
	if !i.hasMajor || !i.hasMinor {
		return ""
	}
	parts := []string{strconv.Itoa(i.Major), strconv.Itoa(i.Minor)}
	if i.hasTiny {
		parts = append(parts, strconv.Itoa(i.Tiny))
	} else if i.hasPatch {
		parts = append(parts, strconv.Itoa(i.Patch))
	}
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts, ".")
}

var (
	versionConstRe = regexp.MustCompile(`(?m)^\s*(VERSION|STRING)\s*=\s*['"]([^'"]+)['"]`)
	numericConstRe = regexp.MustCompile(`(?m)^\s*(MAJOR|MINOR|TINY|PATCH)\s*=\s*(\d+)`)
)

func parseRubyVersionInfo(content string) rubyVersionInfo {
	info := rubyVersionInfo{}
	for _, match := range versionConstRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 3 {
			continue
		}
		switch match[1] {
		case "VERSION":
			if info.Version == "" {
				info.Version = match[2]
			}
		case "STRING":
			if info.String == "" {
				info.String = match[2]
			}
		}
	}

	for _, match := range numericConstRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 3 {
			continue
		}
		value, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		switch match[1] {
		case "MAJOR":
			info.Major = value
			info.hasMajor = true
		case "MINOR":
			info.Minor = value
			info.hasMinor = true
		case "TINY":
			info.Tiny = value
			info.hasTiny = true
		case "PATCH":
			info.Patch = value
			info.hasPatch = true
		}
	}

	return info
}

func resolveGemspecVersion(versionExpr, rootDir string) string {
	if versionExpr == "" {
		return ""
	}
	if strings.Contains(versionExpr, "::") {
		if resolved := resolveVersionFromFiles(rootDir, versionExpr); resolved != "" {
			return resolved
		}
		return versionExpr
	}
	if _, err := NewSemverVersion(versionExpr); err == nil {
		return versionExpr
	}
	if resolved := resolveVersionFromFiles(rootDir, versionExpr); resolved != "" {
		return resolved
	}
	return versionExpr
}

func resolveVersionFromFiles(rootDir, versionExpr string) string {
	files := findVersionFiles(rootDir)
	if len(files) == 0 {
		return ""
	}

	preferString := strings.HasSuffix(versionExpr, "::STRING")
	preferVersion := strings.HasSuffix(versionExpr, "::VERSION") && !preferString

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		info := parseRubyVersionInfo(string(content))
		if preferString && info.String != "" {
			return info.String
		}
		if preferVersion && info.Version != "" {
			return info.Version
		}
		if info.Version != "" {
			return info.Version
		}
		if info.String != "" {
			return info.String
		}
		if computed := info.computed(); computed != "" {
			return computed
		}
	}

	return ""
}

func findVersionFiles(rootDir string) []string {
	var files []string
	libDir := filepath.Join(rootDir, "lib")
	_ = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "version.rb" || d.Name() == "gem_version.rb" {
			files = append(files, path)
		}
		return nil
	})
	return files
}
