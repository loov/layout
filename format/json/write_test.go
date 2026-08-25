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
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
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
