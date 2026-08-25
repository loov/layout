package layout_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func TestForce(t *testing.T) {
	graph := layout.NewGraph()
	// a 5-cycle plus a pendant node
	for _, e := range [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "E"}, {"E", "A"}, {"A", "F"}} {
		graph.Edge(e[0], e[1])
	}
	if err := layout.Force(graph); err != nil {
		t.Fatal(err)
	}
	for _, a := range graph.Nodes {
		if a.Center.X < a.Radius.X || a.Center.Y < a.Radius.Y {
			t.Errorf("%v is outside the drawing at %v", a, a.Center)
		}
		for _, b := range graph.Nodes {
			if a == b {
				continue
			}
			d := math.Hypot(float64(a.Center.X-b.Center.X), float64(a.Center.Y-b.Center.Y))
			if d < float64(a.Radius.X+b.Radius.X) {
				t.Errorf("%v and %v overlap, distance %.1f", a, b, d)
			}
		}
	}
	for _, edge := range graph.Edges {
		if len(edge.Path) != 2 {
			t.Errorf("%v has no path", edge)
		}
	}
	var got bytes.Buffer
	if err := svg.Write(&got, graph); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "testdata/force.svg", got.Bytes())
}
