package layout

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/loov/layout/internal/draw"
)

// Compaction takes the room out of a routed layout on its own geometry,
// the way format/text carves seams out of its character grid: what can
// move is pushed together along one axis and then the other, while edges
// stretch and shrink to follow their ends, see Options.Compact.
//
// It is one-dimensional compaction as in VLSI layout. Along x, node boxes,
// the vertical runs of edges, labels and the sides of clusters are rigid
// objects; the horizontal runs between them are elastic. Every two objects
// that share a row keep their order and at least a gap between them, and
// the least positions that satisfy all of those difference constraints
// give the narrowest drawing; network simplex then shortens the edges at
// that width. Along y the roles swap: the layout is transposed and
// compacted again.

// compactGrid is the grid compaction measures and moves objects on:
// everything moves by whole cells, so a drawing on a grid rounds the
// same. space is the cells from the end of an object to the start of the
// next one that faces it, and lineSpace from a line to the next, to a
// node, or to its own label; an arrowhead takes 2*lineSpace.
type compactGrid struct {
	cell             Vector
	space, lineSpace int
}

// textGrid returns the grid of a layout made for format/text: its
// cells, with a blank cell between objects, as carving leaves, and lines
// in neighboring cells.
func textGrid(graph *Graph) compactGrid {
	return compactGrid{cell: Vector{X: graph.FontSize * 0.55, Y: graph.LineHeight}, space: 2, lineSpace: 1}
}

// pointGrid returns the grid for other layouts: finer than the strokes,
// so that rounding to it moves nothing visibly, lines EdgePadding apart
// and other objects NodePadding.
func pointGrid(graph *Graph) compactGrid {
	const perPoint = 4
	return compactGrid{
		cell:      Vector{X: 1.0 / perPoint, Y: 1.0 / perPoint},
		space:     int(graph.NodePadding * perPoint),
		lineSpace: int(graph.EdgePadding * perPoint),
	}
}

// compact takes the room out from between the objects of l, along x and
// along y until neither shrinks. Nodes of a rank stay in line. Edge paths
// should be orthogonal; a sloped segment moves as a whole with its ends,
// which keeps it but compacts less around it.
func compact(l *Layout, grid compactGrid) {
	sideways := sideways(l.Graph.RankDir)
	// a cluster is as wide as its label on the top of its frame, see
	// format/text's clusterLabelWidth, and two cells tall for its frame
	spanX, spanY := make([]int, len(l.Clusters)), make([]int, len(l.Clusters))
	for i, cluster := range l.Graph.Clusters {
		spanY[i] = 2
		if cluster.Label != "" {
			spanX[i] = draw.Columns(strings.ReplaceAll(draw.PlainLabel(cluster.Label), "\n", " ")) + 3
		}
	}
	// paths move in place, and an edge listed twice shares its path
	for i := range l.Edges {
		l.Edges[i].Path = slices.Clone(l.Edges[i].Path)
	}
	cell := grid.cell
	// text measures cells from 0, or from where the drawing starts before
	// it; the drawing goes past 0 by whole cells, so that compacting can
	// keep it there and round as text does
	topLeft, _ := l.Bounds()
	translateLayout(l, Vector{
		X: max(0, Length(math.Ceil(float64(-topLeft.X/cell.X)))) * cell.X,
		Y: max(0, Length(math.Ceil(float64(-topLeft.Y/cell.Y)))) * cell.Y,
	})
	crowd := crowded(l, cell)
	// A pass along one axis can clear the way for the other. Passes add
	// no crossings nor overlaps, see TestCompact; checking that with
	// Diagnose after every pass would double the time.
	for range 4 {
		moved := compactAxis(l, cell, grid, sideways, false, spanX, crowd)
		transposeLayout(l)
		moved = compactAxis(l, Vector{X: cell.Y, Y: cell.X}, grid, !sideways, true, spanY, crowd) || moved
		transposeLayout(l)
		if !moved {
			break
		}
	}
	for i := range l.Edges {
		l.Edges[i].Path = simplifyPath(l.Edges[i].Path)
	}
	onOutlines(l)
}

// onOutlines puts the ends of edges back on the outlines of ellipses
// and polygons: an end moves with the run it is on, which can slide it
// along such an outline, and into the node
func onOutlines(l *Layout) {
	for i, edge := range l.Graph.Edges {
		path := l.Edges[i].Path
		if len(path) < 2 || edge.From == edge.To {
			continue
		}
		for _, end := range [2]struct {
			at, next int
			node     *Node
		}{{0, 1, edge.From}, {len(path) - 1, len(path) - 2, edge.To}} {
			box := l.Nodes[l.nodeIndex[end.node]]
			switch box.Shape {
			case Box, Square, Record, None, PointShape:
				continue
			}
			// as Diagnose tells an end inside its node, see Shafts
			if node := box.node(); segmentHitsNode(path[end.at], path[end.at], node, -0.5) {
				path[end.at] = node.outlineAlong(path[end.at], path[end.next])
			}
		}
	}
}

// crowded returns the edges that end where another edge ends, or run
// along one, in the same cells: text draws them apart where there
// is room, so what is beside them keeps the room it has
func crowded(l *Layout, cell Vector) []bool {
	out := make([]bool, len(l.Edges))
	near := func(a, b Vector) bool {
		return math.Abs(float64(a.X-b.X)) < float64(cell.X)/2 && math.Abs(float64(a.Y-b.Y)) < float64(cell.Y)/2
	}
	// along reports whether the segments ab and cd run along each other
	along := func(a, b, c, d Vector) bool {
		overlap := func(a0, a1, b0, b1 Length) bool {
			return max(min(a0, a1), min(b0, b1)) < min(max(a0, a1), max(b0, b1))
		}
		switch {
		case a.X == b.X && c.X == d.X:
			return math.Abs(float64(a.X-c.X)) < float64(cell.X)/2 && overlap(a.Y, b.Y, c.Y, d.Y)
		case a.Y == b.Y && c.Y == d.Y:
			return math.Abs(float64(a.Y-c.Y)) < float64(cell.Y)/2 && overlap(a.X, b.X, c.X, d.X)
		}
		return false
	}
	// O(n²) over the segments of all edges; a sweep would do on big graphs
	for i, p := range l.Edges {
		for j := i + 1; j < len(l.Edges); j++ {
			q := l.Edges[j]
			if len(p.Path) < 2 || len(q.Path) < 2 || p.Merged != [2]int{} && p.Merged == q.Merged {
				continue
			}
			// edges out of one node, or into one, leave it together and
			// fork, and run along each other as they do
			ei, ej := l.Graph.Edges[i], l.Graph.Edges[j]
			fork := ei.From == ej.From || ei.To == ej.To || ei.From == ej.To || ei.To == ej.From
			pe, qe := p.Path[len(p.Path)-1], q.Path[len(q.Path)-1]
			hit := near(p.Path[0], q.Path[0]) && ei.From != ej.From || near(pe, qe) && ei.To != ej.To ||
				near(p.Path[0], qe) || near(pe, q.Path[0])
			for a := 0; a+1 < len(p.Path) && !hit && !fork; a++ {
				for b := 0; b+1 < len(q.Path) && !hit; b++ {
					hit = along(p.Path[a], p.Path[a+1], q.Path[b], q.Path[b+1])
				}
			}
			if hit {
				out[i], out[j] = true, true
			}
		}
	}
	return out
}

// headMarked and tailMarked report whether text draws an arrowhead or
// another marker at the head or the tail of edge
func headMarked(edge *Edge) bool {
	return (edge.ArrowHead != "" || edge.Directed) && edge.ArrowHead != ArrowNone
}

func tailMarked(edge *Edge) bool { return edge.ArrowTail != "" && edge.ArrowTail != ArrowNone }

// rigid is something rigid along the axis of a pass: it moves with item,
// and covers the cells lo to hi along the axis and c0 to c1 across it
type rigid struct {
	item   int
	lo, hi int
	c0, c1 int
	kind   rigidKind
	index  int // the node, or the edge of a run or label
	group  int // which run of the edge, in path order
	// of an invisible edge: it moves along, but nothing keeps clear of it
	hidden bool
	corner bool // a run at the end of an edge in the column of a corner
	marker bool // the cell of an arrowhead before a box, see addEdges
}

type rigidKind uint8

const (
	nodeKind rigidKind = iota
	runKind
	labelKind
	borderKind // a side of a cluster
)

// labelRef is a label of an edge (which: 0 its label, 1 head, 2 tail)
// beside the run of anchor: side -1 before it, 1 after it, dist cells
// off; 0 along it, moving with it. With side 2 it is along the elastic
// run from anchor to end, its middle dist cells past the start and
// endDist before the end. at is the cell text measures it from.
type labelRef struct{ edge, which, item, anchor, end, side, dist, endDist, at int }

// compactPass compacts a layout along x
type compactPass struct {
	l    *Layout
	cell Vector
	grid compactGrid
	flip bool // l is transposed
	// edges that share an end or a run with another, see crowded
	crowded []bool

	objs   []rigid
	parent []int // items, joined by union
	varOf  []int // the variable of every item, see variables
	// the item of every point of every edge path, see addEdges
	pointItem [][]int
	labels    []labelRef
	sides     [][2]int // the objects of the sides of every cluster
}

// compactAxis pushes the objects of l together along x, with the nodes
// of a rank in line when tie is set (ranks are columns), and reports
// whether anything moved; l is transposed when flip is set. A cluster i
// stays at least span[i] cells wide.
func compactAxis(l *Layout, cell Vector, grid compactGrid, tie, flip bool, span []int, crowd []bool) bool {
	p := &compactPass{l: l, cell: cell, grid: grid, flip: flip, crowded: crowd}
	p.addNodes(tie)
	p.addEdges()
	p.addClusters(span)
	n := p.variables()
	cons := p.constraints(span)

	// nothing moves past where the drawing starts, with the margin of a
	// cell that text leaves, and past 0, see compact
	start := math.MaxInt
	for _, o := range p.objs {
		start = min(start, o.lo)
	}
	start = max(min(start, grid.lineSpace), 1)
	lower, reach := make([]int, n), make([]int, n)
	for i := range lower {
		lower[i], reach[i] = math.MinInt32, math.MinInt32
	}
	for _, o := range p.objs {
		v := p.varOf[o.item]
		lower[v] = max(lower[v], start-o.lo)
		reach[v] = max(reach[v], o.hi)
	}
	d := leastSolution(n, cons, lower)
	d = shortenRuns(n, cons, lower, reach, d)
	return p.apply(d)
}

// border returns the cell of the side of node that a run to the cell
// toward leaves from, or at when toward is not beside the node: the ends
// of paths can sit a cell inside the border, where text draws them
func (p *compactPass) border(node *Node, toward, at int) int {
	box := p.objs[p.l.nodeIndex[node]]
	switch {
	case p.l.Nodes[p.l.nodeIndex[node]].Shape == PointShape:
		// a point is one cell, see addNodes, but the ends of its edges
		// stay where the layout put them, on the border of its box
		return at
	case toward > box.hi:
		return box.hi
	case toward < box.lo:
		return box.lo
	}
	return at
}

// col and row return the cell of a coordinate, as format/text rounds to
// cells, from 0, see compact
func (p *compactPass) col(v Length) int { return int(math.Floor(float64(v/p.cell.X) + 0.5)) }
func (p *compactPass) row(v Length) int { return int(math.Floor(float64(v/p.cell.Y) + 0.5)) }

// newItem returns a new item, which moves on its own until joined
func (p *compactPass) newItem() int {
	p.parent = append(p.parent, len(p.parent))
	return len(p.parent) - 1
}

func (p *compactPass) find(i int) int {
	if p.parent[i] != i {
		p.parent[i] = p.find(p.parent[i])
	}
	return p.parent[i]
}

// union joins the items a and b, which then move together
func (p *compactPass) union(a, b int) { p.parent[p.find(a)] = p.find(b) }

// addNodes adds the boxes of the nodes, as items 0 to n-1; with tie, the
// nodes of a rank move together
func (p *compactPass) addNodes(tie bool) {
	rank := map[Length]int{}
	for i, box := range p.l.Nodes {
		p.newItem()
		o := rigid{item: i, lo: p.col(box.Left()), hi: p.col(box.Right()), c0: p.row(box.Top()), c1: p.row(box.Bottom()), kind: nodeKind, index: i}
		if box.Shape == PointShape {
			// text draws a point in one cell, however much room the layout
			// keeps around it
			o.lo, o.c0 = p.col(box.Center.X), p.row(box.Center.Y)
			o.hi, o.c1 = o.lo, o.c0
		}
		p.objs = append(p.objs, o)
		if tie {
			if j, ok := rank[box.Center.X]; ok {
				p.union(i, j)
			}
			rank[box.Center.X] = i
		}
	}
}

// addEdges adds the runs of the edges and their labels. Points along a
// run across the axis, or a sloped segment, share an item; a run along
// the axis is elastic between its ends.
func (p *compactPass) addEdges() {
	p.pointItem = make([][]int, len(p.l.Edges))
	merged := map[[4]int]int{}
	for e, path := range p.l.Edges {
		if len(path.Path) == 0 {
			continue
		}
		edge := p.l.Graph.Edges[e]
		items := make([]int, len(path.Path))
		group := -1
		var groups []int // index into objs of each run
		pointGroup := make([]int, len(path.Path))
		for i, pt := range path.Path {
			c, r := p.col(pt.X), p.row(pt.Y)
			if i == 0 || r == p.row(path.Path[i-1].Y) && c != p.col(path.Path[i-1].X) {
				group++
				pointGroup[i] = group
				items[i] = p.newItem()
				groups = append(groups, len(p.objs))
				p.objs = append(p.objs, rigid{item: items[i], lo: c, hi: c, c0: r, c1: r, kind: runKind, index: e, group: group, hidden: edge.Invisible})
				continue
			}
			pointGroup[i] = group
			items[i] = items[i-1]
			o := &p.objs[groups[group]]
			o.lo, o.hi, o.c0, o.c1 = min(o.lo, c), max(o.hi, c), min(o.c0, r), max(o.c1, r)
		}
		// the ends stay where they leave their nodes; a run that leaves
		// the top or bottom in the column of a corner must not line up
		// with the next, or text can't tell it from the corner
		for _, end := range [2]struct {
			at   int
			node *Node
		}{{0, edge.From}, {len(items) - 1, edge.To}} {
			node := p.l.nodeIndex[end.node]
			p.union(items[end.at], node)
			run, box := &p.objs[groups[pointGroup[end.at]]], p.objs[node]
			run.corner = run.c0 != run.c1 && (run.lo == box.lo || run.lo == box.hi)
			// a run along the side of its node, as from a field to a
			// port past it, text draws out of the side instead, straight
			// for two cells before it can turn: keep them
			if run.c0 != run.c1 && run.c0 >= box.c0 && run.c1 <= box.c1 {
				switch run.lo {
				case box.lo:
					run.lo -= 2 * p.grid.lineSpace
				case box.hi:
					run.hi += 2 * p.grid.lineSpace
				}
			}
		}
		// merged edges run together from their shared end to where they
		// turn apart, and stay together where their points meet
		for i, pt := range path.Path {
			for k, group := range path.Merged {
				if group == 0 {
					continue
				}
				at := [4]int{k, group, p.col(pt.X), p.row(pt.Y)}
				if item, ok := merged[at]; ok {
					p.union(items[i], item)
				} else {
					merged[at] = items[i]
				}
			}
		}
		// an arrowhead at a box takes the cell before its border: a line
		// that ran through that cell would leave text no room for it but
		// on the border. A run across the axis covers it already.
		for _, end := range [2]struct {
			at, next int
			node     *Node
			marked   bool
		}{{0, 1, edge.From, tailMarked(edge)}, {len(items) - 1, len(items) - 2, edge.To, headMarked(edge)}} {
			if !end.marked || len(items) < 2 || p.l.Nodes[p.l.nodeIndex[end.node]].Shape == None {
				continue
			}
			at, next := path.Path[end.at], path.Path[end.next]
			c, r := p.col(next.X), p.row(at.Y)
			if p.row(next.Y) != r || c == p.col(at.X) {
				continue
			}
			// the cell past the side the run leaves from, also when the
			// end is on that side already
			switch box := p.objs[p.l.nodeIndex[end.node]]; {
			case c > box.hi:
				c = box.hi + 1
			case c < box.lo:
				c = box.lo - 1
			default:
				continue
			}
			// not a group next to the runs of the edge, see need
			p.objs = append(p.objs, rigid{item: items[end.at], lo: c, hi: c, c0: r, c1: r, kind: runKind, index: e, group: -2, hidden: edge.Invisible, marker: true})
		}
		p.pointItem[e] = items

		for which, label := range []struct {
			text         string
			center, size Vector
		}{
			{edge.Label, path.LabelCenter, path.LabelSize},
			{edge.HeadLabel, path.HeadLabelCenter, path.HeadLabelSize},
			{edge.TailLabel, path.TailLabelCenter, path.TailLabelSize},
		} {
			if label.text != "" {
				p.addLabel(e, which, label.center, label.size, items, func(i int) rigid { return p.objs[groups[pointGroup[i]]] })
			}
		}
	}
}

// addLabel adds a label of edge e: it stays beside the segment of the
// path nearest to it, on the same side and no further, but it may come
// closer; along an elastic run its middle stays over the run. run
// returns the run of a point of the path, whose items are items.
func (p *compactPass) addLabel(e, which int, center, size Vector, items []int, run func(i int) rigid) {
	path := p.l.Edges[e].Path
	o := rigid{item: p.newItem(), kind: labelKind, index: e, hidden: p.l.Graph.Edges[e].Invisible}
	// as format/text puts a label: from its left, and around its middle
	// row
	n := max(1, int(math.Round(float64(size.X/p.cell.X))))
	o.lo = p.col(center.X - size.X/2)
	if p.flip {
		o.lo = p.col(center.X) - (n-1)/2
	}
	o.hi = o.lo + n - 1
	n = max(1, int(math.Round(float64(size.Y/p.cell.Y))))
	o.c0 = p.row(center.Y) - (n-1)/2
	if p.flip {
		o.c0 = p.row(center.Y - size.Y/2)
	}
	o.c1 = o.c0 + n - 1

	best, bestDist := 0, math.MaxInt
	for i := range max(len(items)-1, 1) {
		a, b := path[i], path[min(i+1, len(items)-1)]
		lo, hi := min(p.col(a.X), p.col(b.X)), max(p.col(a.X), p.col(b.X))
		c0, c1 := min(p.row(a.Y), p.row(b.Y)), max(p.row(a.Y), p.row(b.Y))
		if d := max(lo-o.hi, o.lo-hi, 0) + max(c0-o.c1, o.c0-c1, 0); d < bestDist {
			best, bestDist = i, d
		}
	}
	ref := labelRef{edge: e, which: which, item: o.item, at: o.lo}
	if p.flip {
		ref.at = p.col(center.X)
	}
	i, j := best, min(best+1, len(items)-1)
	ci, cj := p.col(path[i].X), p.col(path[j].X)
	if items[i] != items[j] && ci > cj {
		i, j, ci, cj = j, i, cj, ci
	}
	switch anchor := run(i); {
	case items[i] != items[j] && o.lo <= cj && o.hi >= ci && which != 0:
		// a head or tail label along a run keeps its distance from the
		// end it labels, or it could slide to the other
		k := max(i, j)
		if which == 2 {
			k = min(i, j)
		}
		ref.anchor = items[k]
		p.union(o.item, items[k])
	case items[i] != items[j] && o.lo <= cj && o.hi >= ci:
		ref.side, ref.anchor, ref.end = 2, items[i], items[j]
		ref.dist, ref.endDist = (o.lo+o.hi)/2-ci, cj-(o.lo+o.hi)/2
	case items[i] != items[j] && o.lo > cj:
		// past the end of an elastic run
		anchor = run(j)
		fallthrough
	default:
		ref.anchor = anchor.item
		switch {
		case o.hi < anchor.lo:
			ref.side, ref.dist = -1, anchor.lo-o.hi
		case o.lo > anchor.hi:
			ref.side, ref.dist = 1, o.lo-anchor.hi
		default:
			p.union(o.item, anchor.item)
		}
	}
	p.objs = append(p.objs, o)
	p.labels = append(p.labels, ref)
}

// addClusters adds the sides of the clusters, and along x their labels,
// which text writes over the top of the frame from the corner on, span
// cells: lines that cross the frame there would be cut
func (p *compactPass) addClusters(span []int) {
	p.sides = make([][2]int, len(p.l.Clusters))
	for i, box := range p.l.Clusters {
		c0, c1 := p.row(box.TopLeft.Y), p.row(box.BottomRight.Y)
		x0, x1 := p.col(box.TopLeft.X), p.col(box.BottomRight.X)
		p.sides[i] = [2]int{len(p.objs), len(p.objs) + 1}
		left := p.newItem()
		p.objs = append(p.objs,
			rigid{item: left, lo: x0, hi: x0, c0: c0, c1: c1, kind: borderKind},
			rigid{item: p.newItem(), lo: x1, hi: x1, c0: c0, c1: c1, kind: borderKind})
		if !p.flip && span[i] > 0 {
			p.objs = append(p.objs, rigid{item: left, lo: x0 + 1, hi: x0 + span[i] - 1, c0: c0, c1: c0, kind: labelKind, index: -1})
		}
	}
}

// variables numbers the joined items, and returns how many there are
func (p *compactPass) variables() int {
	vars := map[int]int{}
	p.varOf = make([]int, len(p.parent))
	for i := range p.parent {
		r := p.find(i)
		if _, ok := vars[r]; !ok {
			vars[r] = len(vars)
		}
		p.varOf[i] = vars[r]
	}
	return len(vars)
}

// need returns the cells from the end of a to the start of b: space, but
// lines may run side by side, and beside nodes and their own labels, as
// carving leaves them. Two runs of an edge that meet only at the run
// between them may line up, which straightens the edge.
func (p *compactPass) need(a, b rigid) int {
	if a.kind > b.kind {
		a, b = b, a
	}
	plain := func(o rigid) bool { return o.kind == nodeKind && p.l.Nodes[o.index].Shape == None }
	switch {
	case a.marker || b.marker:
		// a marker only keeps its own cell
		return p.grid.lineSpace
	case plain(a) && plain(b):
		// the blank frames of nodes without outline keep their texts
		// apart already
		return p.grid.lineSpace
	case a.kind == runKind && b.kind == runKind:
		if a.index == b.index && (a.group-b.group == 1 || b.group-a.group == 1) && (a.c1 == b.c0 || b.c1 == a.c0) && !a.corner && !b.corner {
			return 0
		}
		return p.grid.lineSpace
	case a.kind == nodeKind && b.kind == runKind:
		return p.grid.lineSpace
	case a.kind == runKind && b.kind == labelKind && a.index == b.index:
		return p.grid.lineSpace
	}
	return p.grid.space
}

// constraints returns the constraints on how far the variables move
func (p *compactPass) constraints(span []int) (cons []constraint) {
	objs, varOf, grid := p.objs, p.varOf, p.grid
	cons = p.separations()
	// a run along the axis doesn't turn around; one that ends at a node
	// keeps a cell for each arrowhead or other marker at its ends, and
	// from a node with an outline a cell more past its border: a bend
	// next to the border, or the line between two boxes, which text
	// would otherwise draw as a bare arrowhead between borders
	for e, items := range p.pointItem {
		path, edge := p.l.Edges[e].Path, p.l.Graph.Edges[e]
		for i := 1; i < len(items); i++ {
			va, vb := varOf[items[i-1]], varOf[items[i]]
			if va == vb {
				continue
			}
			ca, cb := p.col(path[i-1].X), p.col(path[i].X)
			a, b := ca, cb
			// (a node without outline takes its marker in its blank frame)
			keep := 0
			end := func(node *Node, marked bool, at *int, toward int) {
				if marked {
					keep += grid.lineSpace
				}
				if p.l.Nodes[p.l.nodeIndex[node]].Shape != None {
					keep += grid.lineSpace
					*at = p.border(node, toward, *at)
				}
			}
			if i == len(items)-1 {
				end(edge.To, headMarked(edge), &b, ca)
			}
			if i == 1 {
				end(edge.From, tailMarked(edge), &a, cb)
			}
			if a > b {
				a, b, va, vb = b, a, vb, va
			}
			cons = append(cons, constraint{va, vb, min(keep, b-a) - (b - a), true})
		}
	}
	// a label beside its run comes closer to it, but not up to it, as
	// the runs it shares no row with don't keep it off; shortenRuns pulls
	// it in as it does the runs of edges
	for _, lb := range p.labels {
		label, anchor := varOf[lb.item], varOf[lb.anchor]
		switch lb.side {
		case -1:
			cons = append(cons, constraint{anchor, label, 0, false}, constraint{label, anchor, min(grid.lineSpace, lb.dist) - lb.dist, true})
		case 1:
			cons = append(cons, constraint{label, anchor, 0, false}, constraint{anchor, label, min(grid.lineSpace, lb.dist) - lb.dist, true})
		case 2:
			end := varOf[lb.end]
			cons = append(cons, constraint{anchor, label, min(0, lb.dist) - lb.dist, false}, constraint{label, end, min(0, lb.endDist) - lb.endDist, false})
		}
	}
	for i, s := range p.sides {
		a, b := objs[s[0]], objs[s[1]]
		dist := b.lo - a.hi
		cons = append(cons, constraint{varOf[a.item], varOf[b.item], min(span[i], dist) - dist, false})
	}
	return cons
}

// separations returns the constraints that keep objects apart. Every two
// objects that share a row, or come within lineSpace of it, keep their
// order and the cells they need, or as many as they have when fewer:
// what overlaps already gets no closer. The rows around count too, as the
// pass across can stretch the runs that join objects into rows they
// don't share now: such runs keep apart as the objects at their ends do.
//
// Most of those pairs follow from others, and network simplex spends its
// time on how many constraints it gets. In a row, the objects between a
// and c keep each the next apart, and with it a from c by the gaps
// between them and their widths: when that is as much as a and c need,
// their own constraint is left out, which keeps the same solutions. So
// every object looks back along each of its rows only until the objects
// between keep anything further back apart from it.
func (p *compactPass) separations() []constraint {
	objs, varOf, grid := p.objs, p.varOf, p.grid
	// the most cells two objects need, see need
	most := max(grid.space, grid.lineSpace) + grid.lineSpace
	// objects in the order their pairs are oriented: by middle, then index
	byRank := make([]int, len(objs))
	for i := range byRank {
		byRank[i] = i
	}
	slices.SortFunc(byRank, func(a, b int) int {
		return cmp.Or(objs[a].lo+objs[a].hi-objs[b].lo-objs[b].hi, a-b)
	})
	// gap returns the cells that the objects i and j, i first, keep
	// between them: as many as they have when they move together
	gap := func(i, j int) int {
		a, b := objs[i], objs[j]
		dist := b.lo - a.hi
		if varOf[a.item] == varOf[b.item] {
			return dist
		}
		need := p.need(a, b)
		if !a.marker && !b.marker && (a.kind == runKind && p.crowded[a.index] || b.kind == runKind && p.crowded[b.index]) {
			need += grid.lineSpace // room to draw them apart, see crowded
		}
		return min(need, dist)
	}

	// Any two objects in one of the rows near pair, as each covers its
	// rows and lineSpace more below them. A marker keeps only its own
	// cell, so it pairs only with what is in its row, in along.
	var others, markerRows []int // where the rows start
	for _, o := range objs {
		switch {
		case o.hidden:
		case o.marker:
			markerRows = append(markerRows, o.c0)
		default:
			others = append(others, o.c0, o.c1+grid.lineSpace+1)
		}
	}
	slices.Sort(others)
	others = slices.Compact(others)
	slices.Sort(markerRows)
	markerRows = slices.Compact(markerRows)
	near := p.rowsOf(byRank, others, func(o rigid) (int, int, bool) {
		return o.c0, o.c1 + grid.lineSpace, !o.hidden && !o.marker
	}, gap)
	along := p.rowsOf(byRank, markerRows, func(o rigid) (int, int, bool) {
		return o.c0, o.c1, !o.hidden
	}, gap)

	// pairs go in order of the first row of one object and then of the
	// other, as network simplex breaks ties by the order it gets them in
	pos := make([]int, len(objs))
	for i := range pos {
		pos[i] = i
	}
	slices.SortFunc(pos, func(a, b int) int { return objs[a].c0 - objs[b].c0 })
	for x, i := range slices.Clone(pos) {
		pos[i] = x
	}
	type pair struct {
		key uint64
		c   constraint
	}
	var pairs []pair
	seen := make([]int, len(objs)) // the object a candidate was found for, +1
	implied := make([]bool, len(objs))
	var found []int
	for c, o := range objs {
		if o.hidden {
			continue
		}
		found = found[:0]
		walk := func(rs *rowSet, markersOnly bool) {
			for r := rs.first[c]; r <= rs.last[c]; r++ {
				row, q, qmax := rs.row(r)
				pc := rs.at[rs.atStart[c]+r-rs.first[c]]
				for i := pc - 1; i >= 0; i-- {
					// what is further back is kept apart by what is between
					if i < pc-1 && q[pc]-qmax[i]-(o.hi-o.lo) >= most {
						break
					}
					a := row[i]
					if markersOnly && !o.marker && !objs[a].marker || varOf[objs[a].item] == varOf[o.item] {
						continue
					}
					if seen[a] != c+1 {
						seen[a], implied[a] = c+1, false
						found = append(found, a)
					}
					if i < pc-1 && !implied[a] && q[pc]-q[i]-(o.hi-o.lo) >= gap(a, c) {
						implied[a] = true
					}
				}
			}
		}
		walk(near, false)
		walk(along, true)
		for _, a := range found {
			if !implied[a] {
				x, y := pos[a], pos[c]
				pairs = append(pairs, pair{uint64(min(x, y))<<32 | uint64(max(x, y)), constraint{varOf[objs[a].item], varOf[o.item], gap(a, c) - (o.lo - objs[a].hi), false}})
			}
		}
	}
	slices.SortFunc(pairs, func(a, b pair) int { return cmp.Compare(a.key, b.key) })
	cons := make([]constraint, len(pairs))
	for i, pr := range pairs {
		cons[i] = pr.c
	}
	return cons
}

// rowSet is the objects in a set of rows, each row in the order of
// byRank, with the gaps that keep them apart
type rowSet struct {
	start []int // where every row starts in objs, q and qmax
	objs  []int
	// q[k] is the cells from the start of the first object of its row to
	// the end of the k-th, as the gaps between them keep them; qmax[k]
	// the most of q from the start of the row to k
	q, qmax     []int
	first, last []int // the rows of every object, last < first if none
	atStart     []int // where the rows of every object start in at
	at          []int // where an object is in each of its rows
}

func (rs *rowSet) row(r int) (objs, q, qmax []int) {
	a, b := rs.start[r], rs.start[r+1]
	return rs.objs[a:b], rs.q[a:b], rs.qmax[a:b]
}

// rowsOf returns the objects in the rows that start at starts, which end
// where the next starts; span returns the rows an object covers, which
// start and end with rows of starts, and whether it is in any
func (p *compactPass) rowsOf(byRank, starts []int, span func(rigid) (lo, hi int, ok bool), gap func(i, j int) int) *rowSet {
	n := len(p.objs)
	rs := &rowSet{start: make([]int, len(starts)+1), first: make([]int, n), last: make([]int, n), atStart: make([]int, n)}
	total := 0
	for i, o := range p.objs {
		rs.first[i], rs.last[i] = 0, -1
		if lo, hi, ok := span(o); ok {
			rs.first[i], _ = slices.BinarySearch(starts, lo)
			rs.last[i], _ = slices.BinarySearch(starts, hi+1)
			rs.last[i]--
		}
		rs.atStart[i] = total
		for r := rs.first[i]; r <= rs.last[i]; r++ {
			rs.start[r+1]++
			total++
		}
	}
	for r := range starts {
		rs.start[r+1] += rs.start[r]
	}
	rs.objs, rs.q, rs.qmax, rs.at = make([]int, total), make([]int, total), make([]int, total), make([]int, total)
	fill := slices.Clone(rs.start[:len(starts)])
	for _, i := range byRank {
		for r := rs.first[i]; r <= rs.last[i]; r++ {
			rs.at[rs.atStart[i]+r-rs.first[i]] = fill[r] - rs.start[r]
			rs.objs[fill[r]] = i
			fill[r]++
		}
	}
	for r := range starts {
		row, q, qmax := rs.row(r)
		for k, i := range row {
			q[k] = p.objs[i].hi - p.objs[i].lo
			qmax[k] = q[k]
			if k > 0 {
				q[k] += q[k-1] + gap(row[k-1], i)
				qmax[k] = max(qmax[k-1], q[k])
			}
		}
	}
	return rs
}

// apply moves everything by d cells, and reports whether anything moved
func (p *compactPass) apply(d []int) bool {
	l, cell := p.l, p.cell
	moved := false
	shift := func(item int) Length {
		k := d[p.varOf[item]]
		moved = moved || k != 0
		return Length(k) * cell.X
	}
	// a point goes in the middle of its cell, as text draws it, and the
	// ends of its edges follow; snap is how far past its whole cells. A
	// box moves by whole cells, so that both its sides round as before.
	// A point in line with a box that it moves with, as in a rank, stays
	// in line with it.
	snap := make([]Length, len(l.Nodes))
	boxAt := map[[2]Length]bool{} // the var and center of every box
	for i, box := range l.Nodes {
		if box.Shape != PointShape {
			boxAt[[2]Length{Length(p.varOf[i]), box.Center.X}] = true
		}
	}
	for i := range l.Nodes {
		box := &l.Nodes[i]
		at := box.Center.X + shift(i)
		if box.Shape == PointShape && !boxAt[[2]Length{Length(p.varOf[i]), box.Center.X}] {
			box.Center.X = Length(p.objs[i].lo+d[p.varOf[i]]) * cell.X
		} else {
			box.Center.X = at // its sides round to the cells they did
		}
		snap[i] = box.Center.X - at
	}
	// a run across the axis goes to the middle of its cell, where text
	// draws it: off the middle, runs in neighboring cells can be a hair
	// apart, which reads as overlapping where the diagnostics measure.
	// The end of a run along it stays on the side of its node.
	for e, items := range p.pointItem {
		path := l.Edges[e].Path
		edge := l.Graph.Edges[e]
		for i, item := range items {
			switch {
			case i > 0 && items[i-1] == item || i+1 < len(items) && items[i+1] == item:
				path[i].X = Length(p.col(path[i].X)+d[p.varOf[item]]) * cell.X
			case i == 0:
				path[i].X += shift(item) + snap[l.nodeIndex[edge.From]]
			case i == len(items)-1:
				path[i].X += shift(item) + snap[l.nodeIndex[edge.To]]
			default:
				path[i].X += shift(item)
			}
		}
	}
	// labels go on the grid as text draws them, so that their boxes keep
	// off the lines in neighboring cells where the diagnostics measure:
	// the left side just past where its first cell starts, or the middle
	// row centered on its cell
	for _, lb := range p.labels {
		at := &l.Edges[lb.edge]
		center := []*Vector{&at.LabelCenter, &at.HeadLabelCenter, &at.TailLabelCenter}[lb.which]
		size := []Vector{at.LabelSize, at.HeadLabelSize, at.TailLabelSize}[lb.which]
		k := lb.at + d[p.varOf[lb.item]]
		moved = moved || d[p.varOf[lb.item]] != 0
		if p.flip {
			center.X = Length(k) * cell.X
		} else {
			center.X = (Length(k)-0.49)*cell.X + size.X/2
		}
	}
	for i, s := range p.sides {
		l.Clusters[i].TopLeft.X += shift(p.objs[s[0]].item)
		l.Clusters[i].BottomRight.X += shift(p.objs[s[1]].item)
	}
	return moved
}
