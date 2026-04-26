package version

var (
	// Version is set via ldflags at build time (e.g. -X noto/internal/version.Version=v1.2.3).
	Version = "dev"
	// Commit is set via ldflags at build time.
	Commit = "none"
	// Date is set via ldflags at build time (UTC RFC3339 recommended).
	Date = "unknown"
)

// String returns the user-facing version string.
func String() string {
	return Version
}
