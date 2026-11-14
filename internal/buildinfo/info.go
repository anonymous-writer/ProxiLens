// Package buildinfo exposes metadata embedded by CI and release builds.
package buildinfo

var (
	Version   = "development"
	Commit    = "unknown"
	BuildKind = "local"
)

// DisplayVersion returns a concise, human-readable installed build identifier.
func DisplayVersion() string {
	kind := BuildKind
	if kind == "" || kind == "local" {
		kind = "Local-Build"
	}
	version := Version
	if version == "" || version == "development" {
		version = "development"
	}
	commit := Commit
	if commit == "" {
		commit = "unknown"
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return kind + " " + version + " · " + commit
}
