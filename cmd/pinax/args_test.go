package main

import (
	"flag"
	"reflect"
	"testing"
)

// Real-world bug: `pinax serve docs-convex-dev --http --port 8421` left
// useHTTP=false because Go's flag.Parse stops at the first non-flag token.
// reorderArgs moves positional args after flags, while correctly skipping the
// "value" step for boolean flags.

func newServeFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Bool("http", false, "")
	fs.Int("port", 8080, "")
	return fs
}

func TestReorderArgs_FlagsAfterPositional(t *testing.T) {
	fs := newServeFlagSet()
	got := reorderArgs(fs, []string{"my-server", "--http", "--port", "9000"})
	want := []string{"--http", "--port", "9000", "my-server"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderArgs: got %v, want %v", got, want)
	}
	if err := fs.Parse(got); err != nil {
		t.Fatal(err)
	}
	if !fs.Lookup("http").Value.(flag.Getter).Get().(bool) {
		t.Error("--http should be true")
	}
	if fs.Lookup("port").Value.(flag.Getter).Get().(int) != 9000 {
		t.Error("--port should be 9000")
	}
	if fs.NArg() != 1 || fs.Arg(0) != "my-server" {
		t.Errorf("positional lost: %v", fs.Args())
	}
}

func TestReorderArgs_BoolFlagDoesNotEatPositional(t *testing.T) {
	fs := newServeFlagSet()
	// --http (bool) immediately followed by a positional must NOT consume it.
	got := reorderArgs(fs, []string{"--http", "my-server"})
	want := []string{"--http", "my-server"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderArgs_EqualsForm(t *testing.T) {
	fs := newServeFlagSet()
	got := reorderArgs(fs, []string{"my-server", "--port=4242", "--http"})
	if err := fs.Parse(got); err != nil {
		t.Fatal(err)
	}
	if fs.Lookup("port").Value.(flag.Getter).Get().(int) != 4242 {
		t.Errorf("--port=4242 not parsed; got=%v", fs.Lookup("port").Value)
	}
	if !fs.Lookup("http").Value.(flag.Getter).Get().(bool) {
		t.Error("--http should be true")
	}
}

func TestReorderArgs_DoubleDashTerminator(t *testing.T) {
	fs := newServeFlagSet()
	got := reorderArgs(fs, []string{"--http", "--", "--not-a-flag", "name"})
	want := []string{"--http", "--not-a-flag", "name"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderArgs_AllFlagsFirst(t *testing.T) {
	fs := newServeFlagSet()
	in := []string{"--http", "--port", "9000", "my-server"}
	got := reorderArgs(fs, in)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("already-ordered args should pass through: got %v want %v", got, in)
	}
}
