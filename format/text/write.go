// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals. Edges are rasterized onto a character grid as horizontal
// and vertical runs, so ortho splines look best; diagonal segments become
// staircases.
package text

import (
	"io"
	"strings"

	"github.com/loov/layout"
)

// line direction bits of a cell
const (
	up = 1 << iota
	down
	left
	right
)

// box maps a direction mask to the line character with those arms.
var box = [16]rune{
	0: ' ', up: '│', down: '│', up | down: '│',
	left: '─', right: '─', left | right: '─',
	down | right: '┌', down | left: '┐', up | right: '└', up | left: '┘',
	up | down | right: '├', up | down | left: '┤',
	left | right | down: '┬', left | right | up: '┴',
	up | down | left | right: '┼',
}

var arrow = map[int]rune{up: '▲', down: '▼', left: '◀', right: '▶'}

type canvas struct {
	w, h         int
	cells        []rune
	lines        []int  // direction mask per cell, for joining edge runs
	solid        []bool // cells covered by a node; edges do not draw there
	lastX, lastY int    // last cell an edge run drew into
}

func (c *canvas) set(x, y int, r rune) {
	if x >= 0 && x < c.w && y >= 0 && y < c.h {
		c.cells[y*c.w+x] = r
		c.lines[y*c.w+x] = 0
	}
}

func (c *canvas) line(x, y int, mask int) {
	if x < 0 || x >= c.w || y < 0 || y >= c.h || c.solid[y*c.w+x] {
		return
	}
	i := y*c.w + x
	c.lines[i] |= mask
	c.cells[i] = box[c.lines[i]]
}

func (c *canvas) text(x, y int, s string) {
	for i, r := range []rune(s) {
		c.set(x+i, y, r)
	}
}

func (c *canvas) rect(x0, y0, x1, y1 int, style string) {
	s := []rune(style) // top-left, top-right, bottom-left, bottom-right, horizontal, vertical
	for x := x0 + 1; x < x1; x++ {
		c.set(x, y0, s[4])
		c.set(x, y1, s[4])
	}
	for y := y0 + 1; y < y1; y++ {
		c.set(x0, y, s[5])
		c.set(x1, y, s[5])
	}
	c.set(x0, y0, s[0])
	c.set(x1, y0, s[1])
	c.set(x0, y1, s[2])
	c.set(x1, y1, s[3])
}

// record draws field texts centered in their boxes with dividers between
// sibling fields
func (c *canvas) record(rec *layout.RecordField, origin layout.Vector, col, row func(layout.Length) int) {
	if len(rec.Fields) == 0 {
		x0, y0 := col(origin.X+rec.TopLeft.X), row(origin.Y+rec.TopLeft.Y)
		x1, y1 := col(origin.X+rec.BottomRight.X), row(origin.Y+rec.BottomRight.Y)
		r := []rune(rec.Text)
		c.text((x0+x1+1)/2-len(r)/2, (y0+y1)/2, rec.Text)
		return
	}
	// dividers first, so that field texts win when rows are too coarse
	for i, field := range rec.Fields {
		if i == 0 {
			continue
		}
		if rec.Vertical {
			y := row(origin.Y + field.TopLeft.Y)
			for x := col(origin.X+field.TopLeft.X) + 1; x < col(origin.X+field.BottomRight.X); x++ {
				c.set(x, y, '─')
			}
		} else {
			x := col(origin.X + field.TopLeft.X)
			for y := row(origin.Y + field.TopLeft.Y); y <= row(origin.Y+field.BottomRight.Y); y++ {
				c.set(x, y, '│')
			}
		}
	}
	for _, field := range rec.Fields {
		c.record(field, origin, col, row)
	}
}

// walk draws a run from (x0,y0) to (x1,y1) as a horizontal then a
// vertical leg.
func (c *canvas) walk(x0, y0, x1, y1 int) {
	step := func(x, y, dir, back int) {
		c.line(x, y, dir)
		c.line(x+dx(dir), y+dy(dir), back)
	}
	for x := x0; x != x1; x += sign(x1 - x0) {
		if x1 > x0 {
			step(x, y0, right, left)
		} else {
			step(x, y0, left, right)
		}
	}
	for y := y0; y != y1; y += sign(y1 - y0) {
		if y1 > y0 {
			step(x1, y, down, up)
		} else {
			step(x1, y, up, down)
		}
	}
}

func absLength(v layout.Length) layout.Length {
	if v < 0 {
		return -v
	}
	return v
}

func dx(dir int) int { return map[int]int{left: -1, right: 1}[dir] }
func dy(dir int) int { return map[int]int{up: -1, down: 1}[dir] }
func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// Write draws the laid out graph as text. One character cell is
// graph.FontSize*0.55 wide and graph.LineHeight tall.
func Write(w io.Writer, graph *layout.Graph) error {
	cellW, cellH := graph.FontSize*0.55, graph.LineHeight
	if cellW <= 0 {
		cellW = 8
	}
	if cellH <= 0 {
		cellH = 16
	}
	col := func(v layout.Length) int { return int(v/cellW + 0.5) }
	row := func(v layout.Length) int { return int(v/cellH + 0.5) }

	_, size := graph.Bounds()
	c := &canvas{w: col(size.X) + 2, h: row(size.Y) + 2}
	for _, edge := range graph.Edges {
		if edge.Label != "" {
			c.w = max(c.w, col(edge.LabelPos.X-edge.LabelRadius.X)+len([]rune(edge.Label))+1)
		}
	}
	c.cells = []rune(strings.Repeat(" ", c.w*c.h))
	c.lines = make([]int, c.w*c.h)
	c.solid = make([]bool, c.w*c.h)

	for _, cluster := range graph.Clusters {
		c.rect(col(cluster.TopLeft.X), row(cluster.TopLeft.Y), col(cluster.BottomRight.X), row(cluster.BottomRight.Y), "┌┐└┘┄┆")
	}

	for _, node := range graph.Nodes {
		x0, y0 := col(node.Left()), row(node.Top())
		x1, y1 := col(node.Right()), row(node.Bottom())
		lines := strings.Split(node.DefaultLabel(), "\n")
		// the box must hold the label and have distinct edges
		if node.Shape != layout.Record {
			for _, line := range lines {
				x1 = max(x1, x0+len([]rune(line))+1)
			}
		}
		x1 = max(x1, x0+2)
		y1 = max(y1, y0+2)
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
			inside := func(v layout.Length) int { return min(max(row(v), y0+1), y1-1) }
			c.record(graph.LayoutRecord(node), node.TopLeft(), col, inside)
			continue
		}
		for i, line := range lines {
			r := []rune(line)
			c.text((x0+x1+1)/2-len(r)/2, (y0+y1)/2-len(lines)/2+i, line)
		}
	}

	for _, edge := range graph.Edges {
		path := edge.Path
		if len(path) < 2 {
			continue
		}
		cells := make([][2]int, len(path))
		for i, p := range path {
			cells[i] = [2]int{col(p.X), row(p.Y)}
		}
		for i := 0; i+1 < len(cells); i++ {
			c.walk(cells[i][0], cells[i][1], cells[i+1][0], cells[i+1][1])
		}
		if edge.Directed && edge.ArrowHead != layout.ArrowNone {
			// the arrowhead sits on the node border, pointing along the last segment
			a, b := path[len(path)-2], path[len(path)-1]
			dir := down
			switch dx, dy := b.X-a.X, b.Y-a.Y; {
			case dy < 0 && -dy >= absLength(dx):
				dir = up
			case dx > 0 && dx > absLength(dy):
				dir = right
			case dx < 0 && -dx > absLength(dy):
				dir = left
			}
			end := cells[len(cells)-1]
			c.set(end[0], end[1], arrow[dir])
		}
	}
	for _, edge := range graph.Edges {
		if edge.Label != "" {
			c.text(col(edge.LabelPos.X-edge.LabelRadius.X), row(edge.LabelPos.Y), edge.Label)
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.Label != "" {
			c.text(col(cluster.TopLeft.X)+1, row(cluster.TopLeft.Y), " "+cluster.Label+" ")
		}
	}

	var out strings.Builder
	for y := 0; y < c.h; y++ {
		out.WriteString(strings.TrimRight(string(c.cells[y*c.w:(y+1)*c.w]), " "))
		out.WriteByte('\n')
	}
	_, err := io.WriteString(w, out.String())
	return err
}
