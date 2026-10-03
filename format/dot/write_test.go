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
	if got := len(strings.Fields(position)); got != 7 {
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
