package config

import (
	"os"
	"runtime"
	"strconv"
)

const (
	defaultDownloadWorkersMin = 4
	defaultDownloadWorkersMax = 16
)

// DefaultDownloadWorkers returns the resolved worker count for downloads.
func DefaultDownloadWorkers(cfg *Config) int {
	workers, _ := ResolveDownloadWorkers(cfg)
	return workers
}

// ResolveDownloadWorkers returns the resolved worker count and its source.
func ResolveDownloadWorkers(cfg *Config) (int, string) {
	if value := os.Getenv("ORE_DOWNLOAD_WORKERS"); value != "" {
		if workers, ok := parseWorkerCount(value); ok {
			return workers, "env:ORE_DOWNLOAD_WORKERS"
		}
	}
	if value := os.Getenv("ORE_LIGHT_DOWNLOAD_WORKERS"); value != "" {
		if workers, ok := parseWorkerCount(value); ok {
			return workers, "env:ORE_LIGHT_DOWNLOAD_WORKERS"
		}
	}
	if cfg != nil && cfg.DownloadWorkers > 0 {
		return cfg.DownloadWorkers, "config:ore"
	}
	return adaptiveDownloadWorkers(), "default"
}

func adaptiveDownloadWorkers() int {
	cpus := runtime.GOMAXPROCS(0)
	if cpus < 1 {
		cpus = 1
	}
	workers := cpus * 2
	if workers < defaultDownloadWorkersMin {
		workers = defaultDownloadWorkersMin
	}
	if workers > defaultDownloadWorkersMax {
		workers = defaultDownloadWorkersMax
	}
	return workers
}

func parseWorkerCount(value string) (int, bool) {
	workers, err := strconv.Atoi(value)
	if err != nil || workers <= 0 {
		return 0, false
	}
	return workers, true
}
