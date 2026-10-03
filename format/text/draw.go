package text

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/loov/layout"
)

// drawCluster draws the dashed frame and fill of a cluster
func (c *canvas) drawCluster(cluster *layout.Cluster) {
	x0, y0 := c.col(cluster.TopLeft.X), c.row(cluster.TopLeft.Y)
	x1, y1 := c.col(cluster.BottomRight.X), c.row(cluster.BottomRight.Y)
	c.edge++
	c.ink = rgb(cluster.LineColor)
	c.fill(x0, y0, x1, y1, rgb(cluster.FillColor))
	c.dashed = true
	c.frame(x0, y0, x1, y1)
	c.dashed = false
}

// drawNode draws a node as a box that holds its label, and marks the box
// solid so that edges don't draw over it
func (c *canvas) drawNode(graph *layout.Graph, node *layout.Node) {
	if node.Shape == layout.Dot {
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
	x0, y0 := c.col(node.Left()), c.row(node.Top())
	x1, y1 := c.col(node.Right()), c.row(node.Bottom())
	lines := strings.Split(node.DefaultLabel(), "\n")
	// the box must hold the label and have distinct edges
	if node.Shape != layout.Record {
		for _, line := range lines {
			x1 = max(x1, x0+len([]rune(line))+1)
		}
	}
	x1 = max(x1, x0+2)
	y1 = max(y1, y0+2)
	c.boxes[node] = [4]int{x0, y0, x1, y1}
	c.ink, c.font = rgb(node.LineColor), rgb(node.FontColor)
	c.fill(x0, y0, x1, y1, rgb(node.FillColor))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, ' ')
			if x >= 0 && x < c.w && y >= 0 && y < c.h {
				c.solid[y*c.w+x] = true
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
	if node.Shape == layout.Record {
		inside := func(v layout.Length) int { return min(max(c.row(v), y0+1), y1-1) }
		c.record(graph.LayoutRecord(node), node.TopLeft(), c.col, inside)
		return
	}
	for i, line := range lines {
		r := []rune(line)
		c.text((x0+x1+1-len(r))/2, (y0+y1)/2-len(lines)/2+i, line)
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
	return cells
}

// spreadSides gives every edge end on the left or right side of a box a
// row of its own between the borders, as close to where it rounded as
// the others allow. The layout spreads ends a row or so apart, but
// rounds them to rows independently of the box, so ends could share a
// row or land on a border. An end whose run goes on straight keeps its
// row if it can; a moved end takes the bend before it along.
func (c *canvas) spreadSides(edges []*layout.Edge, paths [][][2]int) {
	type end struct {
		path   [][2]int
		i, j   int  // the end and the bend before it
		fixed  bool // the run goes on straight past the bend
		toward int  // the row the edge heads for past the bend
	}
	type side struct {
		node  *layout.Node
		right bool
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
			right := bend[0] > b[2]
			if b[3]-b[1] < 2 || at[1] != bend[1] || at[1] < b[1] || at[1] > b[3] || !right && bend[0] >= b[0] {
				continue // not along a row from beside the box
			}
			// on the side, also where a rounder outline curves inside
			path[e.i][0] = map[bool]int{false: b[0], true: b[2]}[right]
			end := end{path: path, i: e.i, j: e.j, fixed: true, toward: at[1]}
			if e.k >= 0 && e.k < len(path) {
				end.fixed = path[e.k][1] == bend[1]
				end.toward = path[e.k][1]
			}
			key := side{e.node, right}
			sides[key] = append(sides[key], end)
		}
	}
	for key, ends := range sides {
		b := c.boxes[key.node]
		lo, hi := b[1]+1, b[3]-1
		if len(ends) > hi-lo+1 {
			continue // no room; keep them as they are
		}
		slices.SortStableFunc(ends, func(a, b end) int {
			return cmp.Or(cmp.Compare(a.path[a.i][1], b.path[b.i][1]), cmp.Compare(a.toward, b.toward))
		})
		want := make([]int, len(ends))
		fixed := make([]bool, len(ends))
		for n, e := range ends {
			want[n], fixed[n] = e.path[e.i][1], e.fixed
		}
		for n, row := range spreadRows(want, fixed, lo, hi) {
			e := ends[n]
			e.path[e.i][1], e.path[e.j][1] = row, row
		}
	}
}

// spreadRows moves the ordered rows want as little as possible so that
// each gets its own row within [lo, hi], with fixed rows moving only
// when they must. Shifting row n by n turns this into ordering, solved
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
	if !c.marker(head, path[len(path)-2], path[len(path)-1], cells[last]) {
		c.join(cells[last], edge.To)
	}
	if !c.marker(edge.ArrowTail, path[1], path[0], cells[0]) {
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
	if node.Shape == layout.None || node.Shape == layout.Dot {
		return // no border to join
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

// drawLabels writes edge and cluster labels over everything else
func (c *canvas) drawLabels(graph *layout.Graph) {
	for _, edge := range graph.Edges {
		if edge.Label != "" {
			c.font = rgb(edge.FontColor)
			c.text(c.col(edge.LabelPos.X-edge.LabelRadius.X), c.row(edge.LabelPos.Y), edge.Label)
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.Label != "" {
			c.font = 0
			c.text(c.col(cluster.TopLeft.X)+1, c.row(cluster.TopLeft.Y), " "+cluster.Label+" ")
		}
	}
}
