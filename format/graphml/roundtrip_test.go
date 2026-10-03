package graphml

import (
	"bytes"
	"strings"
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

func TestSchemaLocation(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, layout.NewGraph()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), ` xsi:schemaLocation="`) {
		t.Errorf("missing xsi:schemaLocation:\n%s", &buf)
	}
}

func TestArrowsSetEdgeDirection(t *testing.T) {
	graph := layout.NewGraph()
	forward := graph.Edge("a", "b") // dot's dir=forward in an undirected graph
	forward.ArrowHead, forward.ArrowTail = layout.ArrowNormal, layout.ArrowNone
	graph.Edge("b", "c")
	digraph := layout.NewDigraph()
	none := digraph.Edge("a", "b") // dir=none in a directed graph
	none.ArrowHead, none.ArrowTail = layout.ArrowNone, layout.ArrowNone
	digraph.Edge("b", "c")

	var buf bytes.Buffer
	if err := Write(&buf, graph, digraph); err != nil {
		t.Fatal(err)
	}
	graphs, err := Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range [][2]bool{{true, false}, {false, true}} {
		edges := graphs[i].Edges
		if edges[0].Directed != want[0] || edges[1].Directed != want[1] {
			t.Errorf("graph %d: edges directed %v, %v, want %v", i, edges[0].Directed, edges[1].Directed, want)
		}
	}
}
