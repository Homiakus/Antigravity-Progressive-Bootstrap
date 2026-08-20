package registry

import "testing"

func TestPlanRemote(t *testing.T) {
	s := Server{Name: "x", Raw: map[string]any{"server": map[string]any{"remotes": []any{map[string]any{"url": "https://example.test/mcp", "type": "streamable-http"}}}}}
	p, err := Plan(s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Server.ServerURL != "https://example.test/mcp" {
		t.Fatalf("url=%q", p.Server.ServerURL)
	}
}
func TestPlanNPM(t *testing.T) {
	s := Server{Name: "x", Raw: map[string]any{"server": map[string]any{"packages": []any{map[string]any{"registryType": "npm", "identifier": "@example/mcp", "version": "1.2.3"}}}}}
	p, err := Plan(s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Server.Command == "" || len(p.Server.Args) == 0 {
		t.Fatalf("bad plan %+v", p)
	}
}
