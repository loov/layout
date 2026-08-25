package graphml

import (
	"bytes"
	"testing"

	"github.com/loov/layout"
)

func TestRoundTrip(t *testing.T) {
	graph := layout.NewDigraph()
	graph.ID = "g"
	graph.Node("A").Label = "Alpha & <Omega>"
	graph.Node("B").Shape = layout.Box
	graph.Edge("A", "B").Label = "a to b"
	graph.Edge("B", "C")

	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	graphs, err := Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(graphs) != 1 {
		t.Fatalf("expected one graph, got %d", len(graphs))
	}
	got := graphs[0]
	if got.ID != "g" || !got.Directed || len(got.Nodes) != 3 || len(got.Edges) != 2 {
		t.Fatalf("mismatch: %+v", got)
	}
	if got.Node("A").Label != "Alpha & <Omega>" || got.Node("B").Shape != layout.Box || got.Edges[0].Label != "a to b" {
		t.Errorf("attributes lost: %q %q %q", got.Node("A").Label, got.Node("B").Shape, got.Edges[0].Label)
	}
}
