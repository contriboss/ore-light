package resolver

import (
	"regexp"
	"strings"
)

var darwinPlatformRegex = regexp.MustCompile(`^(.*-darwin)(?:-?\d+)?$`)
var linuxLibcRegex = regexp.MustCompile(`^(.*-linux)-(gnu|musl)$`)

func normalizePlatformForIndex(platform string) string {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	if normalized == "" || normalized == "ruby" {
		return ""
	}

	if matches := darwinPlatformRegex.FindStringSubmatch(normalized); matches != nil {
		normalized = matches[1]
	}

	if matches := linuxLibcRegex.FindStringSubmatch(normalized); matches != nil {
		normalized = matches[1]
	}

	return normalized
}
