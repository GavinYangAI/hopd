package config

import "testing"

func baseCfg(t *testing.T) *Config {
	t.Helper()
	c, err := Parse([]byte(`
groups:
  prod:
    - {name: a, local: "1", remote: h:1, via: x}
    - {name: b, local: "2", remote: h:2, via: x}
`))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAddTunnel(t *testing.T) {
	c := baseCfg(t)
	if err := c.AddTunnel(Tunnel{Name: "c", Group: "prod", Local: "3", Remote: "h:3", Via: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Tunnel("c"); !ok {
		t.Fatal("c not added")
	}
	if len(c.Tunnels()) != 3 {
		t.Fatalf("want 3 tunnels, got %d", len(c.Tunnels()))
	}
	if err := c.AddTunnel(Tunnel{Name: "a", Group: "prod", Local: "9", Remote: "h:9", Via: "x"}); err == nil {
		t.Fatal("adding duplicate name should error")
	}
}

func TestUpdateTunnel(t *testing.T) {
	c := baseCfg(t)
	if err := c.UpdateTunnel("b", Tunnel{Name: "b", Group: "prod", Local: "2", Remote: "h:22", Via: "x"}); err != nil {
		t.Fatal(err)
	}
	got, _ := c.Tunnel("b")
	if got.Remote != "h:22" {
		t.Fatalf("remote not updated: %+v", got)
	}
	if err := c.UpdateTunnel("a", Tunnel{Name: "a2", Group: "prod", Local: "1", Remote: "h:1", Via: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Tunnel("a"); ok {
		t.Fatal("old name a should be gone")
	}
	if _, ok := c.Tunnel("a2"); !ok {
		t.Fatal("new name a2 should exist")
	}
	if err := c.UpdateTunnel("a2", Tunnel{Name: "b", Group: "prod", Local: "1", Remote: "h:1", Via: "x"}); err == nil {
		t.Fatal("renaming onto existing name should error")
	}
	if err := c.UpdateTunnel("nope", Tunnel{Name: "nope", Group: "prod", Local: "1", Remote: "h:1", Via: "x"}); err == nil {
		t.Fatal("updating missing tunnel should error")
	}
}

func TestRemoveTunnel(t *testing.T) {
	c := baseCfg(t)
	if err := c.RemoveTunnel("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Tunnel("a"); ok {
		t.Fatal("a should be removed")
	}
	if len(c.Tunnels()) != 1 {
		t.Fatalf("want 1 tunnel, got %d", len(c.Tunnels()))
	}
	got, ok := c.Tunnel("b")
	if !ok || got.Remote != "h:2" {
		t.Fatalf("survivor b wrong: %+v ok=%v", got, ok)
	}
	if err := c.RemoveTunnel("nope"); err == nil {
		t.Fatal("removing missing tunnel should error")
	}
}
