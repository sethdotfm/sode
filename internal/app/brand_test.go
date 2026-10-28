package app

import "testing"

// Branding is display only (#126): saving it keeps the router state.
func TestBrandNotRouterSide(t *testing.T) {
	a := pinApp(t)
	st := a.router
	a.status.Err = "marker"
	cfg := a.r().cfg
	cfg.BrandText = "Studio B"
	cfg.BrandBorder = "teal"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.router != st || a.status.Err != "marker" {
		t.Fatal("brand change reset the router state")
	}
	if c := a.r().cfg; c.BrandText != "Studio B" || c.BrandBorder != "teal" {
		t.Fatalf("not applied: %q %q", c.BrandText, c.BrandBorder)
	}
	cfg.BrandBorder = "red"
	if err := a.ApplyConfig(cfg); err == nil {
		t.Fatal("red accepted")
	}
}
