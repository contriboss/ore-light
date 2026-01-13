package extensions

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type cargoManifest struct {
	Workspace struct {
		Members []string `toml:"members"`
	} `toml:"workspace"`
	Package map[string]interface{} `toml:"package"`
	Lib     struct {
		CrateType []string `toml:"crate-type"`
	} `toml:"lib"`
}

func workspaceCdylibManifests(manifestPath string) ([]string, bool) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, false
	}

	var manifest cargoManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, false
	}

	if len(manifest.Workspace.Members) == 0 || len(manifest.Package) > 0 {
		return nil, false
	}

	baseDir := filepath.Dir(manifestPath)
	var results []string
	for _, member := range manifest.Workspace.Members {
		memberPath := filepath.Join(baseDir, member, "Cargo.toml")
		if _, err := os.Stat(memberPath); err != nil {
			continue
		}
		if cargoHasCdylib(memberPath) {
			results = append(results, memberPath)
		}
	}

	if len(results) == 0 {
		return nil, false
	}
	return results, true
}

func cargoHasCdylib(manifestPath string) bool {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	var manifest cargoManifest
	if err := toml.Unmarshal(data, &manifest); err == nil {
		for _, crateType := range manifest.Lib.CrateType {
			if strings.EqualFold(crateType, "cdylib") {
				return true
			}
		}
	}

	// Fallback: simple substring match in case of partial parsing issues.
	return strings.Contains(string(data), "cdylib")
}
