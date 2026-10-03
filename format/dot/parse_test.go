package dot

import (
	"fmt"
	"testing"
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
