package buildinfo

import (
	"strings"
	"testing"
)

func TestUserAgentContainsVersionAndURL(t *testing.T) {
	got := UserAgent()
	if !strings.HasPrefix(got, "Mozilla/5.0 (compatible; Pinax/") {
		t.Errorf("UserAgent should start with the compatible-bot prefix, got %q", got)
	}
	if !strings.Contains(got, "+https://github.com/desmondsanctity/pinax") {
		t.Errorf("UserAgent should advertise the project URL, got %q", got)
	}
}

func TestUserAgentReflectsLdflagsVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "1.2.3"
	got := UserAgent()
	if !strings.Contains(got, "Pinax/1.2.3") {
		t.Errorf("UserAgent should embed Version, got %q", got)
	}
}

func TestResolveFallsBackToBuildInfo(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "dev"
	v, _, _ := Resolve()
	// In tests run via `go test` there's usually no module version, so the
	// fallback should still leave Version == "dev". Just assert it doesn't
	// panic and returns something.
	if v == "" {
		t.Error("Resolve returned empty version")
	}
}
