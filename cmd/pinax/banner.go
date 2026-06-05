package main

import (
	"io"
	"os"
)

// ANSI 256-colour escapes. cyan-ish for the letters, amber for the tagline.
const (
	ansiReset = "\x1b[0m"
	ansiCyan1 = "\x1b[38;5;117m" // light sky
	ansiCyan2 = "\x1b[38;5;87m"  // brighter cyan
	ansiTeal  = "\x1b[38;5;79m"  // teal
	ansiAmber = "\x1b[38;5;215m" // warm amber
	ansiDim   = "\x1b[38;5;245m" // muted grey
)

var bannerLines = []string{
	` ____ ___ _   _    _    __  __`,
	`|  _ \_ _| \ | |  / \   \ \/ /`,
	`| |_) | ||  \| | / _ \   \  / `,
	`|  __/| || |\  |/ ___ \  /  \ `,
	`|_|  |___|_| \_/_/   \_\/_/\_\`,
}

var bannerColours = []string{ansiCyan1, ansiCyan1, ansiCyan2, ansiCyan2, ansiTeal}

func printBanner(w io.Writer) {
	colour := useColour(w)
	for i, line := range bannerLines {
		if colour {
			_, _ = io.WriteString(w, bannerColours[i]+line+ansiReset+"\n")
		} else {
			_, _ = io.WriteString(w, line+"\n")
		}
	}
	if colour {
		_, _ = io.WriteString(w, "   "+ansiAmber+"any docs site "+ansiDim+"→"+ansiAmber+" local MCP server"+ansiReset+"\n")
	} else {
		_, _ = io.WriteString(w, "   any docs site → local MCP server\n")
	}
}

// useColour reports whether ANSI escapes are safe to emit on w. Requires both
// a TTY and the absence of NO_COLOR (https://no-color.org).
func useColour(w io.Writer) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	return isTerminalWriter(w)
}

// isTerminalWriter reports whether w is an *os.File backed by a character
// device. Used to gate decorative output so it never lands in pipes, files,
// or MCP stdio traffic.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
