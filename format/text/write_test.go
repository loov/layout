package text

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loov/layout"
)

func TestWrite(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("A", "B").Label = "yes"
	graph.Edge("A", "C")
	graph.Edge("B", "D")
	graph.Edge("C", "D")
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	t.Log("\n" + got)
	for _, want := range []string{"A", "B", "C", "D", "yes", "▼", "╭", "│"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}
