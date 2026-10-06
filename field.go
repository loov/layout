package layout

import (
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
		return draw.ParseRecord(node.DefaultLabel(), sideways(graph.RankDir)).Field(field) != nil
	}
	for _, edge := range graph.Edges {
		if edge.From == edge.To || !has(edge.From, edge.FromField) {
			edge.FromField = ""
		}
		if edge.From == edge.To || !has(edge.To, edge.ToField) {
			edge.ToField = ""
		}
	}
}

// fieldOffset returns where the middle of the field of a record node is
// from the middle of the node, in the frame that hierarchical lays the
// graph out in, ranks top to bottom; false when it has no such field
func (graph *lgraph) fieldOffset(node *lnode, field string) (Vector, bool) {
	// the record as it is drawn, in the frame of the drawing
	size := Vector{2 * node.Radius.X, 2 * node.Radius.Y}
	side := sideways(graph.RankDir)
	if side {
		size.X, size.Y = size.Y, size.X
	}
	rec := draw.LayoutRecord(node.DefaultLabel(), side, float64(size.X), float64(size.Y),
		float64(graph.LineHeight), float64(node.FontSize), graph.lineWidth(node.FontName, node.FontSize))
	f := rec.Field(field)
	if f == nil {
		return Vector{}, false
	}
	d := Vector{Length(f.X0+f.X1)/2 - size.X/2, Length(f.Y0+f.Y1)/2 - size.Y/2}
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
		top, bottom := c.graph.Nodes[c.nodes[edge.From]], c.graph.Nodes[c.nodes[edge.To]]
		atTop, atBottom := along(edge.From, edge.FromField), along(edge.To, edge.ToField)
		if top.Rank > bottom.Rank {
			top, bottom, atTop, atBottom = bottom, top, atBottom, atTop
		}
		if top.Rank == bottom.Rank {
			continue
		}
		// the first and last steps of the chain of the edge down the
		// ranks, through its virtual nodes
		for _, first := range top.Out {
			end := first
			for end.Virtual && len(end.Out) > 0 {
				end = end.Out[0]
			}
			if end != bottom {
				continue
			}
			last := top
			for n := first; n != bottom; n = n.Out[0] {
				last = n
			}
			if c.graph.Ports == nil {
				c.graph.Ports = map[[2]hier.ID][2]float32{}
			}
			k := [2]hier.ID{top.ID, first.ID}
			p := c.graph.Ports[k]
			p[0] = atTop
			c.graph.Ports[k] = p
			k = [2]hier.ID{last.ID, bottom.ID}
			p = c.graph.Ports[k]
			p[1] = atBottom
			c.graph.Ports[k] = p
			break
		}
	}
}

// freeStart and freeEnd report whether the layout places the start and
// the end of the edge, which no compass port or field pins
func (edge *ledge) freeStart() bool { return edge.FromPort == CompassAuto && edge.FromField == "" }
func (edge *ledge) freeEnd() bool   { return edge.ToPort == CompassAuto && edge.ToField == "" }
