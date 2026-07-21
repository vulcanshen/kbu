// Package version holds the km8 build version, injected at build time via
// goreleaser ldflags (-X github.com/vulcanshen/kbu/internal/version.Version=...).
// Local `go build` produces a "dev" build that surfaces in --version output
// and the splash screen tagline.
package version

// Version is the build version. Set to the goreleaser tag minus its "v"
// prefix (e.g. "1.0.9") for tagged releases, "dev" otherwise.
var Version = "dev"

// Display returns a human-friendly version string for UI display:
// "dev" for local builds, "v1.0.9" for tagged releases.
func Display() string {
	if Version == "dev" {
		return "dev"
	}
	return "v" + Version
}
