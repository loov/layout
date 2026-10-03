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
func (c *canvas) drawCluster(cluster *layout.Cluster) {
	x0, y0 := c.col(cluster.TopLeft.X), c.row(cluster.TopLeft.Y)
	x1, y1 := c.col(cluster.BottomRight.X), c.row(cluster.BottomRight.Y)
	if cluster.Label != "" {
		// the label fits, spaced, between the corners; estimates of its
		// width can fall a few cells short
		x1 = max(x1, x0+clusterLabelWidth(cluster))
	}
	c.edge++
	c.ink = rgb(cluster.LineColor)
	c.fill(x0, y0, x1, y1, rgb(cluster.FillColor))
	c.dashed = true
	c.frame(x0, y0, x1, y1)
	c.dashed = false
}

// peripheryGap matches the layout's distance between a node's outlines
const peripheryGap = 4 * layout.Point

// drawNode draws a node as a box that holds its label, and marks the box
// solid so that edges don't draw over it
func (c *canvas) drawNode(graph *layout.Graph, node *layout.Node) {
	if node.Shape == layout.PointShape {
		x, y := c.col(node.Center.X), c.row(node.Center.Y)
		c.boxes[node] = [4]int{x, y, x, y}
		c.ink = rgb(node.LineColor)
		if node.FillColor != nil {
			c.ink = rgb(node.FillColor)
		}
		c.set(x, y, '●')
		if x >= 0 && x < c.w && y >= 0 && y < c.h {
			c.solid[y*c.w+x] = true
		}
		return
	}
	// a double outline takes the cells of a single one, so the room the
	// layout reserves for extra outlines stays outside the box
	inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
	x0, y0 := c.col(node.Left()+inset), c.row(node.Top()+inset)
	x1, y1 := c.col(node.Right()-inset), c.row(node.Bottom()-inset)
	lines := strings.Split(draw.PlainLabel(node.DefaultLabel()), "\n")
	// the box must hold the label and have distinct edges
	if node.Shape != layout.Record {
		for _, line := range lines {
			x1 = max(x1, x0+draw.Columns(line)+1)
		}
	}
	x1 = max(x1, x0+2)
	y1 = max(y1, y0+2)
	var rec *draw.Record
	if node.Shape == layout.Record {
		rec = layoutRecord(graph, node)
		y1 = max(y1, y0+draw.RecordRows(rec)+1)
	} else {
		y1 = max(y1, y0+len(lines)+1)
	}
	c.boxes[node] = [4]int{x0, y0, x1, y1}
	c.ink, c.font = rgb(node.LineColor), rgb(node.FontColor)
	c.fill(x0, y0, x1, y1, rgb(node.FillColor))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, ' ')
			if x >= 0 && x < c.w && y >= 0 && y < c.h {
				c.solid[y*c.w+x] = true
			}
			if x > x0 && x < x1 && y > y0 && y < y1 {
				c.hold(x, y) // blanks inside a box are part of it
			}
		}
	}
	style := "╭╮╰╯─│"
	switch {
	case node.Shape == layout.None:
		style = "      "
	case node.Peripheries > 1:
		style = "╔╗╚╝═║" // like double circles, of any shape
	case node.Shape == layout.Box, node.Shape == layout.Square, node.Shape == layout.Record:
		style = "┌┐└┘─│"
	}
	c.rect(x0, y0, x1, y1, style)
	if rec != nil {
		// fields at their share of the rows, which can be more than the
		// node rounds to
		inside := func(v layout.Length) int {
			share := float64((v - node.Top()) / (node.Bottom() - node.Top()))
			return y0 + int(math.Round(share*float64(y1-y0)))
		}
		c.record(rec, node.TopLeft(), c.col, inside)
		return
	}
	top := (y0 + y1 + 1 - len(lines)) / 2
	if graph.PackEdgeEnds && (graph.RankDir == layout.LeftToRight || graph.RankDir == layout.RightToLeft) {
		top = y0 + 1 // with the main path, along the first row
	}
	for i, line := range lines {
		c.text((x0+x1+1-draw.Columns(line))/2, top+i, line)
	}
}

// edgeCells returns the cells of the path of an edge, with its ends on
// the borders of the node boxes; the nodes must be drawn first
func (c *canvas) edgeCells(edge *layout.Edge) [][2]int {
	if len(edge.Path) < 2 {
		return nil
	}
	cells := make([][2]int, len(edge.Path))
	for i, p := range edge.Path {
		cells[i] = [2]int{c.col(p.X), c.row(p.Y)}
	}
	last := len(cells) - 1
	cells[0] = c.border(cells[0], cells[1], edge.From)
	cells[last] = c.border(cells[last], cells[last-1], edge.To)
	c.atDot(cells, 0, 1, 2, edge.From)
	c.atDot(cells, last, last-1, last-2, edge.To)
	return c.across(cells, edge.From, edge.To)
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
	if node.Shape != layout.PointShape {
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
				path[e.i][across] = map[bool]int{false: b[across], true: b[across+2]}[after]
				end := end{path: path, i: e.i, j: e.j, fixed: true, toward: at[along]}
				if e.k >= 0 && e.k < len(path) {
					end.fixed = path[e.k][along] == bend[along]
					end.toward = path[e.k][along]
				}
				key := side{e.node, along, after}
				sides[key] = append(sides[key], end)
				break
			}
		}
	}
	for key, ends := range sides {
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
func (c *canvas) drawEdge(edge *layout.Edge, cells [][2]int) {
	if len(cells) < 2 {
		return
	}
	path := edge.Path
	last := len(cells) - 1
	c.edge++
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
	if !c.marker(head, arrival(cells[last-1], cells[last], false, path[len(path)-2], path[len(path)-1]), cells[last]) {
		c.join(cells[last], edge.To)
	}
	if !c.marker(edge.ArrowTail, arrival(cells[1], cells[0], true, path[1], path[0]), cells[0]) {
		c.join(cells[0], edge.From)
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
	if node.Shape == layout.None || node.Shape == layout.PointShape {
		return // no border to join
	}
	if end[0] < 0 || end[0] >= c.w || end[1] < 0 || end[1] >= c.h {
		return // pinned nodes can lie outside the canvas
	}
	b := c.boxes[node]
	i := end[1]*c.w + end[0]
	double := node.Peripheries > 1
	pick := func(single, double2 rune) rune { return map[bool]rune{false: single, true: double2}[double] }
	switch side := end[1] > b[1] && end[1] < b[3]; {
	case side && end[0] == b[0] && end[0] > 0 && c.lines[i-1]&right != 0:
		c.cells[i] = pick('┤', '╢')
	case side && end[0] == b[2] && end[0]+1 < c.w && c.lines[i+1]&left != 0:
		c.cells[i] = pick('├', '╟')
	case end[0] <= b[0] || end[0] >= b[2]:
		// corners and outside the box
	case end[1] == b[1] && end[1] > 0 && c.lines[i-c.w]&down != 0:
		c.cells[i] = pick('┴', '╨')
	case end[1] == b[3] && end[1]+1 < c.h && c.lines[i+c.w]&up != 0:
		c.cells[i] = pick('┬', '╥')
	}
}

// drawLabels writes edge and cluster labels over everything else; paths
// are the cells of the edges
func (c *canvas) drawLabels(graph *layout.Graph, paths [][][2]int) {
	for i, edge := range graph.Edges {
		if edge.Label != "" {
			lines := strings.Split(draw.PlainLabel(edge.Label), "\n")
			w := 0
			for _, line := range lines {
				w = max(w, draw.Columns(line))
			}
			x, y := c.col(edge.LabelPos.X-edge.LabelRadius.X), c.row(edge.LabelPos.Y)-(len(lines)-1)/2
			x, y = c.nearEdge(x, y, w, len(lines), paths[i])
			c.font = rgb(edge.FontColor)
			for k, line := range lines {
				c.text(x, y+k, line)
			}
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.Label != "" {
			c.font = 0
			c.text(c.col(cluster.TopLeft.X)+1, c.row(cluster.TopLeft.Y), " "+strings.ReplaceAll(draw.PlainLabel(cluster.Label), "\n", " ")+" ")
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
	if y < 0 || y+h > c.h || x < 0 || x+w > c.w {
		return false
	}
	for row := y; row < y+h; row++ {
		for i := row*c.w + x; i < row*c.w+x+w; i++ {
			if c.cells[i] != ' ' || c.solid[i] {
				return false
			}
		}
	}
	return true
}
