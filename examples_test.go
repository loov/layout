package layout_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/svg"
)

var update = flag.Bool("update", false, "update testdata golden files")

var examples = map[string]func() *layout.Graph{
	"minimal": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		return graph
	},
	"basic": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Node("A")
		graph.Node("B")
		graph.Node("C")
		graph.Node("D")
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		return graph
	},
	"weighted": func() *layout.Graph {
		// K2,2 must have one crossing; the heavy P->C edge should stay straight
		graph := layout.NewDigraph()
		graph.Edge("P", "B")
		graph.Edge("P", "C").Weight = 10
		graph.Edge("Q", "B")
		graph.Edge("Q", "C")
		return graph
	},
	"flat": func() *layout.Graph {
		// A, B, C on one rank with A->B adjacent and A->C arcing over B
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("R", "A")
		graph.Edge("R", "B")
		graph.Edge("R", "C")
		graph.SameRank = append(graph.SameRank, []*layout.Node{graph.Node("A"), graph.Node("B"), graph.Node("C")})
		return graph
	},
	"loop": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "A")
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "B")
		graph.Edge("C", "D")
		return graph
	},
	"multi": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.Edge("B", "C")
		graph.Edge("C", "B")
		graph.Edge("A", "D")
		graph.Edge("D", "C")
		return graph
	},
	"rankdir": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		a := graph.Node("A")
		a.Shape = layout.Box
		a.Label = "Left\nto\nright"
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		graph.Edge("D", "D")
		return graph
	},
	"minmax": func() *layout.Graph {
		// X is pinned to the top and Y to the bottom despite their edges
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("B", "C")
		graph.Edge("C", "D")
		graph.Edge("B", "X")
		graph.Edge("Y", "C")
		graph.MinRank = []*layout.Node{graph.Node("X")}
		graph.MaxRank = []*layout.Node{graph.Node("Y")}
		return graph
	},
	"components": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("X", "Y")
		graph.Edge("Y", "Z")
		graph.Edge("Z", "X")
		graph.Node("Lonely")
		return graph
	},
	"labels": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B").Label = "yes"
		graph.Edge("A", "C").Label = "no"
		graph.Edge("B", "D")
		graph.Edge("C", "D").Label = "long label here"
		graph.Edge("D", "A").Label = "back"
		return graph
	},
	"complex": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RowPadding = 30 * layout.Point

		a := graph.Node("A")
		a.Shape = layout.Box
		a.Label = "Lorem\nIpsum\nDolorem"
		a.FillColor = layout.RGB{0xFF, 0xA0, 0x20}

		b := graph.Node("B")
		b.Shape = layout.Ellipse
		b.Label = "Ignitus"
		b.FillColor = layout.HSL{0, 0.7, 0.7}

		c := graph.Node("C")
		c.Shape = layout.Square
		c.FontSize = 12 * layout.Point
		c.FontColor = layout.RGB{0x20, 0x20, 0x20}

		graph.Node("D")

		ab := graph.Edge("A", "B")
		ab.LineWidth = 4 * layout.Point
		ac := graph.Edge("A", "C")
		ac.LineWidth = 4 * layout.Point
		if col, ok := layout.ColorByName("blue"); ok {
			ac.LineColor = col
		}
		bd := graph.Edge("B", "D")
		bd.LineColor = layout.RGB{0xA0, 0xFF, 0xA0}
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		return graph
	},
}

// TestExamples renders each example and compares it to testdata/<name>.svg.
// Run `go test -update` to regenerate the golden files.
func TestExamples(t *testing.T) {
	for name, build := range examples {
		t.Run(name, func(t *testing.T) {
			checkGolden(t, filepath.Join("testdata", name+".svg"), build())
		})
	}
}

// TestGraphviz lays out the classic Graphviz directed examples in
// testdata/graphviz/*.gv and compares them to the .svg next to them.
func TestGraphviz(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".gv"), func(t *testing.T) {
			graphs, err := dot.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if len(graphs) != 1 {
				t.Fatalf("expected one graph, got %d", len(graphs))
			}
			checkGolden(t, strings.TrimSuffix(file, ".gv")+".svg", graphs[0])
		})
	}
}

func checkGolden(t *testing.T, path string, graph *layout.Graph) {
	t.Helper()
	layout.Hierarchical(graph)

	var got bytes.Buffer
	if err := svg.Write(&got, graph); err != nil {
		t.Fatal(err)
	}

	if *update {
		if err := os.WriteFile(path, got.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("%s differs from golden file; run `go test -update`", path)
	}
}
