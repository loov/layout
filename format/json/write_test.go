package json

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/loov/layout"
)

func TestRoundTrip(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("A", "B").Label = "x"
	graph.Edge("A", "C")
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	var got Graph
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 3 || len(got.Edges) != 2 || got.Edges[0].LabelPos == nil || len(got.Edges[0].Path) < 2 || got.Width <= 0 {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestNegativePositions(t *testing.T) {
	graph := layout.NewDigraph()
	a, b := graph.Node("a"), graph.Node("b")
	a.Pos, b.Pos = &layout.Vector{X: -100, Y: -50}, &layout.Vector{X: 50, Y: 80}
	a.MinSize, b.MinSize = layout.Vector{X: 20, Y: 20}, layout.Vector{X: 20, Y: 20}
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	var got struct{ X, Y, Width, Height float64 }
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.X > -110 || got.Y > -60 || got.Width <= 0 || got.X+got.Width < 60 || got.Y+got.Height < 90 {
		t.Fatalf("drawing does not contain the nodes:\n%s", buf.String())
	}
}
