package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
)

// Stamped at build time via -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func printVersion(w io.Writer) {
	v, c, d := resolveVersion()
	fmt.Fprintf(w, "pinax %s (commit %s, built %s, %s/%s, %s)\n",
		v, c, d, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// resolveVersion prefers ldflags-stamped values and falls back to the Go
// build info embedded in the binary so `go install`-ed builds still show
// something useful.
func resolveVersion() (string, string, string) {
	v, c, d := version, commit, date
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
			if c == "none" {
				c = s.Value
			}
		case "vcs.time":
			if d == "unknown" {
				d = s.Value
			}
		}
	}
	return v, c, d
}
