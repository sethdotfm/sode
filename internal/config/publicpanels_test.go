package config

import "testing"

func TestPanelsFieldsDefaultForAnOldFile(t *testing.T) {
	c, err := Parse([]byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !c.PublicPanels || c.NewPanelActs != "public" {
		t.Fatalf("publicPanels %v, newPanelActs %q; want on and public", c.PublicPanels, c.NewPanelActs)
	}
	c, err = Parse([]byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\npublicPanels: false\nnewPanelActs: session\n"))
	if err != nil || c.PublicPanels || c.NewPanelActs != "session" {
		t.Fatalf("stored values: %v %q %v", c.PublicPanels, c.NewPanelActs, err)
	}
}

func TestNewPanelActsIsAnAuthenticationMode(t *testing.T) {
	for _, m := range []string{"session", "api", "both", "public"} {
		c := Defaults()
		c.Normalize()
		c.NewPanelActs = m
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", m, err)
		}
	}
	c := Defaults()
	c.Normalize()
	c.NewPanelActs = "anyone"
	if c.Validate() == nil {
		t.Fatal("an unknown mode saved")
	}
	c.NewPanelActs = ""
	c.Normalize()
	if c.NewPanelActs != "public" {
		t.Fatalf("an empty mode normalizes to %q", c.NewPanelActs)
	}
}
