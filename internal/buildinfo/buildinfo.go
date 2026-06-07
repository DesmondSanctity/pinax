// Package buildinfo exposes ldflags-stamped build metadata so any package
// can read it without importing main.
package buildinfo

import (
	"fmt"
	"runtime/debug"
)

// Stamped at build time via -ldflags
// "-X pinax/internal/buildinfo.Version=... -X pinax/internal/buildinfo.Commit=... -X pinax/internal/buildinfo.Date=...".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Resolve returns the effective build info, falling back to the Go module
// build info embedded by `go install` when ldflags weren't set.
func Resolve() (version, commit, date string) {
	v, c, d := Version, Commit, Date
	if v != "dev" {
		return v, c, d
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return v, c, d
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		v = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				c = s.Value[:7]
			} else {
				c = s.Value
			}
		case "vcs.time":
			d = s.Value
		}
	}
	return v, c, d
}

// UserAgent returns the HTTP User-Agent string Pinax sends to docs sites.
// The Mozilla prefix is the standard "compatible bot" convention that gets
// through naive UA filters while still identifying the client honestly.
func UserAgent() string {
	v, _, _ := Resolve()
	return fmt.Sprintf("Mozilla/5.0 (compatible; Pinax/%s; +https://github.com/desmondsanctity/pinax)", v)
}
