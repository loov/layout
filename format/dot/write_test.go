package dot

import (
	"bytes"
	"github.com/loov/layout"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestWritePolylineRoundTrip(t *testing.T) {
	g := layout.NewDigraph()
	e := g.Edge("a", "b")
	e.From.Center, e.To.Center = layout.Vector{X: 50, Y: 50}, layout.Vector{X: 150, Y: 150}
	e.From.Radius, e.To.Radius = layout.Vector{X: 10, Y: 10}, layout.Vector{X: 10, Y: 10}
	e.Path = []layout.Vector{{X: 60, Y: 50}, {X: 150, Y: 50}, {X: 150, Y: 140}}
	var out bytes.Buffer
	if err := Write(&out, g); err != nil {
		t.Fatal(err)
	}
	_, edgeAttrs, ok := strings.Cut(out.String(), " -> ")
	if !ok {
		t.Fatal("missing edge")
	}
	_, position, ok := strings.Cut(edgeAttrs, `pos="`)
	if !ok {
		t.Fatal("missing edge position")
	}
	position, _, _ = strings.Cut(position, `"`)
	controls := slices.DeleteFunc(strings.Fields(position), func(field string) bool {
		return strings.HasPrefix(field, "s,") || strings.HasPrefix(field, "e,")
	})
	if got := len(controls); got != 7 {
		t.Fatalf("two segments need seven cubic control points, got %d", got)
	}
	graphs, err := Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := graphs[0].Edges[0].Path; !slices.Equal(got, e.Path) {
		t.Fatalf("path changed: %v, want %v", got, e.Path)
	}
}

func TestWritePathsAreAcceptedByGraphviz(t *testing.T) {
	neato, err := exec.LookPath("neato")
	if err != nil {
		t.Skip("neato is not installed")
	}
	for _, path := range [][]layout.Vector{
		{{X: 60, Y: 50}, {X: 140, Y: 150}},
		{{X: 60, Y: 50}, {X: 150, Y: 50}, {X: 150, Y: 140}},
	} {
		g := layout.NewDigraph()
		e := g.Edge("a", "b")
		e.From.Center, e.To.Center = layout.Vector{X: 50, Y: 50}, layout.Vector{X: 150, Y: 150}
		e.From.Radius, e.To.Radius = layout.Vector{X: 10, Y: 10}, layout.Vector{X: 10, Y: 10}
		e.Path = path
		var out, diagnostics bytes.Buffer
		if err := Write(&out, g); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(neato, "-n2", "-Tsvg")
		cmd.Stdin, cmd.Stderr = &out, &diagnostics
		if err := cmd.Run(); err != nil {
			t.Fatalf("neato: %v: %s", err, &diagnostics)
		}
		if diagnostics.Len() != 0 {
			t.Fatalf("neato rejected path: %s", &diagnostics)
		}
	}
}

func TestWriteArrowsRoundTripAndRenderInGraphviz(t *testing.T) {
	for _, tc := range []struct {
		name       string
		head, tail layout.Arrow
		directed   bool
		arrows     int
	}{
		{"directed", layout.ArrowDefault, layout.ArrowDefault, true, 1},
		{"undirected", layout.ArrowDefault, layout.ArrowDefault, false, 0},
		{"both", layout.ArrowNormal, layout.ArrowNormal, true, 2},
		{"back", layout.ArrowNone, layout.ArrowNormal, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := layout.NewDigraph()
			e := g.Edge("a", "b")
			e.Directed, e.ArrowHead, e.ArrowTail = tc.directed, tc.head, tc.tail
			e.From.Center, e.To.Center = layout.Vector{X: 50, Y: 50}, layout.Vector{X: 150, Y: 150}
			e.From.Radius, e.To.Radius = layout.Vector{X: 10, Y: 10}, layout.Vector{X: 10, Y: 10}
			e.Path = []layout.Vector{{X: 60, Y: 50}, {X: 150, Y: 50}, {X: 150, Y: 140}}
			var out bytes.Buffer
			if err := Write(&out, g); err != nil {
				t.Fatal(err)
			}
			graphs, err := Parse(bytes.NewReader(out.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if got := graphs[0].Edges[0].Path; !slices.Equal(got, e.Path) {
				t.Fatalf("path changed: %v, want %v\n%s", got, e.Path, &out)
			}

			neato, err := exec.LookPath("neato")
			if err != nil {
				t.Skip("neato is not installed")
			}
			var svg, diagnostics bytes.Buffer
			cmd := exec.Command(neato, "-n2", "-Tsvg")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = &out, &svg, &diagnostics
			if err := cmd.Run(); err != nil || diagnostics.Len() != 0 {
				t.Fatalf("neato: %v: %s", err, &diagnostics)
			}
			_, edge, _ := strings.Cut(svg.String(), `class="edge"`)
			if got := strings.Count(edge, "<polygon"); got != tc.arrows {
				t.Fatalf("got %d arrowheads, want %d", got, tc.arrows)
			}
		})
	}
}

func TestWriteQuotesArrowNames(t *testing.T) {
	g := layout.NewDigraph()
	e := g.Edge("a", "b")
	e.ArrowHead, e.ArrowTail = `x]; evil [label="y`, `z]; evil2 [label="w`
	var out bytes.Buffer
	if err := Write(&out, g); err != nil {
		t.Fatal(err)
	}
	graphs, err := Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(graphs[0].Nodes) != 2 {
		t.Fatalf("arrow names injected nodes:\n%s", &out)
	}
	if got := graphs[0].Edges[0]; got.ArrowHead != e.ArrowHead || got.ArrowTail != e.ArrowTail {
		t.Fatalf("arrows = %q, %q", got.ArrowHead, got.ArrowTail)
	}
}

func TestWriteEscapesRoundTrip(t *testing.T) {
	g := layout.NewDigraph()
	e := g.Edge(`say "hi"`, `back\slash`)
	e.From.Label = "two\nlines"
	e.To.Label = `literal \n and "quotes"`
	e.Label = `C:\dir\"x"`
	var out bytes.Buffer
	if err := Write(&out, g); err != nil {
		t.Fatal(err)
	}
	graphs, err := Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := graphs[0]
	for i, node := range g.Nodes {
		if got.Nodes[i].ID != node.ID || got.Nodes[i].Label != node.Label {
			t.Errorf("node %q label %q, want %q label %q", got.Nodes[i].ID, got.Nodes[i].Label, node.ID, node.Label)
		}
	}
	if got.Edges[0].Label != e.Label {
		t.Errorf("edge label %q, want %q", got.Edges[0].Label, e.Label)
	}
}

func TestWriteQuotesShape(t *testing.T) {
	g := layout.NewDigraph()
	g.Node("a").Shape = `box]; evil [label="x`
	var out bytes.Buffer
	if err := Write(&out, g); err != nil {
		t.Fatal(err)
	}
	graphs, err := Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(graphs[0].Nodes) != 1 {
		t.Fatalf("shape injected nodes:\n%s", &out)
	}
}
