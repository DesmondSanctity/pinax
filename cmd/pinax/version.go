package main

import (
	"fmt"
	"io"
	"runtime"

	"pinax/internal/buildinfo"
)

func printVersion(w io.Writer) {
	v, c, d := buildinfo.Resolve()
	fmt.Fprintf(w, "pinax %s (commit %s, built %s, %s/%s, %s)\n",
		v, c, d, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
