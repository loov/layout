package text

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// clusterBox returns the cells of the frame of the cluster at index i
func (c *canvas) clusterBox(i int) [4]int {
	cluster, box := c.l.Graph.Clusters[i], c.l.Clusters[i]
	x0, y0 := c.col(box.TopLeft.X), c.row(box.TopLeft.Y)
	x1, y1 := c.col(box.BottomRight.X), c.row(box.BottomRight.Y)
	if cluster.Label != "" {
		// the label fits, spaced, between the corners; layouts not made
		// for text can fall a few cells short
		x1 = max(x1, x0+clusterLabelWidth(cluster))
	}
	return [4]int{x0, y0, x1, y1}
}

// drawCluster draws the dashed frame and fill of the cluster at index i
func (c *canvas) drawCluster(i int) {
	cluster := c.l.Graph.Clusters[i]
	b := c.clusterBox(i)
	x0, y0, x1, y1 := b[0], b[1], b[2], b[3]
	c.ids++
	c.pen = pen{ink: rgb(cluster.LineColor), dashed: true, edge: c.ids, frame: true}
	c.fill(x0, y0, x1, y1, rgb(cluster.FillColor))
	c.frame(x0, y0, x1, y1)
}

// peripheryGap matches the layout's distance between a node's outlines
const peripheryGap = 4 * layout.Point

// nodeBox returns the cells of the box of a node, which holds its label
// with a space on either side; a dot takes one cell
func (c *canvas) nodeBox(node *layout.Node) [4]int {
	box := c.l.Node(node)
	if box.Shape == layout.PointShape {
		x, y := c.col(box.Center.X), c.row(box.Center.Y)
		return [4]int{x, y, x, y}
	}
	// a double outline takes the cells of a single one, so the room the
	// layout reserves for extra outlines stays outside the box
	inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
	x0, y0 := c.col(box.Left()+inset), c.row(box.Top()+inset)
	x1, y1 := c.col(box.Right()-inset), c.row(box.Bottom()-inset)
	label := draw.PlainLabel(box.Label)
	w, h := draw.LabelBox(label)
	// the box must hold the label and have distinct edges
	if box.Shape != layout.Record {
		x1 = max(x1, x0+w)
		// an odd number of spare cells can't be split evenly around the
		// label, give one back on a side that no edge ends next to on the
		// top or bottom, where spreadSides puts them; ends on the sides
		// move onto them
		if spare := x1 - x0 - 1 - draw.TextColumns(label); spare >= 3 && spare%2 == 1 {
			type end struct {
				exact    float64
				straight bool
			}
			var top, bottom []end
			add := func(path []layout.Vector) {
				p := path[0]
				// straight on past the first bend, see spreadSides
				straight := len(path) < 3 || absLength(path[2].X-path[1].X) < 0.01
				e := end{float64((p.X - c.origin.X) / c.cellW), straight}
				switch r := c.row(p.Y); {
				case r <= y0:
					top = append(top, e)
				case r >= y1:
					bottom = append(bottom, e)
				}
			}
			for i, edge := range c.l.Graph.Edges {
				path := c.l.Edges[i].Path
				if len(path) == 0 {
					continue
				}
				if edge.From == node {
					add(path)
				}
				if edge.To == node {
					back := slices.Clone(path)
					slices.Reverse(back)
					add(back)
				}
			}
			lo, hi := x1, x0 // the columns of the edge ends
			for _, ends := range [][]end{top, bottom} {
				slices.SortFunc(ends, func(a, b end) int { return cmp.Compare(a.exact, b.exact) })
				cells := make([]int, len(ends))
				exact, fixed := make([]float64, len(ends)), make([]bool, len(ends))
				for n, e := range ends {
					cells[n], exact[n], fixed[n] = int(e.exact+0.5), e.exact, e.straight
				}
				packEnds(cells, exact, fixed)
				for _, x := range cells {
					lo, hi = min(lo, x), max(hi, x)
				}
			}
			switch {
			case hi < x1-1:
				x1--
			case lo > x0+1:
				x0++
			}
		}
	}
	x1 = max(x1, x0+2)
	y1 = max(y1, y0+2)
	if box.Shape == layout.Record {
		y1 = max(y1, y0+draw.RecordRows(layoutRecord(c.l.Graph, node, box))+1)
	} else {
		y1 = max(y1, y0+h)
	}
	return [4]int{x0, y0, x1, y1}
}

// drawNode draws a node in its box, see nodeBox, and marks the box solid
// so that edges don't draw over it; an invisible node only keeps its box
// clear
func (c *canvas) drawNode(graph *layout.Graph, node *layout.Node) {
	box := c.l.Node(node)
	b := c.boxes[node]
	x0, y0, x1, y1 := b[0], b[1], b[2], b[3]
	if box.Shape == layout.PointShape {
		if node.Invisible {
			return
		}
		c.pen = pen{ink: rgb(node.LineColor)}
		if node.FillColor != nil {
			c.pen.ink = rgb(node.FillColor)
		}
		c.set(x0, y0, '●')
		if p := c.at(x0, y0); p != nil {
			p.solid, p.node = true, c.nodes[node]
		}
		return
	}
	c.pen = pen{ink: rgb(node.LineColor), font: rgb(node.FontColor)}
	if !node.Invisible {
		c.fill(x0, y0, x1, y1, rgb(node.FillColor))
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, ' ')
			if p := c.at(x, y); p != nil {
				p.solid, p.node = true, c.nodes[node]
			}
			if x > x0 && x < x1 && y > y0 && y < y1 {
				c.hold(x, y) // blanks inside a box are part of it
			}
		}
	}
	if node.Invisible {
		return
	}
	// rounded corners for round shapes, Auto included as it is drawn as
	// an ellipse; there are no rounded double corners
	style := "┌┐└┘─│"
	switch {
	case box.Shape == layout.None:
		style = "      "
	case node.Peripheries > 1:
		style = "╔╗╚╝═║" // like double circles, of any shape
	case box.Shape == layout.Circle, box.Shape == layout.Ellipse, box.Shape == layout.Auto:
		style = "╭╮╰╯─│"
	}
	c.rect(x0, y0, x1, y1, style)
	if box.Shape == layout.Record {
		rec := layoutRecord(graph, node, box)
		// fields at their share of the rows, which can be more than the
		// node rounds to
		inside := func(v layout.Length) int {
			share := float64((v - box.Top()) / (box.Bottom() - box.Top()))
			return y0 + int(math.Round(share*float64(y1-y0)))
		}
		c.evenRecord(rec, box.TopLeft())
		c.record(rec, box.TopLeft(), c.col, inside)
		return
	}
	lines := strings.Split(draw.PlainLabel(box.Label), "\n")
	top := (y0 + y1 + 1 - len(lines)) / 2
	if graph.PackEdgeEnds && c.sideways() {
		top = y0 + 1 // with the main path, along the first row
	}
	for i, line := range lines {
		c.text(centered(x0, x1, line), top+i, line)
	}
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

// spreadSides gives every edge end on a side of a box a row, or on the
// top or bottom a column, of its own between the corners, as close to
// where it rounded as the others allow. The layout spreads ends apart,
// but rounds them to cells independently of the box, so ends could share
// a cell or land on a corner. An end whose run goes on straight keeps
// its place if it can; a moved end takes the bend before it along.
func (c *canvas) spreadSides(edges []*layout.Edge, paths [][][2]int) {
	type end struct {
		path   [][2]int
		i, j   int     // the end and the bend before it
		fixed  bool    // the run goes on straight past the bend
		toward int     // where the edge heads past the bend, along the side
		group  int     // the merged ends there, see layout.EdgePath.Merged
		exact  float64 // where the layout ends the edge along the side, in cells
		marker bool    // drawn with an arrowhead or another mark, not a junction

		also []end // further edges of a shared start, which follow it
	}
	type side struct {
		node  *layout.Node
		along int  // the axis along the side: 1 for left and right
		after bool // right or bottom
	}
	sides := map[side][]end{}
	// merged ends on a side move as one, led by an end that goes on
	// straight when there is one, as that stays; leads holds where each
	// group's lead is in sides
	type group struct {
		side side
		id   int
	}
	leads := map[group]int{}
	for k, path := range paths {
		if len(path) < 2 {
			continue
		}
		last := len(path) - 1
		for _, e := range []struct {
			i, j, k int
			node    *layout.Node
		}{{0, 1, 2, edges[k].From}, {last, last - 1, last - 2, edges[k].To}} {
			b := c.boxes[e.node]
			at, bend := path[e.i], path[e.j]
			for along := range 2 {
				across := 1 - along
				after := bend[across] > b[across+2]
				if b[along+2]-b[along] < 2 || at[along] != bend[along] || at[along] < b[along] || at[along] > b[along+2] ||
					!after && bend[across] >= b[across] {
					continue // not straight into the side from beyond it
				}
				// onto the side, also where a rounder outline curves inside
				path[e.i][across] = b[across]
				if after {
					path[e.i][across] = b[across+2]
				}
				end := end{path: path, i: e.i, j: e.j, fixed: true, toward: at[along]}
				if e.k >= 0 && e.k < len(path) {
					end.fixed = path[e.k][along] == bend[along]
					end.toward = path[e.k][along]
				}
				end.group = c.l.Edges[k].Merged[min(e.i, 1)] // the start, else the end
				mark := edges[k].ArrowTail
				if e.i != 0 {
					mark = edges[k].ArrowHead
					if mark == layout.ArrowDefault && edges[k].Directed {
						mark = layout.ArrowNormal
					}
				}
				end.marker = mark != layout.ArrowDefault && mark != layout.ArrowNone
				p := c.l.Edges[k].Path[0]
				if e.i != 0 {
					p = c.l.Edges[k].Path[len(c.l.Edges[k].Path)-1]
				}
				end.exact = float64((p.X - c.origin.X) / c.cellW)
				if along == 1 {
					end.exact = float64((p.Y - c.origin.Y) / c.cellH)
				}
				key := side{e.node, along, after}
				if n, ok := leads[group{key, end.group}]; ok {
					lead := &sides[key][n]
					if end.fixed && !lead.fixed {
						end.also, lead.also = lead.also, nil
						*lead, end = end, *lead
					}
					lead.also = append(lead.also, end)
					break
				}
				if end.group != 0 {
					leads[group{key, end.group}] = len(sides[key])
				}
				sides[key] = append(sides[key], end)
				break
			}
		}
	}
	// sides in the order of the nodes, as an edge can end on two of them
	// and moving one end can move the other
	keys := slices.Collect(maps.Keys(sides))
	order := map[*layout.Node]int{}
	for i, node := range c.l.Graph.Nodes {
		order[node] = i
	}
	slices.SortFunc(keys, func(a, b side) int {
		return cmp.Or(cmp.Compare(order[a.node], order[b.node]), cmp.Compare(a.along, b.along), cmp.Compare(boolInt(a.after), boolInt(b.after)))
	})
	for _, key := range keys {
		ends := sides[key]
		b := c.boxes[key.node]
		lo, hi := b[key.along]+1, b[key.along+2]-1
		if len(ends) > hi-lo+1 {
			continue // no room; keep them as they are
		}
		slices.SortStableFunc(ends, func(a, b end) int {
			return cmp.Or(cmp.Compare(a.path[a.i][key.along], b.path[b.i][key.along]), cmp.Compare(a.toward, b.toward))
		})
		want := make([]int, len(ends))
		exact := make([]float64, len(ends))
		fixed := make([]bool, len(ends))
		for n, e := range ends {
			want[n], exact[n], fixed[n] = e.path[e.i][key.along], e.exact, e.fixed
		}
		packEnds(want, exact, fixed)
		// an end that turns toward a place along the side goes straight
		// there instead, past no other end; an arrowhead may go as far as
		// a corner, see below
		for n, e := range ends {
			if e.fixed || e.toward < lo-1 || e.toward > hi+1 {
				continue
			}
			past := slices.ContainsFunc(want, func(w int) bool {
				return w != want[n] && (w-want[n])*(w-e.toward) <= 0
			})
			if !past && e.toward >= lo && e.toward <= hi {
				want[n], fixed[n] = e.toward, true
			}
		}
		// with spread, a cell between ends where the side has room, as
		// balanced as the ends were
		rows := spreadRows(want, fixed, lo, hi, 1)
		if c.spread && 2*len(want)-1 <= hi-lo+1 {
			wide := spreadRows(want, fixed, lo, hi, 2)
			moved := false
			for n := range want {
				moved = moved || fixed[n] && wide[n] != want[n]
			}
			if !moved {
				rows = wide
			}
		}
		// of several ends, the first or last goes on to the corner when
		// its arrowhead lines up with where the edge comes from, so the
		// edge runs straight in beside the box; a lone end there would
		// read as reaching for the corner
		for _, n := range []int{0, len(ends) - 1} {
			e := ends[n]
			if len(ends) > 1 && !e.fixed && e.marker && e.group == 0 && (n == 0 && e.toward == lo-1 || n == len(ends)-1 && e.toward == hi+1) {
				rows[n] = e.toward
			}
		}
		for n, at := range rows {
			e := ends[n]
			e.path[e.i][key.along], e.path[e.j][key.along] = at, at
			for _, f := range e.also {
				f.path[f.i][key.along], f.path[f.j][key.along] = at, at
			}
		}
	}
}

// packEnds moves the ends along a side, in order at cells, no further
// from the end before than their distance at the exact cells where the
// layout ends them, so that ends about a cell apart stay next to each
// other however they round. Fixed ends, which go on straight, stay.
func packEnds(cells []int, exact []float64, fixed []bool) {
	for n := 1; n < len(cells); n++ {
		if !fixed[n] {
			cells[n] = min(cells[n], cells[n-1]+max(1, int(math.Round(exact[n]-exact[n-1]))))
		}
	}
}

// spreadRows moves the ordered rows, or columns, want as little as
// possible so that they are at least gap apart within [lo, hi], with fixed
// ones moving only when they must. Shifting row n by n*gap turns this into
// ordering, solved by pooling adjacent violators.
func spreadRows(want []int, fixed []bool, lo, hi, gap int) []int {
	type block struct {
		sum, weight float64
		n           int
	}
	var blocks []block
	for n, row := range want {
		weight := 1.0
		if fixed[n] {
			weight = 1e6
		}
		blocks = append(blocks, block{float64(row-n*gap) * weight, weight, 1})
		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/a.weight <= b.sum/b.weight {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{a.sum + b.sum, a.weight + b.weight, a.n + b.n})
		}
	}
	rows := make([]int, 0, len(want))
	for _, b := range blocks {
		first := min(max(int(math.Round(b.sum/b.weight)), lo), hi-(len(want)-1)*gap)
		for range b.n {
			rows = append(rows, first+len(rows)*gap)
		}
	}
	return rows
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
		ink:    rgb(edge.LineColor),
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
	arm := func(dx, dy, dir int) bool {
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

// drawLabels writes edge and cluster labels over everything else; paths
// are the cells of the edges
func (c *canvas) drawLabels(graph *layout.Graph, paths [][][2]int) {
	for i, edge := range graph.Edges {
		if edge.Label != "" && !edge.Invisible {
			label, x, y := c.edgeLabel(i)
			lines := strings.Split(label, "\n")
			x, y = c.nearEdge(x, y, draw.TextColumns(label), len(lines), paths[i])
			c.pen = pen{font: rgb(edge.FontColor)}
			for k, line := range lines {
				c.text(x, y+k, line)
			}
			// the blanks around the label keep it beside its edge when
			// carving, see seams
			w := draw.TextColumns(label)
			for row := y - 1; row <= y+len(lines); row++ {
				for col := x - 1; col <= x+w; col++ {
					if p := c.at(col, row); p != nil && p.r == ' ' && !p.solid {
						p.glue = true
					}
				}
			}
		}
	}
	for i, cluster := range graph.Clusters {
		if cluster.Label == "" || cluster.Invisible {
			continue
		}
		// the label goes on the frame after carving, see frameLabels, at
		// the first place along the top, else along the bottom, where the
		// lines that cross the frame fall on spaces of it; carving keeps
		// those cells
		b := c.clusterBox(i)
		text := " " + clusterLabel(cluster) + " "
		var runes []rune
		for _, r := range text {
			switch {
			case draw.IsZeroWidth(r), unicode.IsControl(r):
			case draw.IsWide(r):
				runes = append(runes, r, covered)
			default:
				runes = append(runes, r)
			}
		}
		fits := func(x, y int) bool {
			for j, r := range runes {
				p := c.at(x+j, y)
				if p == nil || x+j >= b[2] || !strings.ContainsRune("┈┉", p.r) && !(r == ' ' && p.lines&(up|down) != 0) {
					return false
				}
			}
			return true
		}
		placed := false
		for _, y := range []int{b[1], b[3]} {
			for x := b[0] + 1; x < b[2] && !placed; x++ {
				if fits(x, y) {
					for j := range runes {
						c.at(x+j, y).need = len(runes)
					}
					c.at(x, y).label = i + 1
					placed = true
				}
			}
		}
		if placed {
			continue
		}
		c.pen = pen{}
		c.text(b[0]+1, b[1], text)
	}
}

// frameLabels writes the cluster labels at the cells marked for them on the
// carved grid, see drawLabels
func (c *canvas) frameLabels(grid [][]cell) {
	for _, row := range grid {
		for x := range row {
			id := row[x].label
			if id == 0 {
				continue
			}
			for _, r := range " " + clusterLabel(c.l.Graph.Clusters[id-1]) + " " {
				switch {
				case draw.IsZeroWidth(r), unicode.IsControl(r):
				case row[x].lines&(up|down) != 0:
					x++ // a line that crosses the frame, on a space, stays
				case draw.IsWide(r):
					row[x].r, row[x].fg, row[x+1].r = r, 0, covered
					x += 2
				default:
					row[x].r, row[x].fg = r, 0
					x++
				}
			}
		}
	}
}

// nearEdge moves a label w cells wide and h tall at x, y toward its edge,
// drawn along
// path, while a blank row or more than one blank column separates them
// and the cells it moves into are blank. The layout keeps labels an edge
// padding from their edge, which is a whole row in text, and rounding can
// leave a blank row between them.
func (c *canvas) nearEdge(x, y, w, h int, path [][2]int) (int, int) {
	var cells [][2]int
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		for x := a[0]; x != b[0]; x += sign(b[0] - a[0]) {
			cells = append(cells, [2]int{x, a[1]})
		}
		for y := a[1]; y != b[1]; y += sign(b[1] - a[1]) {
			cells = append(cells, [2]int{b[0], y})
		}
	}
	if len(path) > 0 {
		cells = append(cells, path[len(path)-1])
	}
	for range c.h + c.w {
		// the nearest cell of the edge above or below the label, or
		// beside it on its rows, in blank cells between
		dx, dy, gap := 0, 0, math.MaxInt
		for _, p := range cells {
			beside := p[1] >= y && p[1] < y+h
			switch {
			case p[0] >= x && p[0] < x+w && p[1] < y:
				if d := y - p[1] - 1; d < gap {
					dx, dy, gap = 0, -1, d
				}
			case p[0] >= x && p[0] < x+w && p[1] >= y+h:
				if d := p[1] - (y + h); d < gap {
					dx, dy, gap = 0, 1, d
				}
			case beside && p[0] < x:
				if d := x - p[0] - 1; d < gap {
					dx, dy, gap = -1, 0, d
				}
			case beside && p[0] >= x+w:
				if d := p[0] - (x + w); d < gap {
					dx, dy, gap = 1, 0, d
				}
			}
		}
		if gap <= 0 || gap == math.MaxInt || !c.blank(x+dx, y+dy, w, h) {
			break
		}
		x, y = x+dx, y+dy
	}
	return x, y
}

// blank reports whether the w by h cells from x, y are empty
func (c *canvas) blank(x, y, w, h int) bool {
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			if p := c.at(col, row); p == nil || p.r != ' ' || p.solid {
				return false
			}
		}
	}
	return true
}

// evenRecord moves the divider after a field a column left when the
// field's text would have an odd number of spare columns around it, so
// that the text is centered; the field after takes the column.
func (c *canvas) evenRecord(rec *draw.Record, origin layout.Vector) {
	for i, field := range rec.Fields {
		if !rec.Vertical && i+1 < len(rec.Fields) && len(field.Fields) == 0 {
			x0, x1 := c.col(origin.X+layout.Length(field.X0)), c.col(origin.X+layout.Length(field.X1))
			if spare := x1 - x0 - 1 - draw.TextColumns(field.Text); spare >= 3 && spare%2 == 1 {
				x := field.X1 - float64(c.cellW)
				field.X1 = x
				setLeft(rec.Fields[i+1], x)
			}
		}
		c.evenRecord(field, origin)
	}
}

// setLeft moves the left edge of a field and of the fields along it
func setLeft(rec *draw.Record, x float64) {
	rec.X0 = x
	for i, field := range rec.Fields {
		if rec.Vertical || i == 0 {
			setLeft(field, x)
		}
	}
}

// boolInt returns 1 for true and 0 for false
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
