package text

import (
	"bytes"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/draw"
	"github.com/loov/layout/internal/examples"
)

func TestPlain(t *testing.T) {
	for label, want := range map[string]string{
		"plain\ntext":               "plain\ntext",
		"<<B>bold</B>>":             "bold",
		"<one<br/>two &amp; three>": "one\ntwo & three",
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>":    "a b\nc",
		"<  spaced   <I>out</I>  >":                                            "spaced out",
		"<<TABLE>\n\t<TR>\n\t\t<TD>a</TD>\n\t\t<TD>b</TD>\n\t</TR>\n</TABLE>>": "a b",
	} {
		// the text, without the sequences of its styles, see draw.Spans
		if got := draw.StripStyle(draw.PlainLabel(label)); got != want {
			t.Errorf("draw.PlainLabel(%q) = %q, want %q", label, got, want)
		}
	}
}

// TestHTMLLabel checks that an HTML-like label is drawn as its text in a
// complete box.
func TestHTMLLabel(t *testing.T) {
	graph := layout.NewDigraph()
	node := graph.Node("a")
	node.Shape = layout.Box
	node.Label = "<<B>bold</B>>"
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !regexp.MustCompile(`(?m)^ *┌(─+)┐\n *│ *bold *│\n *└(─+)┘$`).MatchString(got) {
		t.Errorf("want bold in a whole box:\n%s", got)
	}
}

// TestWideLabel checks that a label of wide characters fills two columns
// per character, so that the box around it lines up.
func TestWideLabel(t *testing.T) {
	graph := layout.NewDigraph()
	node := graph.Node("a")
	node.Shape = layout.Box
	node.Label = "漢字漢字"
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	var widths []int
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		widths = append(widths, draw.Columns(strings.TrimSpace(line)))
	}
	if len(widths) != 3 || widths[0] != widths[1] || widths[1] != widths[2] || widths[1] < 10 {
		t.Errorf("box rows are %v columns wide, want at least 10 each:\n%s", widths, buf.String())
	}
}

// TestClusterLabel checks that a long cluster label is drawn whole on the
// top of its frame.
func TestClusterLabel(t *testing.T) {
	graph := layout.NewDigraph()
	label := "Extremely long cluster label spanning many characters"
	graph.Clusters = []*layout.Cluster{{ID: "c", Label: label, Nodes: []*layout.Node{graph.Node("a")}}}
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "┌ "+label+" ") {
		t.Errorf("label is cut:\n%s", buf.String())
	}
}

// TestBlanksInLabels checks that tidying keeps the blank columns and rows
// inside labels and boxes.
func TestBlanksInLabels(t *testing.T) {
	graph := layout.NewDigraph()
	wide := graph.Node("a")
	wide.Label, wide.MinSize.X = "x            y", 216
	graph.Node("b").Label = "top\n\n\n\nbottom"
	graph.Edge("c", "d").Label = "p      q"
	got := render(t, graph)
	for _, want := range []string{"x            y", "p      q"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q:\n%s", want, got)
		}
	}
	lines := strings.Split(got, "\n")
	top := slices.IndexFunc(lines, func(s string) bool { return strings.Contains(s, "top") })
	bottom := slices.IndexFunc(lines, func(s string) bool { return strings.Contains(s, "bottom") })
	if bottom-top != 4 {
		t.Errorf("want 3 blank lines between top and bottom, got %d:\n%s", bottom-top-1, got)
	}
}

// TestControlCharacters checks that control characters in labels are not
// written, as they would garble the terminal, and take no columns.
func TestControlCharacters(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Shape = layout.Box
	graph.Node("a").Label = "x\x1b[31my\tz\x00"
	graph.Edge("a", "b").Label = "e\x1b[0m"
	got := render(t, graph)
	if strings.ContainsFunc(got, func(r rune) bool { return r != '\n' && unicode.IsControl(r) }) {
		t.Errorf("control characters are written:\n%q", got)
	}
	if !regexp.MustCompile(`│ *x\[31myz *│`).MatchString(got) {
		t.Errorf("want the label without control characters in a whole box:\n%s", got)
	}
}

// TestFitsLayout checks that the layout reserves the cells text draws in:
// node boxes and cluster frames don't grow past their layout boxes, and
// node boxes don't overlap, and edge labels get their columns.
func TestFitsLayout(t *testing.T) {
	labels := []string{
		"a", "hello world", "one\ntwo\nthree", "漢字", "é 👩‍💻", "iiiiiiii", "<<b>bold</b> text>",
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>",
	}
	records := []string{"a|b", "{a|b|c}", "<p> left|{top|mid|bottom}|right", "x|{y|{z|w}}", "{a\\nb|c}"}
	shapes := []layout.Shape{layout.Box, layout.Ellipse, layout.Circle, layout.Square, layout.None, layout.Record, layout.Auto}
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight} {
		for _, shape := range shapes {
			for peripheries := 1; peripheries <= 2; peripheries++ {
				graph := layout.NewDigraph()
				graph.RankDir = dir
				for i := range 8 {
					node := graph.Node(string(rune('a' + i)))
					node.Shape, node.Peripheries = shape, peripheries
					node.Label = labels[(i+peripheries)%len(labels)]
					if shape == layout.Record {
						node.Label = records[i%len(records)]
					}
				}
				for i := 1; i < 8; i++ {
					graph.Edge(graph.Nodes[(i-1)/2].ID, graph.Nodes[i].ID).Label = labels[i%len(labels)]
				}
				graph.Edge("d", "d")
				graph.Clusters = append(graph.Clusters, &layout.Cluster{ID: "c", Label: "iiiiiiiiiiiiiiii", Nodes: graph.Nodes[1:2]})
				l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
				if err != nil {
					t.Fatal(err)
				}
				c := newCanvas(l, false)
				for _, node := range graph.Nodes {
					c.drawNode(node)
				}
				for i, node := range graph.Nodes {
					box := l.Nodes[i]
					inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
					want := cellBox{point{c.col(box.Left() + inset), c.row(box.Top() + inset)}, point{c.col(box.Right() - inset), c.row(box.Bottom() - inset)}}
					if got := c.boxes[node]; got.Max.X > want.Max.X || got.Max.Y > want.Max.Y {
						t.Errorf("%v %v %d: %q drawn at %v, laid out at %v", dir, shape, peripheries, node.Label, got, want)
					}
					for _, other := range graph.Nodes[i+1:] {
						p, q := c.boxes[node], c.boxes[other]
						if p.overlaps(q) {
							t.Errorf("%v %v %d: %q at %v overlaps %q at %v", dir, shape, peripheries, node.Label, p, other.Label, q)
						}
					}
				}
				for i, edge := range graph.Edges {
					at := l.Edges[i]
					if w := draw.TextColumns(draw.PlainLabel(edge.Label)); c.col(at.LabelCenter.X+at.LabelSize.X/2)-c.col(at.LabelCenter.X-at.LabelSize.X/2) < w {
						t.Errorf("%v %v %d: edge label %q has %.1f for %d columns", dir, shape, peripheries, edge.Label, at.LabelSize.X, w)
					}
				}
				box := l.Clusters[0]
				if need := c.col(box.TopLeft.X) + clusterLabelWidth(graph.Clusters[0]); need > c.col(box.BottomRight.X) {
					t.Errorf("%v %v %d: cluster label needs column %d, frame ends at %d", dir, shape, peripheries, need, c.col(box.BottomRight.X))
				}
			}
		}
	}
}

// TestWidenedBoxFits checks that the canvas holds a box widened past its
// layout for its label, as when the layout wasn't made for text.
func TestWidenedBoxFits(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("iiiiiiiiiiiiiiiiiiii")
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Write(&out, l); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "iiii │") {
		t.Errorf("right border cut off:\n%s", out.String())
	}
}

// TestStyledText checks that bold, italic and colored text of HTML-like
// labels is written with its escape codes, and plain without them
func TestStyledText(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("a").Label = `<<b>bold</b> <i>it</i> <font color="red">red</font>>`
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var plain, colored bytes.Buffer
	if err := Write(&plain, l); err != nil {
		t.Fatal(err)
	}
	if err := WriteColor(&colored, l, Options{Palette: TrueColor}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain.String(), "bold it red") {
		t.Errorf("want the text plain:\n%s", plain.String())
	}
	for _, want := range []string{";1;", ";3;", "38;2;255;0;0"} {
		if !strings.Contains(colored.String(), want) {
			t.Errorf("want %q in:\n%q", want, colored.String())
		}
	}
}

// TestHugOrder checks that when two labels can both move into the same
// cell, the one first in reading order does, whatever map order is.
func TestHugOrder(t *testing.T) {
	// b, label of edge 2, sits above the cell its edge runs a cell below;
	// a, label of edge 1, sits left of it, its edge running down a cell
	// further right; each has the other's line beside it
	//
	//	..b|.
	//	.a.|.
	//	.--|.
	g := make(grid, 3)
	for r := range g {
		g[r] = make([]cell, 5)
		for x := range g[r] {
			g[r][x] = cell{r: ' '}
		}
	}

	vertical := cell{r: '│', lines: up | down, owner: [4]edgeID{1, 1}}
	horizontal := cell{r: '─', lines: left | right, owner: [4]edgeID{0, 0, 2, 2}}
	for r := range g {
		g[r][3] = vertical
	}
	g[2][1], g[2][2] = horizontal, horizontal
	g[0][2] = cell{r: 'b', text: 2, keep: true}
	g[1][1] = cell{r: 'a', text: 1, keep: true}

	// map order picked a in about one run of eight
	for range 50 {
		h := make(grid, len(g))
		for r := range g {
			h[r] = slices.Clone(g[r])
		}
		hug(h, []edgeID{0, 1, 2})
		if h[1][2].r != 'b' || h[1][1].r != 'a' {
			t.Fatalf("got\n%c\n%c\n%c, want b moved down, a kept", runes(h[0]), runes(h[1]), runes(h[2]))
		}
	}
}

// runes returns the characters of a row of cells
func runes(row []cell) []rune {
	var rs []rune
	for _, c := range row {
		rs = append(rs, c.r)
	}
	return rs
}

// TestHugLabelsApart checks that hug moves each label of an edge on its
// own. With one box around a label and a tail label of the same edge, the
// box took in the line between them, and moving it tore that line off
// its node.
func TestHugLabelsApart(t *testing.T) {
	graph := layout.NewDigraph()
	long := graph.Edge("n3", "n4")
	long.Label, long.TailLabel = "a long one", "ab"
	other := graph.Edge("n3", "n2")
	other.Label, other.HeadLabel = "head", "yes"
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	c, g := drawn(l, false)
	for _, edge := range lost(c, g, l) {
		t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(g, c.palette, nil))
	}
}

// TestHugPull checks that hug moves a label that its edge's line was slid
// away from back to it, a blank short of it.
func TestHugPull(t *testing.T) {
	//	..........
	//	.10.....|.
	//	..........
	g := make(grid, 3)
	for r := range g {
		g[r] = make([]cell, 10)
		for x := range g[r] {
			g[r][x] = cell{r: ' '}
		}
		g[r][8] = cell{r: '│', lines: up | down, owner: [4]edgeID{1, 1}}
	}

	g[1][1] = cell{r: '1', text: 1, keep: true}
	g[1][2] = cell{r: '0', text: 1, keep: true}
	hug(g, []edgeID{0, 1})
	if got, want := string(runes(g[1])), "     10 │ "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestLabelsDrawn checks that the text of every label, of nodes, record
// fields, edges and clusters, is in the drawing of the examples and the
// Graphviz files, laid out for text with and without compaction: a line
// drawn over a label loses its letters.
func TestLabelsDrawn(t *testing.T) {
	check := func(t *testing.T, build func() *layout.Graph, opts layout.Options) {
		for _, compact := range []bool{false, true} {
			opts := opts
			opts.ForText, opts.NoCompact = true, !compact
			l, err := layout.Hierarchical(build(), opts)
			if err != nil {
				t.Fatal(err)
			}
			c, g := drawn(l, false)
			drawing := encode(g, c.palette, nil)
			for _, text := range missingLabels(l, drawing) {
				t.Errorf("compact %v: %q is not drawn:\n%s", compact, text, drawing)
			}
		}
	}

	for name, build := range examples.Graphs {
		t.Run(name, func(t *testing.T) { check(t, build, examples.Options[name]) })
	}
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "graphviz", "*.gv"))
	for _, file := range files {
		for suffix, merge := range map[string]bool{"": false, "_merged": true} {
			t.Run(strings.TrimSuffix(filepath.Base(file), ".gv")+suffix, func(t *testing.T) {
				check(t, func() *layout.Graph {
					graphs, err := dot.ParseFile(file)
					if err != nil {
						t.Fatal(err)
					}
					graphs[0].MergeEdges = merge
					return graphs[0]
				}, layout.Options{})
			})
		}
	}
}

// missingLabels returns the lines of the labels of l that drawing has
// fewer of than l has labels with them; a line that crosses a cluster
// frame on a space of its label can stand for the space, see frameLabels
func missingLabels(l *layout.Layout, drawing string) []string {
	want := map[string]int{}
	add := func(label string) {
		for _, line := range draw.Lines(draw.StripStyle(draw.PlainLabel(label))) {
			if text := strings.TrimSpace(line.Text); text != "" {
				want[text]++
			}
		}
	}

	var fields func(rec *draw.Record)
	fields = func(rec *draw.Record) {
		if len(rec.Fields) == 0 {
			add(rec.Text)
		}
		for _, field := range rec.Fields {
			fields(field)
		}
	}

	graph := l.Graph
	for _, node := range graph.Nodes {
		box := l.Node(node)
		switch {
		case node.Invisible || box.Shape == layout.PointShape:
		case box.Shape == layout.Record:
			fields(draw.ParseRecord(box.Label, false))
		default:
			add(box.Label)
		}
	}

	for _, edge := range graph.Edges {
		if !edge.Invisible {
			add(edge.Label)
			add(edge.HeadLabel)
			add(edge.TailLabel)
		}
	}

	for _, cluster := range graph.Clusters {
		if !cluster.Invisible && cluster.Label != "" {
			add(clusterLabel(cluster))
		}
	}
	var missing []string
	for text, n := range want {
		parts := strings.Split(text, " ")
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		if len(regexp.MustCompile(strings.Join(parts, "[ │┃║┆┇┊┋╂╋┼╫╪]")).FindAllString(drawing, -1)) < n {
			missing = append(missing, text)
		}
	}

	slices.Sort(missing)
	return missing
}
