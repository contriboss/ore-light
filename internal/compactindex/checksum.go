package compactindex

import (
	"regexp"
	"strings"
)

// FindChecksum returns the checksum for a specific version+platform entry.
// The compact index stores checksums in the requirements map under "checksum".
func FindChecksum(infoList []VersionInfo, version, platform string) (string, bool) {
	normalized := normalizePlatform(platform)
	for i := range infoList {
		info := infoList[i]
		if info.Version != version {
			continue
		}
		if normalizePlatform(info.Platform) != normalized {
			continue
		}
		checksum := strings.TrimSpace(info.Requirements["checksum"])
		if checksum == "" {
			return "", false
		}
		return checksum, true
	}
	return "", false
}

func normalizePlatform(platform string) string {
	platform = strings.TrimSpace(strings.ToLower(platform))
	if platform == "" || platform == "ruby" {
		return ""
	}
	if matches := regexp.MustCompile(`^(.*-darwin)(?:-?\d+)?$`).FindStringSubmatch(platform); matches != nil {
		return matches[1]
	}
	return platform
}
