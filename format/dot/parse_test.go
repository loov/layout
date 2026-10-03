package dot

import (
	"fmt"
	"testing"

	"github.com/loov/layout"
)

func TestSubgraphRankAttributesAcceptBothForms(t *testing.T) {
	for _, tc := range []struct {
		rank   string
		groups [3]int
	}{
		{"same", [3]int{1, 0, 0}},
		{"min", [3]int{0, 2, 0}},
		{"source", [3]int{0, 2, 0}},
		{"max", [3]int{0, 0, 2}},
		{"sink", [3]int{0, 0, 2}},
	} {
		for _, form := range []string{"rank=%s;", "graph [rank=%s];"} {
			t.Run(tc.rank+"/"+form, func(t *testing.T) {
				graphs, err := ParseString("digraph {{" + fmt.Sprintf(form, tc.rank) + "a; b;} a -> c; c -> b;}")
				if err != nil {
					t.Fatal(err)
				}
				g := graphs[0]
				got := [3]int{len(g.SameRank), len(g.MinRank), len(g.MaxRank)}
				if got != tc.groups {
					t.Fatalf("rank groups = %v, want %v", got, tc.groups)
				}
			})
		}
	}
}

func TestSubgraphRankUsesLastAssignment(t *testing.T) {
	for _, tc := range []struct {
		stmts  string
		groups [3]int
	}{
		{"graph [rank=same]; rank=max;", [3]int{0, 0, 2}},
		{"rank=max; graph [rank=same];", [3]int{1, 0, 0}},
		{"rank=min; rank=same;", [3]int{1, 0, 0}},
	} {
		t.Run(tc.stmts, func(t *testing.T) {
			graphs, err := ParseString("digraph {{" + tc.stmts + "a; b;} a -> c; c -> b;}")
			if err != nil {
				t.Fatal(err)
			}
			g := graphs[0]
			got := [3]int{len(g.SameRank), len(g.MinRank), len(g.MaxRank)}
			if got != tc.groups {
				t.Fatalf("rank groups = %v, want %v", got, tc.groups)
			}
		})
	}
}

func TestSubgraphRankIsInheritedFromEnclosingGraph(t *testing.T) {
	for _, tc := range []struct {
		src    string
		groups [3]int
	}{
		{"digraph { rank=same; {a; b} a -> b }", [3]int{1, 0, 0}},
		{"digraph { graph [rank=same]; {a; b} a -> b }", [3]int{1, 0, 0}},
		{"digraph { {a; b} rank=same; a -> b }", [3]int{0, 0, 0}},
		{"digraph { {rank=min; {a; b}} a -> c; b -> c }", [3]int{0, 2, 0}},
		{"digraph { {rank=max; {rank=same; a; b}} a -> b }", [3]int{1, 0, 2}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			graphs, err := ParseString(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			g := graphs[0]
			got := [3]int{len(g.SameRank), len(g.MinRank), len(g.MaxRank)}
			if got != tc.groups {
				t.Fatalf("rank groups = %v, want %v", got, tc.groups)
			}
		})
	}
}

func TestPointShape(t *testing.T) {
	graphs, err := ParseString(`digraph { start [shape=point label="ignored"]; start -> a; }`)
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	start := graph.Node("start")
	if start.Shape != layout.Dot {
		t.Fatalf("shape = %q, want %q", start.Shape, layout.Dot)
	}
	if label := start.DefaultLabel(); label != "" {
		t.Errorf("point has label %q, want none", label)
	}
	if start.Radius.X > 4*layout.Point || start.Radius.Y > 4*layout.Point {
		t.Errorf("point radius = %v, want a small dot", start.Radius)
	}
}
