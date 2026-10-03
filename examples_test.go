package layout_test

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
)

var update = flag.Bool("update", false, "update testdata golden files")

var examples = map[string]func() *layout.Graph{
	// readme is the example in README.md; keep the two in sync
	"readme": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("checkout", "build")
		graph.Edge("checkout", "lint")
		graph.Edge("build", "unit")
		graph.Edge("build", "integration")
		graph.Edge("lint", "review").LineStyle = layout.Dashed
		graph.Edge("unit", "review")
		graph.Edge("integration", "review").Label = "slow"

		approve := graph.Edge("review", "deploy")
		approve.Label = "approve"
		approve.LineColor = layout.RGB{G: 0x80}
		graph.Node("review").Shape = layout.Box
		graph.Node("deploy").FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}

		graph.Clusters = []*layout.Cluster{{
			ID: "test", Label: "test",
			Nodes:     []*layout.Node{graph.Node("unit"), graph.Node("integration")},
			LineColor: layout.RGB{B: 0xFF},
		}}
		return graph
	},
	// regex is a DFA for -?[0-9]+(\.[0-9]+)?, drawn left to right like
	// automata usually are: a start arrow, edge labels on every edge,
	// self-loops and accepting states
	"regex": func() *layout.Graph {
		graph := regexDFA()
		graph.RankDir = layout.LeftToRight
		return graph
	},
	"regex_tb": regexDFA,
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
	"arrows": func() *layout.Graph {
		graph := layout.NewDigraph()
		both := graph.Edge("A", "B")
		both.ArrowHead, both.ArrowTail = layout.ArrowNormal, layout.ArrowNormal
		graph.Edge("A", "C").ArrowHead = layout.ArrowDot
		graph.Edge("A", "D").ArrowHead = layout.ArrowODot
		graph.Edge("B", "E").ArrowHead = layout.ArrowVee
		graph.Edge("C", "E").ArrowHead = layout.ArrowNone
		ports := graph.Edge("D", "E")
		ports.FromPort, ports.ToPort = layout.West, layout.East
		return graph
	},
	"cluster": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("start", "a0")
		graph.Edge("start", "b0")
		graph.Edge("a0", "a1")
		graph.Edge("a1", "a2")
		graph.Edge("b0", "b1")
		graph.Edge("a1", "b1")
		graph.Edge("a2", "end")
		graph.Edge("b1", "end")
		graph.Edge("start", "end")
		graph.Clusters = []*layout.Cluster{
			{ID: "cluster_a", Label: "A side", Nodes: []*layout.Node{graph.Node("a0"), graph.Node("a1"), graph.Node("a2")}, FillColor: layout.RGB{0xEE, 0xEE, 0xFF}},
			{ID: "cluster_b", Label: "B side", Nodes: []*layout.Node{graph.Node("b0"), graph.Node("b1")}, LineColor: layout.RGB{0, 0, 0xFF}},
		}
		return graph
	},
	"record": func() *layout.Graph {
		graph := layout.NewDigraph()
		a := graph.Node("A")
		a.Shape = layout.Record
		a.Label = "<f0> left|<f1> middle|{top|bottom}"
		b := graph.Node("B")
		b.Shape = layout.Record
		b.Label = "{name|type|value}"
		graph.Edge("A", "B")
		graph.Edge("A", "C")
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

// regexDFA returns the automaton of the regex examples
func regexDFA() *layout.Graph {
	graph := layout.NewDigraph()
	graph.Node("start").Shape = layout.Dot
	for _, id := range []string{"s0", "s1", "s2", "s3", "s4"} {
		graph.Node(id).Shape = layout.Circle
	}
	for _, id := range []string{"s2", "s4"} {
		accept := graph.Node(id)
		accept.FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}
		accept.Peripheries = 2
	}
	graph.Edge("start", "s0")
	graph.Edge("s0", "s1").Label = "-"
	graph.Edge("s0", "s2").Label = "0-9"
	graph.Edge("s1", "s2").Label = "0-9"
	graph.Edge("s2", "s2").Label = "0-9"
	graph.Edge("s2", "s3").Label = "."
	graph.Edge("s3", "s4").Label = "0-9"
	graph.Edge("s4", "s4").Label = "0-9"
	return graph
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

// TestExamplesText renders each example with ortho edges as text and
// compares it to testdata/<name>.txt, and colored to testdata/<name>.ans
// with the basic colors and testdata/<name>.truecolor.ans with 24-bit
// colors, when the colors make a difference.
func TestExamplesText(t *testing.T) {
	for name, build := range examples {
		t.Run(name, func(t *testing.T) {
			graph := build()
			text.Prepare(graph)
			if err := layout.Hierarchical(graph); err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := text.Write(&got, graph); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, filepath.Join("testdata", name+".txt"), got.Bytes())

			plain := bytes.Clone(got.Bytes())
			for suffix, palette := range map[string]text.Palette{".ans": text.ANSI16, ".truecolor.ans": text.TrueColor} {
				got.Reset()
				if err := text.WriteColor(&got, graph, text.Options{Palette: palette}); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join("testdata", name+suffix)
				if !bytes.Equal(got.Bytes(), plain) {
					compareGolden(t, path, got.Bytes())
				} else if *update {
					_ = os.Remove(path)
				} else if _, err := os.Stat(path); err == nil {
					t.Errorf("%s is the same as the text output; run `go test -update`", path)
				}
			}
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

// TestWriteDot checks the dot writer against testdata/minimal.dot.
func TestWriteDot(t *testing.T) {
	graph := examples["minimal"]()
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := dot.Write(&got, graph); err != nil {
		t.Fatal(err)
	}
	// round trip: the output must parse again with the same nodes and edges
	parsed, err := dot.Parse(bytes.NewReader(got.Bytes()))
	if err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, got.Bytes())
	}
	if len(parsed) != 1 || len(parsed[0].Nodes) != len(graph.Nodes) || len(parsed[0].Edges) != len(graph.Edges) {
		t.Fatalf("round trip mismatch\n%s", got.Bytes())
	}
	compareGolden(t, filepath.Join("testdata", "minimal.dot"), got.Bytes())
}

func checkGolden(t *testing.T, path string, graph *layout.Graph) {
	t.Helper()
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	if err := svg.Write(&got, graph); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, path, got.Bytes())
}

// compareGolden compares got with the file at path, rewriting it with -update
func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden file; run `go test -update`\n%s", path, diffLines(want, got))
	}
}

func TestHierarchicalErrors(t *testing.T) {
	graph := layout.NewDigraph()
	stray := layout.NewNode("stray")
	graph.AddEdge(layout.NewEdge(graph.Node("A"), stray))
	if err := layout.Hierarchical(graph); err == nil {
		t.Error("expected an error for an edge to a node outside the graph")
	}
	graph = layout.NewDigraph()
	graph.AddEdge(&layout.Edge{From: graph.Node("A")})
	if err := layout.Hierarchical(graph); err == nil {
		t.Error("expected an error for a nil endpoint")
	}
}

func TestHierarchicalWith(t *testing.T) {
	// more iterations and no balancing must still give a valid layout
	graph := examples["complex"]()
	if err := layout.HierarchicalWith(graph, layout.Options{OrderIterations: 100, NoRankBalance: true}); err != nil {
		t.Fatal(err)
	}
	for _, edge := range graph.Edges {
		if len(edge.Path) < 2 {
			t.Errorf("edge %v has no path", edge)
		}
	}
}

// TestPinned round-trips a layout through dot: positions written by
// dot.Write are read back as pos and kept by Hierarchical.
func TestPinned(t *testing.T) {
	graphs, err := dot.ParseFile(filepath.Join("testdata", "graphviz", "fsm.gv"))
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := dot.Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	parsed, err := dot.Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	pinned := parsed[0]
	if !pinned.Pinned {
		t.Fatal("expected graph to be pinned")
	}
	if err := layout.Hierarchical(pinned); err != nil {
		t.Fatal(err)
	}
	near := func(a, b layout.Vector) bool {
		return math.Abs(float64(a.X-b.X)) < 0.05 && math.Abs(float64(a.Y-b.Y)) < 0.05
	}
	for _, node := range graph.Nodes {
		if got := pinned.NodeByID[node.ID]; !near(got.Center, node.Center) {
			t.Errorf("node %v moved from %v to %v", node.ID, node.Center, got.Center)
		}
	}
	for i, edge := range graph.Edges {
		got := pinned.Edges[i]
		if len(got.Path) != len(edge.Path) || !near(got.Path[0], edge.Path[0]) || !near(got.Path[len(got.Path)-1], edge.Path[len(edge.Path)-1]) {
			t.Errorf("edge %v path changed: %v -> %v", edge, edge.Path, got.Path)
		}
		if edge.Label != "" && !near(got.LabelPos, edge.LabelPos) {
			t.Errorf("edge %v label moved from %v to %v", edge, edge.LabelPos, got.LabelPos)
		}
	}
}

// TestDiagnostics records layout.Diagnose for every example and fixture in
// testdata/diagnostics.txt, so that changes in quality show up in diffs.
func TestDiagnostics(t *testing.T) {
	var out bytes.Buffer
	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		graph := examples[name]()
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		d := layout.Diagnose(graph)
		fmt.Fprintf(&out, "%-12s %v\n", name, d)
		for _, line := range d.Details {
			t.Logf("%s: %s", name, line)
		}
	}
	files, _ := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	for _, file := range files {
		graphs, err := dot.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := layout.Hierarchical(graphs[0]); err != nil {
			t.Fatal(err)
		}
		d := layout.Diagnose(graphs[0])
		fmt.Fprintf(&out, "%-12s %v\n", strings.TrimSuffix(filepath.Base(file), ".gv"), d)
		for _, line := range d.Details {
			t.Logf("%s: %s", filepath.Base(file), line)
		}
	}
	compareGolden(t, filepath.Join("testdata", "diagnostics.txt"), out.Bytes())
}

// diffLines reports the mismatching lines between want and got.
func diffLines(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	var out strings.Builder
	for i := range max(len(wantLines), len(gotLines)) {
		w, g := "<missing>", "<missing>"
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			fmt.Fprintf(&out, "line %d:\n\twant: %s\n\tgot:  %s\n", i+1, w, g)
		}
	}
	return out.String()
}
