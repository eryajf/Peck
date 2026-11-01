package update

// Product update policy for the first in-app update release.
const (
	DefaultSource    = SourceGitHub
	DefaultAutoCheck = true
)

// The updater deliberately handles stable macOS application ZIP packages only.
// Automatic checks never download or replace the running application. A user
// action is required for both download and restart.
const (
	SupportedGOOS  = "darwin"
	SupportedAsset = ".zip"
)
