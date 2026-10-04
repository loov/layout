package text

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// drawCluster draws the dashed frame and fill of a cluster
func (c *canvas) drawCluster(cluster *layout.Cluster, box layout.ClusterBox) {
	x0, y0 := c.col(box.TopLeft.X), c.row(box.TopLeft.Y)
	x1, y1 := c.col(box.BottomRight.X), c.row(box.BottomRight.Y)
	if cluster.Label != "" {
		// the label fits, spaced, between the corners; estimates of its
		// width can fall a few cells short
		x1 = max(x1, x0+clusterLabelWidth(cluster))
	}
	c.ids++
	c.edge = c.ids
	c.ink = rgb(cluster.LineColor)
	c.fill(x0, y0, x1, y1, rgb(cluster.FillColor))
	c.dashed = true
	c.frame(x0, y0, x1, y1)
	c.dashed = false
}

// peripheryGap matches the layout's distance between a node's outlines
const peripheryGap = 4 * layout.Point

// drawNode draws a node as a box that holds its label, and marks the box
// solid so that edges don't draw over it; an invisible node only keeps
// its box clear
func (c *canvas) drawNode(graph *layout.Graph, node *layout.Node) {
	box := c.l.Node(node)
	if box.Shape == layout.PointShape {
		x, y := c.col(box.Center.X), c.row(box.Center.Y)
		c.boxes[node] = [4]int{x, y, x, y}
		if node.Invisible {
			return
		}
		c.ink = rgb(node.LineColor)
		if node.FillColor != nil {
			c.ink = rgb(node.FillColor)
		}
		c.set(x, y, '●')
		if p := c.at(x, y); p != nil {
			p.solid = true
		}
		return
	}
	// a double outline takes the cells of a single one, so the room the
	// layout reserves for extra outlines stays outside the box
	inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
	x0, y0 := c.col(box.Left()+inset), c.row(box.Top()+inset)
	x1, y1 := c.col(box.Right()-inset), c.row(box.Bottom()-inset)
	label := draw.PlainLabel(box.Label)
	lines := strings.Split(label, "\n")
	w, h := draw.LabelBox(label)
	// the box must hold the label and have distinct edges
	if box.Shape != layout.Record {
		x1 = max(x1, x0+w)
		// an odd number of spare cells can't be split evenly around the
		// label, give one back
		if spare := x1 - x0 - 1 - draw.TextColumns(label); spare >= 3 && spare%2 == 1 {
			x1--
		}
	}
	x1 = max(x1, x0+2)
	y1 = max(y1, y0+2)
	var rec *draw.Record
	if box.Shape == layout.Record {
		rec = layoutRecord(graph, node, box)
		y1 = max(y1, y0+draw.RecordRows(rec)+1)
	} else {
		y1 = max(y1, y0+h)
	}
	c.boxes[node] = [4]int{x0, y0, x1, y1}
	c.ink, c.font = rgb(node.LineColor), rgb(node.FontColor)
	if !node.Invisible {
		c.fill(x0, y0, x1, y1, rgb(node.FillColor))
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, ' ')
			if p := c.at(x, y); p != nil {
				p.solid = true
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
	if rec != nil {
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
	top := (y0 + y1 + 1 - len(lines)) / 2
	if graph.PackEdgeEnds && (graph.RankDir == layout.LeftToRight || graph.RankDir == layout.RightToLeft) {
		top = y0 + 1 // with the main path, along the first row
	}
	for i, line := range lines {
		c.text(centered(x0, x1, line), top+i, line)
	}
}

// edgeCells returns the cells of the path of an edge, with its ends on
// the borders of the node boxes; the nodes must be drawn first
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
		i, j   int  // the end and the bend before it
		fixed  bool // the run goes on straight past the bend
		toward int  // where the edge heads past the bend, along the side
		group  int  // the merged ends there, see layout.EdgePath.Merged

		also []end // further edges of a shared start, which follow it
	}
	type side struct {
		node  *layout.Node
		along int  // the axis along the side: 1 for left and right
		after bool // right or bottom
	}
	sides := map[side][]end{}
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
				key := side{e.node, along, after}
				sides[key] = append(sides[key], end)
				break
			}
		}
	}
	for key, ends := range sides {
		// merged ends move as one, led by an end that goes on straight
		// when there is one, as that stays
		lead := map[int]int{}
		for n, e := range ends {
			if l, ok := lead[e.group]; e.group != 0 && (!ok || e.fixed && !ends[l].fixed) {
				lead[e.group] = n
			}
		}
		also := map[int][]end{}
		for n, e := range ends {
			if l := lead[e.group]; e.group != 0 && l != n {
				also[l] = append(also[l], e)
			}
		}
		var kept []end
		for n, e := range ends {
			if e.group == 0 || lead[e.group] == n {
				e.also = also[n]
				kept = append(kept, e)
			}
		}
		ends = kept
		b := c.boxes[key.node]
		lo, hi := b[key.along]+1, b[key.along+2]-1
		if len(ends) > hi-lo+1 {
			continue // no room; keep them as they are
		}
		slices.SortStableFunc(ends, func(a, b end) int {
			return cmp.Or(cmp.Compare(a.path[a.i][key.along], b.path[b.i][key.along]), cmp.Compare(a.toward, b.toward))
		})
		want := make([]int, len(ends))
		fixed := make([]bool, len(ends))
		for n, e := range ends {
			want[n], fixed[n] = e.path[e.i][key.along], e.fixed
		}
		for n, at := range spreadRows(want, fixed, lo, hi) {
			e := ends[n]
			e.path[e.i][key.along], e.path[e.j][key.along] = at, at
			for _, f := range e.also {
				f.path[f.i][key.along], f.path[f.j][key.along] = at, at
			}
		}
	}
}

// spreadRows moves the ordered rows, or columns, want as little as
// possible so that each gets its own within [lo, hi], with fixed ones
// moving only when they must. Shifting row n by n turns this into ordering, solved
// by pooling adjacent violators.
func spreadRows(want []int, fixed []bool, lo, hi int) []int {
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
		blocks = append(blocks, block{float64(row-n) * weight, weight, 1})
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
		first := min(max(int(math.Round(b.sum/b.weight)), lo), hi-len(want)+1)
		for range b.n {
			rows = append(rows, first+len(rows))
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
	c.edge = c.ids
	if id, ok := c.merged[edge]; ok {
		c.edge = id // merged edges draw as one, without overlaps
	}
	c.ink = rgb(edge.LineColor)
	c.dashed = edge.LineStyle == layout.Dashed || edge.LineStyle == layout.Dotted
	for i := 0; i+1 < len(cells); i++ {
		c.walk(cells[i][0], cells[i][1], cells[i+1][0], cells[i+1][1])
	}
	c.dashed = false
	head := edge.ArrowHead
	if head == layout.ArrowDefault && edge.Directed {
		head = layout.ArrowNormal
	}
	// merged edges share their ends, which get one marker
	_, merged := c.merged[edge]
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
	if c.l.Node(node).Shape == layout.None || c.l.Node(node).Shape == layout.PointShape || node.Invisible {
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
			c.font = rgb(edge.FontColor)
			for k, line := range lines {
				c.text(x, y+k, line)
			}
		}
	}
	for i, cluster := range graph.Clusters {
		if cluster.Label != "" && !cluster.Invisible {
			c.font = 0
			box := c.l.Clusters[i]
			c.text(c.col(box.TopLeft.X)+1, c.row(box.TopLeft.Y), " "+clusterLabel(cluster)+" ")
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
				if d := x - p[0] - 2; d < gap { // one blank column is close
					dx, dy, gap = -1, 0, d
				}
			case beside && p[0] >= x+w:
				if d := p[0] - (x + w) - 1; d < gap {
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
