package extpoints

import "testing"

func TestHandleRawPreservesTrailingSlash(t *testing.T) {
	r := &RouterRegistry{}
	g := r.Group("/api/v1/nodes")

	if got := g.BasePath(); got != "/api/v1/nodes" {
		t.Fatalf("BasePath() = %q, want %q", got, "/api/v1/nodes")
	}
	slashless := g.Handle("GET", "")
	slashed := g.HandleRaw("GET", "/")

	if slashless.Path != "/api/v1/nodes" {
		t.Errorf("Handle(\"\") path = %q, want %q", slashless.Path, "/api/v1/nodes")
	}
	if slashed.Path != "/api/v1/nodes/" {
		t.Errorf("HandleRaw(\"/\") path = %q, want %q", slashed.Path, "/api/v1/nodes/")
	}
	if slashed.ID == slashless.ID {
		t.Error("HandleRaw must allocate its own route ID")
	}
	if got := len(r.Routes()); got != 2 {
		t.Errorf("registry routes = %d, want 2", got)
	}
}
