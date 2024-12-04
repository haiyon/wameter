package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of Wameter
	Version = "unknown"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildDate is the build date
	BuildDate = "unknown"

	// GoVersion is the golang version
	GoVersion = runtime.Version()

	// Platform is the running platform
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// Info represents version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetInfo returns version information
func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  Platform,
	}
}

// String returns a string representation of version information
func (i Info) String() string {
	return fmt.Sprintf("Version: %s\nGit Commit: %s\nBuild Date: %s\nGo Version: %s\nPlatform: %s",
		i.Version, i.GitCommit, i.BuildDate, i.GoVersion, i.Platform)
}
