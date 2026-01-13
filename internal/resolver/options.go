package resolver

import "sync/atomic"

var allowPrereleaseVersions atomic.Bool

// SetAllowPrereleases controls whether prerelease versions are considered.
// Defaults to false to match Bundler's behavior.
func SetAllowPrereleases(allow bool) {
	allowPrereleaseVersions.Store(allow)
}

func allowPrereleases() bool {
	return allowPrereleaseVersions.Load()
}
