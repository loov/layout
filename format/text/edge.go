package text

import (
	"github.com/loov/layout"
	"math"
	"slices"
)

// marker draws an edge end marker pointing in dir at the end of the edge
// in cell end. It sits in the gap before the node when the run there is
// straight, else on the node border. It reports whether style has a
// marker; styles without a marker of their own draw a normal arrowhead.
func (c *canvas) marker(style layout.Arrow, dir uint8, end [2]int) bool {
	if style == layout.ArrowDefault || style == layout.ArrowNone {
		return false
	}
	r := arrow[dir]
	switch style {
	case layout.ArrowVee:
		r = vee[dir]
	case layout.ArrowDot:
		r = '●'
	case layout.ArrowODot:
		r = '○'
	}
	if before := c.at(end[0]-dx(dir), end[1]-dy(dir)); before != nil && before.lines == dir|opposite(dir) {
		end = [2]int{end[0] - dx(dir), end[1] - dy(dir)}
	}
	c.set(end[0], end[1], r)
	if p := c.at(end[0], end[1]); p != nil {
		p.solid = true // later runs don't erase it
	}
	return true
}

// arrival returns the direction of the leg of the run between cells a
// and b that reaches b, pointing at b. walk draws the horizontal leg
// first, so from b when the run starts at b. Within a cell, it is the
// direction of the segment from a to b in graph coordinates, p to q.
func arrival(a, b [2]int, startsAtB bool, p, q layout.Vector) uint8 {
	horizontal := a[0] != b[0] && (startsAtB || a[1] == b[1])
	switch {
	case horizontal && b[0] > a[0]:
		return right
	case horizontal:
		return left
	case b[1] > a[1]:
		return down
	case b[1] < a[1]:
		return up
	}
	switch dx, dy := q.X-p.X, q.Y-p.Y; {
	case dy < 0 && -dy >= absLength(dx):
		return up
	case dx > 0 && dx > absLength(dy):
		return right
	case dx < 0 && -dx > absLength(dy):
		return left
	}
	return down
}

// edgeCells returns the cells of the path of an edge, with its ends on
// the borders of the node boxes
func (c *canvas) edgeCells(edge *layout.Edge, path []layout.Vector) [][2]int {
	if len(path) < 2 {
		return nil
	}
	cells := make([][2]int, len(path))
	for i, p := range path {
		cells[i] = [2]int{c.col(p.X), c.row(p.Y)}
	}
	last := len(cells) - 1
	cells[0] = c.border(cells[0], cells[1], edge.From)
	cells[last] = c.border(cells[last], cells[last-1], edge.To)
	c.atDot(cells, 0, 1, 2, edge.From)
	c.atDot(cells, last, last-1, last-2, edge.To)
	if edge.From == edge.To && c.loops[edge.From] == 1 {
		c.centerLoop(cells, path, edge.From)
	}
	return c.across(cells, edge.From, edge.To)
}

// centerLoop places the ends of the only self-loop of node symmetrically
// about the middle of its box, as the layout does. Rounding each end to
// a cell on its own can leave the loop off-center and a cell narrower,
// depending on where the node lands.
func (c *canvas) centerLoop(cells [][2]int, path []layout.Vector, node *layout.Node) {
	if len(cells) != 4 {
		return // loops with ports go around the node
	}
	b := c.boxes[node]
	for along := range 2 {
		across := 1 - along
		if cells[0][across] != cells[3][across] || cells[0][along] == cells[3][along] {
			continue // the ends are not on one side
		}
		span := float64((path[3].X - path[0].X) / c.cellW)
		if along == 1 {
			span = float64((path[3].Y - path[0].Y) / c.cellH)
		}
		// the span nearest the layout's that splits evenly about the middle
		sum := b[along] + b[along+2]
		odd := (sum%2 + 2) % 2
		n := 2*int(math.Round((math.Abs(span)-float64(odd))/2)) + odd
		n = max(n, 2-odd)
		if n > b[along+2]-b[along]-2 {
			return // no room inside the corners
		}
		lo, hi := (sum-n)/2, (sum+n)/2
		if span < 0 {
			lo, hi = hi, lo
		}
		cells[0][along], cells[1][along] = lo, lo
		cells[2][along], cells[3][along] = hi, hi
		return
	}
}

// across adds bends to diagonal end segments, which walk draws as a
// horizontal then a vertical leg, so that they leave and reach the boxes
// across their borders instead of along them
func (c *canvas) across(cells [][2]int, from, to *layout.Node) [][2]int {
	// the axis an end crosses the border of its box on, from where it
	// lies beside or above or below the box: 0 across a side, 1 across
	// the top or bottom, -1 off a corner or at a dot
	axis := func(end [2]int, node *layout.Node) int {
		b := c.boxes[node]
		inX, inY := end[0] > b[0] && end[0] < b[2], end[1] > b[1] && end[1] < b[3]
		switch {
		case inY && !inX:
			return 0
		case inX && !inY:
			return 1
		}
		return -1
	}
	bends := func(a, b [2]int, first, last int) [][2]int {
		if a[0] == b[0] || a[1] == b[1] {
			return nil
		}
		switch {
		case first == 1 && last == 1:
			mid := (a[1] + b[1]) / 2
			return [][2]int{{a[0], mid}, {b[0], mid}}
		case first == 0 && last == 0:
			mid := (a[0] + b[0]) / 2
			return [][2]int{{mid, a[1]}, {mid, b[1]}}
		case first == 1 || last == 0:
			return [][2]int{{a[0], b[1]}} // vertical, then horizontal
		}
		return nil
	}
	last := len(cells) - 1
	start, end := axis(cells[0], from), axis(cells[last], to)
	if last == 1 {
		return slices.Concat(cells[:1], bends(cells[0], cells[1], start, end), cells[1:])
	}
	return slices.Concat(cells[:1], bends(cells[0], cells[1], start, -1), cells[1:last],
		bends(cells[last-1], cells[last], -1, end), cells[last:])
}

// atDot starts the edge end cells[i] at a dot node in the dot's cell and,
// when the edge turns at its first bend cells[j], turns it at the dot
// instead, so that edges fanning out of a dot leave it on separate sides
// rather than sharing a run
func (c *canvas) atDot(cells [][2]int, i, j, k int, node *layout.Node) {
	if c.l.Node(node).Shape != layout.PointShape {
		return
	}
	b := c.boxes[node]
	from := cells[i]
	cells[i] = [2]int{b[0], b[1]}
	if k < 0 || k >= len(cells) {
		return
	}
	// keep the first segment straight from the dot's cell
	for axis := range 2 {
		if from[axis] == cells[j][axis] {
			cells[j][axis] = cells[i][axis]
		}
	}
	at, next := cells[i], cells[k]
	switch bend := cells[j]; {
	case at[1] == bend[1] && bend[0] == next[0] && next[1] != at[1]:
		cells[j] = [2]int{at[0], next[1]} // across first, then along
	case at[0] == bend[0] && bend[1] == next[1] && next[0] != at[0]:
		cells[j] = [2]int{next[0], at[1]}
	}
}

// drawEdge draws an edge along its cells, with its end markers
func (c *canvas) drawEdge(edge *layout.Edge, path []layout.Vector, cells [][2]int) {
	if len(cells) < 2 {
		return
	}
	last := len(cells) - 1
	c.ids++
	// merged edges share an id, so that they draw as one without overlaps
	id, merged := c.merged[edge]
	if !merged {
		id = c.ids
	}
	c.drawn[edge] = id
	c.pen = pen{
		ink:    c.color(rgb(edge.LineColor)),
		dashed: edge.LineStyle == layout.Dashed || edge.LineStyle == layout.Dotted,
		edge:   id,
	}
	for i := 0; i+1 < len(cells); i++ {
		c.walk(cells[i][0], cells[i][1], cells[i+1][0], cells[i+1][1])
	}
	head := edge.ArrowHead
	if head == layout.ArrowDefault && edge.Directed {
		head = layout.ArrowNormal
	}
	// merged edges share their ends, which get one marker
	if !merged || !c.ended[cells[last]] {
		if !c.marker(head, arrival(cells[last-1], cells[last], false, path[len(path)-2], path[len(path)-1]), cells[last]) {
			c.join(cells[last], edge.To)
		}
	}
	if !merged || !c.ended[cells[0]] {
		if !c.marker(edge.ArrowTail, arrival(cells[1], cells[0], true, path[1], path[0]), cells[0]) {
			c.join(cells[0], edge.From)
		}
	}
	if merged {
		c.ended[cells[0]], c.ended[cells[last]] = true, true
	}
}

// border moves an edge end inside the box of node out to the border it
// faces, coming from next: every shape is drawn as a box, so ends on a
// rounder outline or a box widened for its label fall inside
func (c *canvas) border(end, next [2]int, node *layout.Node) [2]int {
	b := c.boxes[node]
	if end[0] > b[0] && end[0] < b[2] && end[1] > b[1] && end[1] < b[3] {
		switch {
		case next[1] < end[1]:
			end[1] = b[1]
		case next[1] > end[1]:
			end[1] = b[3]
		case next[0] < end[0]:
			end[0] = b[0]
		case next[0] > end[0]:
			end[0] = b[2]
		}
	}
	return end
}

// join draws the box border of node at an edge end without a marker as a
// junction, so that the edge visibly leaves the node
func (c *canvas) join(end [2]int, node *layout.Node) {
	if shape := c.l.Node(node).Shape; shape == layout.None || shape == layout.PointShape || node.Invisible {
		return // no border to join
	}
	p := c.at(end[0], end[1])
	if p == nil {
		return // pinned nodes can lie outside the canvas
	}
	// whether the cell at dx, dy from the end has an arm toward it
	arm := func(dx, dy int, dir uint8) bool {
		q := c.at(end[0]+dx, end[1]+dy)
		return q != nil && q.lines&dir != 0
	}
	b := c.boxes[node]
	// junctions on the left, right, top and bottom side
	joins := []rune("┤├┴┬")
	if node.Peripheries > 1 {
		joins = []rune("╢╟╨╥")
	}
	switch side := end[1] > b[1] && end[1] < b[3]; {
	case side && end[0] == b[0] && arm(-1, 0, right):
		p.r = joins[0]
	case side && end[0] == b[2] && arm(1, 0, left):
		p.r = joins[1]
	case end[0] <= b[0] || end[0] >= b[2]:
		// corners and outside the box
	case end[1] == b[1] && arm(0, -1, down):
		p.r = joins[2]
	case end[1] == b[3] && arm(0, 1, up):
		p.r = joins[3]
	}
}

// mergedEdges returns an id for the edges that the layout merged, the
// same for those in a group. An edge merges at its start or its end, not
// both. The ids are negative, apart from those the canvas counts up.
//
// Edges also merge at a point, a dot where they meet, as they have to
// leave it along one run: at the point they start at, or else the one
// they end at.
func mergedEdges(l *layout.Layout) map[*layout.Edge]edgeID {
	ids := map[*layout.Edge]edgeID{}
	last := 0
	for i, edge := range l.Graph.Edges {
		if m := l.Edges[i].Merged; m != [2]int{} {
			ids[edge] = -edgeID(max(m[0], m[1]))
			last = max(last, m[0], m[1])
		}
	}
	points := map[*layout.Node]edgeID{}
	point := func(node *layout.Node) bool { return l.Node(node).Shape == layout.PointShape }
	for _, edge := range l.Graph.Edges {
		if _, ok := ids[edge]; ok || edge.From == edge.To {
			continue
		}
		at := edge.From
		if !point(at) {
			at = edge.To
		}
		if !point(at) {
			continue
		}
		if _, ok := points[at]; !ok {
			last++
			points[at] = -edgeID(last)
		}
		ids[edge] = points[at]
	}
	return ids
}

func absLength(v layout.Length) layout.Length {
	if v < 0 {
		return -v
	}
	return v
}
