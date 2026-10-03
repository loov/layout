package text

import (
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
	switch node.Shape {
	case layout.Box, layout.Square, layout.Record:
		style = "┌┐└┘─│"
	case layout.None:
		style = "      "
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

// drawEdge draws the path of an edge with its end markers; the nodes
// must be drawn first
func (c *canvas) drawEdge(edge *layout.Edge) {
	path := edge.Path
	if len(path) < 2 {
		return
	}
	cells := make([][2]int, len(path))
	for i, p := range path {
		cells[i] = [2]int{c.col(p.X), c.row(p.Y)}
	}
	last := len(cells) - 1
	cells[0] = c.border(cells[0], cells[1], edge.From)
	cells[last] = c.border(cells[last], cells[last-1], edge.To)
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

// join draws the box side of node at an edge end without a marker as a
// junction, so that the edge visibly leaves the node
func (c *canvas) join(end [2]int, node *layout.Node) {
	if node.Shape == layout.None || node.Shape == layout.Dot {
		return // no border to join
	}
	b := c.boxes[node]
	if end[1] <= b[1] || end[1] >= b[3] {
		return
	}
	i := end[1]*c.w + end[0]
	switch {
	case end[0] == b[0] && end[0] > 0 && c.lines[i-1]&right != 0:
		c.cells[i] = '┤'
	case end[0] == b[2] && end[0]+1 < c.w && c.lines[i+1]&left != 0:
		c.cells[i] = '├'
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
