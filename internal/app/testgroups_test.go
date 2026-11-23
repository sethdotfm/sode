package app

import (
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// AGENTS.md, Test groups. CI selects the groups by these names and calls, so a wrong one makes a job miss a test.

func TestMain(m *testing.M) {
	config.UseTestCosts()
	code := m.Run()
	if simBuild.dir != "" {
		_ = os.RemoveAll(simBuild.dir)
	}
	os.Exit(code)
}

// Also for leak checks (AGENTS.md, Test groups).
func slowTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("slow test skipped under -short")
	}
}

// browserTest runs first in startBrowser and startSim, so -short skips a browser check before it builds anything.
func browserTest(t *testing.T) {
	t.Helper()
	if !strings.HasPrefix(t.Name(), "TestChrome") {
		t.Fatalf("%s starts a browser, so its name must start with TestChrome (CI selects browser checks by name)", t.Name())
	}
	if testing.Short() {
		t.Skip("browser check skipped under -short")
	}
}

// browserSkip skips a browser check that cannot run here, or fails it under SODE_REQUIRE_BROWSER
// (set in CI). So a job never passes after it ran no browser checks.
func browserSkip(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("SODE_REQUIRE_BROWSER") != "" {
		t.Fatal(why + " (SODE_REQUIRE_BROWSER is set)")
	}
	t.Skip(why)
}
