package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/geminstall"
)

// NewBuildEnv returns a function that builds the execution environment
func NewBuildEnv(cfg *Config) func(vendorDir string, specs []lockfile.GemSpec) ([]string, error) {
	return func(vendorDir string, specs []lockfile.GemSpec) ([]string, error) {
		if err := geminstall.EnsureDir(vendorDir); err != nil {
			return nil, err
		}

		libPaths := collectLibraryPaths(vendorDir, specs)
		if len(libPaths) == 0 {
			return nil, fmt.Errorf("no gem libraries found under %s; run `ore install` first", vendorDir)
		}

		env := os.Environ()

		systemGemDir := getSystemGemDir()
		if vendorDir != systemGemDir {
			env = setEnv(env, "GEM_HOME", vendorDir)
			env = setEnv(env, "GEM_PATH", vendorDir)
			env = setEnv(env, "BUNDLE_GEMFILE", "")
		}

		env = prependPath(env, filepath.Join(vendorDir, "bin"))
		env = prependRubyLib(env, libPaths)

		return env, nil
	}
}

func collectLibraryPaths(vendorDir string, specs []lockfile.GemSpec) []string {
	seen := make(map[string]struct{})
	var libs []string

	for _, spec := range specs {
		libDir := filepath.Join(vendorDir, "gems", spec.FullName(), "lib")
		if _, err := os.Stat(libDir); err != nil {
			continue
		}
		if _, ok := seen[libDir]; ok {
			continue
		}
		seen[libDir] = struct{}{}
		libs = append(libs, libDir)
	}

	return libs
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func prependPath(env []string, path string) []string {
	if path == "" {
		return env
	}
	current, _ := getEnvValue(env, "PATH")
	if current == "" {
		return setEnv(env, "PATH", path)
	}
	return setEnv(env, "PATH", fmt.Sprintf("%s%c%s", path, os.PathListSeparator, current))
}

func prependRubyLib(env []string, libs []string) []string {
	if len(libs) == 0 {
		return env
	}
	libValue := strings.Join(libs, string(os.PathListSeparator))
	if current, _ := getEnvValue(env, "RUBYLIB"); current != "" {
		libValue = libValue + string(os.PathListSeparator) + current
	}
	return setEnv(env, "RUBYLIB", libValue)
}

func getEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	if value, ok := os.LookupEnv(key); ok {
		return value, true
	}
	return "", false
}
