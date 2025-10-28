package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

// Stats represents cache statistics
type Stats struct {
	Files     int
	TotalSize int64
}

// CollectStats walks the cache directory and collects statistics
func CollectStats(cacheDir string) (Stats, error) {
	var stats Stats

	err := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		stats.Files++
		stats.TotalSize += info.Size()
		return nil
	})

	if os.IsNotExist(err) {
		return stats, nil
	}

	return stats, err
}

// HumanBytes converts bytes to human-readable format (KiB, MiB, GiB, etc)
func HumanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
