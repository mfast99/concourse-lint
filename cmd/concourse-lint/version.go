package main

import "runtime/debug"

// version is set by release builds:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/concourse-lint
//
// TestVersionFromLdflags fails if this variable is renamed, because -X
// silently ignores a name that does not exist.
var version = ""

// currentVersion is the version from -ldflags, else the version Go stores in
// the binary, else "dev". Go stores the module version for
// "go install ...@v0.1.0", and since Go 1.24 a pseudo-version with the commit
// for a local build, e.g. "v0.0.0-20261125182536-8b01401a7461+dirty".
func currentVersion() string {
	if version != "" {
		return version
	}
	buildInfo, available := debug.ReadBuildInfo()
	if available && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		return buildInfo.Main.Version
	}
	return "dev"
}
