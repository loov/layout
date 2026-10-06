package dot

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
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
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	start := graph.Node("start")
	if start.Shape != layout.PointShape {
		t.Fatalf("shape = %q, want %q", start.Shape, layout.PointShape)
	}
	if label := start.DefaultLabel(); label != "" {
		t.Errorf("point has label %q, want none", label)
	}
	if size := l.Node(start).Size; size.X > 8*layout.Point || size.Y > 8*layout.Point {
		t.Errorf("point size = %v, want a small dot", size)
	}
}

func TestDoubleCircle(t *testing.T) {
	graphs, err := ParseString(`digraph { a [shape=doublecircle]; b [shape=doublecircle peripheries=1]; c [peripheries=3 shape=doublecircle]; }`)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{"a": 2, "b": 1, "c": 3} {
		node := graphs[0].Node(id)
		if node.Shape != layout.Circle || node.Peripheries != want {
			t.Errorf("%s: shape %q peripheries %d, want circle with %d", id, node.Shape, node.Peripheries, want)
		}
	}
}

// TestDoubleCircleDefaults checks that node defaults decide peripheries
// together with the node's own attributes, as Graphviz does.
func TestDoubleCircleDefaults(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want map[string]int
	}{
		{`digraph { node [shape=doublecircle]; a [shape=circle]; b; }`, map[string]int{"a": 0, "b": 2}},
		{`digraph { node [peripheries=1]; a [shape=doublecircle]; }`, map[string]int{"a": 1}},
		{`digraph { a [shape=doublecircle]; a [label=x]; }`, map[string]int{"a": 2}},
	} {
		graphs, err := ParseString(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		for id, want := range tc.want {
			if got := graphs[0].Node(id).Peripheries; got != want {
				t.Errorf("%s: %s has %d peripheries, want %d", tc.src, id, got, want)
			}
		}
	}
}

func TestEmptyNodeIDIsAnError(t *testing.T) {
	for _, src := range []string{`digraph { "" -> a }`, `digraph { a -> "" }`, `graph { "" [label=x] }`} {
		if _, err := ParseString(src); err == nil {
			t.Errorf("%s: no error", src)
		}
	}
}

func TestNonFiniteNumbersAreIgnored(t *testing.T) {
	graphs, err := ParseString(`digraph {
		nodesep=nan; ranksep=inf;
		a [width=inf, height="1e308", fontsize=NaN, penwidth="-Inf"];
		a -> b [weight=nan, penwidth=infinity, fontsize="1e400", lp="nan,1", minlen=99999999];
	}`)
	if err != nil {
		t.Fatal(err)
	}
	g := graphs[0]
	a, e := g.Node("a"), g.Edges[0]
	for name, v := range map[string]layout.Length{
		"nodesep": g.NodePadding, "ranksep": g.RowPadding,
		"width": a.MinSize.X, "height": a.MinSize.Y,
		"node fontsize": a.FontSize, "node penwidth": a.LineWidth,
		"weight": layout.Length(e.Weight), "edge penwidth": e.LineWidth,
		"edge fontsize": e.FontSize,
	} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("%s = %v", name, v)
		}
	}
	if e.LabelPos != nil {
		t.Errorf("lp = %v", *e.LabelPos)
	}
	if e.MinLen > 1000 {
		t.Errorf("minlen = %d", e.MinLen)
	}
}

func TestInvalidPosDoesNotPin(t *testing.T) {
	graphs, err := ParseString(`digraph { a [pos="bad"]; b [pos="1,x"]; a -> b }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range graphs[0].Nodes {
		if node.Pos != nil {
			t.Fatalf("node %v has position %v from an invalid pos", node, *node.Pos)
		}
	}
}

func TestSolidStyleOverridesInheritedStyle(t *testing.T) {
	graphs, err := ParseString(`digraph { node [style=dashed]; edge [style="dotted"]; a [style="solid,filled"]; a -> b [style=solid] }`)
	if err != nil {
		t.Fatal(err)
	}
	g := graphs[0]
	if got := g.Node("a").LineStyle; got != layout.Solid {
		t.Errorf("node style = %q, want solid", got)
	}
	if got := g.Edges[0].LineStyle; got != layout.Solid {
		t.Errorf("edge style = %q, want solid", got)
	}
}

func TestExplicitArrowsWinOverDir(t *testing.T) {
	for _, tc := range []struct {
		src        string
		head, tail layout.Arrow
	}{
		{`digraph { a -> b [arrowhead=vee, dir=both] }`, layout.ArrowVee, layout.ArrowNormal},
		{`digraph { a -> b [dir=both, arrowtail=dot] }`, layout.ArrowNormal, layout.ArrowDot},
		{`digraph { edge [arrowhead=odot]; a -> b [dir=forward] }`, layout.ArrowODot, layout.ArrowNone},
		{`digraph { edge [dir=back]; a -> b [arrowtail=vee] }`, layout.ArrowNone, layout.ArrowVee},
	} {
		graphs, err := ParseString(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		e := graphs[0].Edges[0]
		if e.ArrowHead != tc.head || e.ArrowTail != tc.tail {
			t.Errorf("%s: arrows %q, %q, want %q, %q", tc.src, e.ArrowHead, e.ArrowTail, tc.head, tc.tail)
		}
	}
}

func TestLabelEscapes(t *testing.T) {
	graphs, err := ParseString(`digraph G {
		node [label="\N"];
		a; b [label="in \G\lleft\rright\l"]; c [label="\\N"];
		subgraph cluster_x { label="\G"; d }
		a -> b [label="\E: \T to \H"];
	}`)
	if err != nil {
		t.Fatal(err)
	}
	g := graphs[0]
	for _, tc := range []struct{ got, want string }{
		{g.Node("a").Label, "a"},
		{g.Node("b").Label, "in G" + draw.LeftMark + "\nleft" + draw.RightMark + "\nright" + draw.LeftMark},
		{g.Node("c").Label, `\N`},
		{g.Node("d").Label, "d"},
		{g.Clusters[0].Label, "cluster_x"},
		{g.Edges[0].Label, "a->b: a to b"},
	} {
		if tc.got != tc.want {
			t.Errorf("label %q, want %q", tc.got, tc.want)
		}
	}
}

func TestQuotedLabelsAreNotHTML(t *testing.T) {
	graphs, err := ParseString(`digraph {
		a [label="<init>"]; b [label=<<b>x</b>>];
		c [label="<f0>|<f1>", shape=record];
		subgraph cluster_x { label="<x>"; d }
		a -> b [label="<y>"];
	}`)
	if err != nil {
		t.Fatal(err)
	}
	g := graphs[0]
	for _, label := range []string{g.Node("a").Label, g.Clusters[0].Label, g.Edges[0].Label} {
		if draw.IsHTMLLabel(label) {
			t.Errorf("quoted label %q reads as HTML", label)
		}
	}
	if !draw.IsHTMLLabel(g.Node("b").Label) {
		t.Errorf("HTML label %q lost", g.Node("b").Label)
	}
	if got := g.Node("c").Label; got != "<f0>|<f1>" {
		t.Errorf("record label = %q", got)
	}
}

func TestPorts(t *testing.T) {
	for _, tc := range []struct {
		src      string
		from, to layout.Compass
	}{
		{`digraph { a -> b [tailport=ne, headport=s] }`, layout.NorthEast, layout.South},
		{`digraph { a -> b [tailport="p:w", headport="field:c"] }`, layout.West, layout.Center},
		{`digraph { a -> b [tailport=bogus, headport=_] }`, layout.CompassAuto, layout.CompassAuto},
		{`digraph { a:p:e -> b:_ }`, layout.East, layout.CompassAuto},
	} {
		graphs, err := ParseString(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		e := graphs[0].Edges[0]
		if e.FromPort != tc.from || e.ToPort != tc.to {
			t.Errorf("%s: ports %q, %q, want %q, %q", tc.src, e.FromPort, e.ToPort, tc.from, tc.to)
		}
	}
}

func TestStrictGraphsMergeEdges(t *testing.T) {
	for _, tc := range []struct {
		src   string
		edges int
	}{
		{`strict digraph { a -> b [color=red]; a -> b [label=x]; b -> a }`, 2},
		{`strict graph { a -- b [color=red]; b -- a [label=x]; a -- a; a -- a }`, 2},
		{`digraph { a -> b; a -> b }`, 2},
	} {
		graphs, err := ParseString(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		g := graphs[0]
		if len(g.Edges) != tc.edges {
			t.Errorf("%s: %d edges, want %d", tc.src, len(g.Edges), tc.edges)
			continue
		}
		if e := g.Edges[0]; tc.edges == 2 && strings.HasPrefix(tc.src, "strict") && (e.Label != "x" || e.LineColor == nil) {
			t.Errorf("%s: attributes not merged: label %q, color %v", tc.src, e.Label, e.LineColor)
		}
	}
}

// TestQuotedHTMLID checks that a node whose quoted id reads as HTML gets a
// default label that isn't HTML, while an HTML id stays HTML.
func TestQuotedHTMLID(t *testing.T) {
	graphs, err := ParseString(`digraph { "<init>" -> b; <<B>bold</B>> -> c; }`)
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	if label := graph.Node("<init>").DefaultLabel(); draw.IsHTMLLabel(label) {
		t.Errorf("quoted id gives an HTML label %q", label)
	}
	if label := graph.Node("<<B>bold</B>>").DefaultLabel(); !draw.IsHTMLLabel(label) {
		t.Errorf("HTML id gives a plain label %q", label)
	}
}

// TestPlaintextShape checks that plaintext and plain nodes draw without
// an outline, as in Graphviz.
func TestPlaintextShape(t *testing.T) {
	graphs, err := ParseString(`digraph { a [shape=plaintext]; b [shape=plain] }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if got := graphs[0].Node(id).Shape; got != layout.None {
			t.Errorf("%s: shape = %q, want %q", id, got, layout.None)
		}
	}
}

// TestInvisible checks that style=invis hides nodes, edges and clusters
// and survives writing the graph back as dot.
func TestInvisible(t *testing.T) {
	graphs, err := ParseString(`digraph {
		subgraph cluster_x { style=invis; a }
		a [style="dashed,invis"]
		a -> b [style=invis]
		b -> c
	}`)
	if err != nil {
		t.Fatal(err)
	}
	check := func(graph *layout.Graph) {
		t.Helper()
		if !graph.Node("a").Invisible || graph.Node("b").Invisible {
			t.Errorf("node invisible: a=%v b=%v", graph.Node("a").Invisible, graph.Node("b").Invisible)
		}
		if !graph.Edges[0].Invisible || graph.Edges[1].Invisible {
			t.Errorf("edge invisible: %v %v", graph.Edges[0].Invisible, graph.Edges[1].Invisible)
		}
		if !graph.Clusters[0].Invisible {
			t.Error("cluster is visible")
		}
	}
	check(graphs[0])
	var out bytes.Buffer
	if err := Write(&out, layoutOf(t, graphs[0])); err != nil {
		t.Fatal(err)
	}
	again, err := ParseString(out.String())
	if err != nil {
		t.Fatal(err)
	}
	check(again[0])
}

// TestRecordText checks that an HTML-like record label keeps its fields
// and loses its markup
func TestRecordText(t *testing.T) {
	got := recordText(`<{<b>«interface» I/O</b> | + a<br align="left"/>b &amp; c}>`)
	if want := "{«interface» I/O | + a" + draw.LeftMark + "\nb & c}"; got != want {
		t.Errorf("recordText = %q, want %q", got, want)
	}
}
