package version

import (
	"runtime"
	"time"
)

// These variables are set at build time using ldflags
var (
	// Version is the semantic version (e.g., "1.0")
	Version = "dev"

	// BuildDate is the build date in YYYYMMDD format
	BuildDate = ""

	// BuildTime is the UTC timestamp of the build
	BuildTime = "unknown"

	// GitCommit is the short git commit hash
	GitCommit = "unknown"

	// Edition identifies this as Community Edition
	Edition = "Community"
)

// Info contains version information
type Info struct {
	Version   string `json:"version"`
	Edition   string `json:"edition"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// FullVersion returns version with build date (e.g., "1.0.20251215")
func FullVersion() string {
	if BuildDate != "" {
		return Version + "." + BuildDate
	}
	// Fallback: use current date if BuildDate not set
	return Version + "." + time.Now().Format("20060102")
}

// Get returns the current version information
func Get() Info {
	return Info{
		Version:   FullVersion(),
		Edition:   Edition,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a formatted version string
func String() string {
	return Version + " " + Edition + " Edition (" + GitCommit + ")"
}
