package version

// Version is set at build time via ldflags:
//
//	go build -ldflags="-X silobang/internal/version.Version=X.Y.Z"
//
// When unset (dev builds), defaults to "dev".
var Version = "dev"
