package layout

import (
	"slices"

	"github.com/loov/layout/internal/draw"
	"github.com/loov/layout/internal/hier"
)

// resolveFields leaves the ends of edges at fields that their nodes don't
// have free, see Edge.FromField, as well as those of loops
func (graph *lgraph) resolveFields() {
	has := func(node *lnode, field string) bool {
		if field == "" || node.Shape != Record {
			return false
		}
		return draw.ParseRecord(node.DefaultLabel(), graph.RankDir.Sideways()).Field(field) != nil
	}

	for _, edge := range graph.Edges {
		if edge.loop() || !has(edge.From, edge.FromField) {
			edge.FromField = ""
		}
		if edge.loop() || !has(edge.To, edge.ToField) {
			edge.ToField = ""
		}
	}
}

// fieldOffset returns where the middle of the field of a record node is
// from the middle of the node, in the frame that hierarchical lays the
// graph out in, ranks top to bottom; false when it has no such field
func (graph *lgraph) fieldOffset(node *lnode, field string) (Vector, bool) {
	// the record as it is drawn, in the frame of the drawing
	size := node.Radius.Scale(2)
	side := graph.RankDir.Sideways()
	if side {
		size.X, size.Y = size.Y, size.X
	}
	rec := draw.LayoutRecord(node.DefaultLabel(), side, float64(size.X), float64(size.Y),
		float64(graph.LineHeight), float64(node.FontSize), graph.lineWidth(node.FontName, node.FontSize))
	f := rec.Field(field)
	if f == nil {
		return Vector{}, false
	}
	x, y := f.Center()
	d := Vector{Length(x) - size.X/2, Length(y) - size.Y/2}

	// into the frame, see the transform back in hierarchical
	switch graph.RankDir {
	case BottomToTop:
		d.Y = -d.Y
	case LeftToRight:
		d.X, d.Y = d.Y, d.X
	case RightToLeft:
		d.X, d.Y = d.Y, -d.X
	}

	return d, true
}

// fieldEnd returns where an edge end at the field of a record node goes,
// coming from next: on the side of the node facing next, across from the
// middle of the field
func (graph *lgraph) fieldEnd(node *lnode, field string, next Vector) Vector {
	d, ok := graph.fieldOffset(node, field)
	if !ok {
		return node.Boundary(next)
	}
	y := node.Top()
	if next.Y > node.Center.Y {
		y = node.Bottom()
	}
	return Vector{node.Center.X + d.X, y}
}

// fieldPorts tells ordering where the edges at fields end along their
// nodes, see hier.Graph.Ports, so that the nodes they lead to go in the
// order of the fields
func (c *hierComponent) fieldPorts() {
	// along, the part of the width of node across from field
	along := func(node *lnode, field string) float32 {
		d, ok := c.graphdef.fieldOffset(node, field)
		if !ok || node.Radius.X <= 0 {
			return 0
		}
		return max(-0.5, min(0.5, float32(d.X/(2*node.Radius.X))))
	}

	for _, edge := range c.graphdef.Edges {
		if edge.FromField == "" && edge.ToField == "" {
			continue
		}
		e, reversed := c.hierEdge(edge)
		if e == nil || e.Flat {
			continue
		}
		atTop, atBottom := along(edge.From, edge.FromField), along(edge.To, edge.ToField)
		if reversed {
			atTop, atBottom = atBottom, atTop
		}

		// the first and last steps of the chain of the edge down the
		// ranks, through its virtual nodes
		first, last := e.To, e.From
		if len(e.Chain) > 0 {
			first, last = e.Chain[0], e.Chain[len(e.Chain)-1]
		}
		if c.graph.Ports == nil {
			c.graph.Ports = map[[2]hier.ID][2]float32{}
		}
		k := [2]hier.ID{e.From.ID, first.ID}
		p := c.graph.Ports[k]
		p[0] = atTop
		c.graph.Ports[k] = p
		k = [2]hier.ID{last.ID, e.To.ID}
		p = c.graph.Ports[k]
		p[1] = atBottom
		c.graph.Ports[k] = p
	}
}

// freeStart and freeEnd report whether the layout places the start and
// the end of the edge, which no compass port or field pins
func (edge *ledge) freeStart() bool { return edge.FromPort == CompassAuto && edge.FromField == "" }
func (edge *ledge) freeEnd() bool   { return edge.ToPort == CompassAuto && edge.ToField == "" }

// slideFieldEnds takes out the jog an orthogonal edge makes right after
// leaving a field of a record, when the run it jogs to still lies within
// the field: the edge then leaves the field there, off its middle, and runs
// straight. Compaction keeps ends where they leave their nodes, and an
// edge that can't run straight from the middle, such as with another
// edge's end below it, jogs less than a track aside, which reads as a
// slip rather than a turn. Only jogs shorter than track are taken out,
// so that ends stay in the middle of their fields otherwise.
func slideFieldEnds(l *Layout, track Length) {
	g := l.Graph
	pad := g.EdgePadding
	for e, edge := range g.Edges {
		at := &l.Edges[e]
		if !at.Merged.IsZero() || edge.Invisible {
			continue
		}
		if edge.FromField != "" {
			at.Path = slideFieldEnd(l, e, at.Path, edge.From, edge.FromField, track, pad)
		}
		if edge.ToField != "" {
			at.Path = slices.Clone(at.Path)
			slices.Reverse(at.Path)
			at.Path = slideFieldEnd(l, e, at.Path, edge.To, edge.ToField, track, pad)
			slices.Reverse(at.Path)
		}
	}
}

// slideFieldEnd returns path, which starts at the field of node, with the
// jog after its first run taken out, see slideFieldEnds
func slideFieldEnd(l *Layout, e int, path Path, node *Node, field string, track, pad Length) Path {
	if len(path) < 4 {
		return path
	}
	p0, p1, p2, p3 := path[0], path[1], path[2], path[3]
	stub, jog, run := p1.Sub(p0), p2.Sub(p1), p3.Sub(p2)
	// a stub and a run on one line across the side, the jog along it
	if stub.Dot(jog) != 0 || stub.Cross(run) != 0 || stub.Dot(run) <= 0 || jog.IsZero() || jog.Len() >= track {
		return path
	}

	// the new end within the field, not on its border
	box := l.Node(node)
	rec := layoutRecord(l.Graph, node, box)
	f := rec.Field(field)
	if f == nil {
		return path
	}
	origin := box.Center.Sub(box.Size.Scale(0.5))
	end := p0.Add(jog)
	inset := min(pad, Length(f.X1-f.X0)/4, Length(f.Y1-f.Y0)/4)
	if jog.Y == 0 && (end.X < origin.X+Length(f.X0)+inset || end.X > origin.X+Length(f.X1)-inset) ||
		jog.X == 0 && (end.Y < origin.Y+Length(f.Y0)+inset || end.Y > origin.Y+Length(f.Y1)-inset) {
		return path
	}

	// the run that grows past the jog keeps off everything else
	lo, hi := end, p2
	clear := rect{Min: Vector{min(lo.X, hi.X), min(lo.Y, hi.Y)}, Max: Vector{max(lo.X, hi.X), max(lo.Y, hi.Y)}}.grow(pad / 2)
	for k := range l.Edges {
		other := &l.Edges[k]
		if k != e {
			for a, b := range other.Path.Segments() {
				if clear.hitBy(a, b) {
					return path
				}
			}
		}
		for _, lb := range []struct{ c, s Vector }{{other.LabelCenter, other.LabelSize}, {other.HeadLabelCenter, other.HeadLabelSize}, {other.TailLabelCenter, other.TailLabelSize}} {
			if !lb.s.IsZero() && clear.overlaps(rect{Min: lb.c.Sub(lb.s.Scale(0.5)), Max: lb.c.Add(lb.s.Scale(0.5))}, 0) {
				return path
			}
		}
	}

	return append(Path{end}, path[3:]...)
}

// layoutRecord lays out the fields of a record node as drawn, in the
// frame of the drawing, as format/svg does
func layoutRecord(graph *Graph, node *Node, box NodeBox) *draw.Record {
	return draw.LayoutRecord(box.Label, graph.RankDir.Sideways(), float64(box.Size.X), float64(box.Size.Y),
		float64(graph.LineHeight), float64(box.FontSize), graph.lineWidth(node.FontName, box.FontSize))
}
