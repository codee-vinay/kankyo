package version

// These values are overridden at build time with -ldflags -X.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
