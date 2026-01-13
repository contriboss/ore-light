package resolver

import "github.com/contriboss/gemfile-go/lockfile"

func buildLockfileChecksums(
	specs []lockfile.GemSpec,
	gitSpecs []lockfile.GitGemSpec,
	pathSpecs []lockfile.PathGemSpec,
	gemSources map[string]string,
	sources map[string]*RubyGemsSource,
	defaultSourceURL string,
) map[string][]lockfile.Checksum {
	checksums := make(map[string][]lockfile.Checksum)

	add := func(name, version, platform string, checksum *lockfile.Checksum, has bool) {
		lockName := name + "-" + version
		if platform != "" {
			lockName = name + "-" + version + "-" + platform
		}
		if has && checksum != nil {
			checksums[lockName] = []lockfile.Checksum{*checksum}
		} else {
			checksums[lockName] = []lockfile.Checksum{}
		}
	}

	for _, spec := range specs {
		srcURL := gemSources[spec.Name]
		src := sources[srcURL]
		if src == nil {
			src = sources[defaultSourceURL]
		}
		if src != nil {
			if checksum, ok := src.GetChecksum(spec.Name, spec.Version, spec.Platform); ok {
				add(spec.Name, spec.Version, spec.Platform, &checksum, true)
				continue
			}
		}
		add(spec.Name, spec.Version, spec.Platform, nil, false)
	}

	for _, spec := range gitSpecs {
		add(spec.Name, spec.Version, "", nil, false)
	}

	for _, spec := range pathSpecs {
		add(spec.Name, spec.Version, "", nil, false)
	}

	return checksums
}
