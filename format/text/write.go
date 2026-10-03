// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals. Edges are rasterized onto a character grid as horizontal
// and vertical runs, so ortho splines look best; diagonal segments become
// staircases. Call Prepare before laying out for output with room for
// the runs; the extra room is carved away again when writing.
package text

import (
	"io"
	"math"
	"slices"
	"strings"

	"github.com/loov/layout"
)

// Prepare sets ortho edges and spacing that leaves rows between ranks
// for horizontal runs and arrowheads: more fan-out needs more rows.
func Prepare(graph *layout.Graph) {
	graph.Splines = layout.SplinesOrtho
	if graph.LineHeight <= 0 {
		graph.LineHeight = 16
	}
	fan := map[*layout.Node]int{}
	labels := false
	for _, edge := range graph.Edges {
		fan[edge.From]++
		fan[edge.To]++
		labels = labels || edge.Label != ""
	}
	rows := 2.0
	for _, n := range fan {
		rows = max(rows, 2+math.Sqrt(float64(n)))
	}
	if labels {
		rows += 2
	}
	graph.RowPadding = graph.LineHeight * layout.Length(math.Min(rows, 8))
	graph.NodePadding = graph.LineHeight * 2
	graph.EdgePadding = graph.LineHeight
}

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
	w, h   int
	cells  []rune
	lines  []int  // direction mask per cell, for joining edge runs
	solid  []bool // cells covered by a node; edges do not draw there
	dashed bool   // straight runs drawn from now on are dashed
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
	if c.dashed {
		switch c.lines[i] {
		case up, down, up | down:
			c.cells[i] = '┊'
		case left, right, left | right:
			c.cells[i] = '┈'
		}
	}
}

// frame draws a rectangle outline through the line merge, so that edges
// crossing it join instead of overwriting it
func (c *canvas) frame(x0, y0, x1, y1 int) {
	for x := x0; x < x1; x++ {
		c.line(x, y0, right)
		c.line(x+1, y0, left)
		c.line(x, y1, right)
		c.line(x+1, y1, left)
	}
	for y := y0; y < y1; y++ {
		c.line(x0, y, down)
		c.line(x0, y+1, up)
		c.line(x1, y, down)
		c.line(x1, y+1, up)
	}
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
		c.text((x0+x1+1-len(r))/2, (y0+y1)/2, rec.Text)
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

func opposite(dir int) int { return map[int]int{up: down, down: up, left: right, right: left}[dir] }
func dx(dir int) int       { return map[int]int{left: -1, right: 1}[dir] }
func dy(dir int) int       { return map[int]int{up: -1, down: 1}[dir] }
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

	c.dashed = true
	for _, cluster := range graph.Clusters {
		c.frame(col(cluster.TopLeft.X), row(cluster.TopLeft.Y), col(cluster.BottomRight.X), row(cluster.BottomRight.Y))
	}
	c.dashed = false

	boxes := map[*layout.Node][4]int{}
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
		boxes[node] = [4]int{x0, y0, x1, y1}
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
			c.text((x0+x1+1-len(r))/2, (y0+y1)/2-len(lines)/2+i, line)
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
		// every shape is drawn as a box, so ends on a rounder outline
		// move out to the box border they face
		border := func(end, next [2]int, node *layout.Node) [2]int {
			b := boxes[node]
			if end[0] > b[0] && end[0] < b[2] && end[1] > b[1] && end[1] < b[3] {
				if next[1] < end[1] {
					end[1] = b[1]
				} else if next[1] > end[1] {
					end[1] = b[3]
				}
			}
			return end
		}
		last := len(cells) - 1
		cells[0] = border(cells[0], cells[1], edge.From)
		cells[last] = border(cells[last], cells[last-1], edge.To)
		c.dashed = edge.LineStyle == layout.Dashed || edge.LineStyle == layout.Dotted
		for i := 0; i+1 < len(cells); i++ {
			c.walk(cells[i][0], cells[i][1], cells[i+1][0], cells[i+1][1])
		}
		c.dashed = false
		if edge.Directed && edge.ArrowHead != layout.ArrowNone {
			// the arrowhead points along the last segment; it sits in the
			// gap before the node when the run there is straight, else on
			// the node border
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
			if px, py := end[0]-dx(dir), end[1]-dy(dir); px >= 0 && px < c.w && py >= 0 && py < c.h && c.lines[py*c.w+px] == dir|opposite(dir) {
				end = [2]int{px, py}
			}
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

	grid := make([][]rune, c.h)
	for y := range grid {
		grid[y] = c.cells[y*c.w : (y+1)*c.w]
	}
	grid = carve(grid, " │┊┆", 1)
	grid = transpose(carve(transpose(grid), " ─┈┄", 2))

	var out strings.Builder
	for _, line := range grid {
		out.WriteString(strings.TrimRight(string(line), " "))
		out.WriteByte('\n')
	}
	_, err := io.WriteString(w, out.String())
	return err
}

// carve removes rows that only continue straight lines or blanks and
// repeat the row before them, keeping at most keep of every such run.
// Removing them keeps the drawing connected, just tighter.
func carve(grid [][]rune, straight string, keep int) [][]rune {
	out := grid[:0:0]
	run := 0
	for i, row := range grid {
		plain := true
		for _, r := range row {
			if !strings.ContainsRune(straight, r) {
				plain = false
				break
			}
		}
		if plain && i > 0 && slices.Equal(row, grid[i-1]) {
			run++
		} else {
			run = 0
		}
		if run < keep {
			out = append(out, row)
		}
	}
	return out
}

func transpose(grid [][]rune) [][]rune {
	if len(grid) == 0 {
		return nil
	}
	out := make([][]rune, len(grid[0]))
	for x := range out {
		out[x] = make([]rune, len(grid))
		for y := range grid {
			out[x][y] = grid[y][x]
		}
	}
	return out
}
