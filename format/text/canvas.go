package text

import (
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

// glyphs holds the line character for every combination of arm weights
// (0 none, 1 light, 2 heavy), indexed by up*27 + down*9 + left*3 + right.
var glyphs = []rune(" ╶╺╴─╼╸╾━╷┌┍┐┬┮┑┭┯╻┎┏┒┰┲┓┱┳╵└┕┘┴┶┙┵┷│├┝┤┼┾┥┽┿╽┟┢┧╁╆┪╅╈╹┖┗┚┸┺┛┹┻╿┞┡┦╀╄┩╃╇┃┠┣┨╂╊┫╉╋")

var rounded = map[rune]rune{'┌': '╭', '┐': '╮', '└': '╰', '┘': '╯'}

// glyph returns the line character with the arms in lines, drawing the
// arms in the mask heavy as heavy lines. A lone arm is drawn as a full
// straight line.
func glyph(lines, heavy int) rune {
	if lines&(lines-1) == 0 {
		lines |= opposite(lines)
		if heavy != 0 {
			heavy = lines
		}
	}
	i := 0
	for _, arm := range []int{up, down, left, right} {
		w := 0
		if lines&arm != 0 {
			w = 1
		}
		if heavy&arm != 0 {
			w = 2
		}
		i = i*3 + w
	}
	return glyphs[i]
}

var arrow = map[int]rune{up: '▲', down: '▼', left: '◀', right: '▶'}

// canvas is the character grid the graph is drawn on, with what is
// needed to join lines and color the cells
type canvas struct {
	w, h   int
	cellW  layout.Length // size of a cell in graph units
	cellH  layout.Length
	origin layout.Vector           // graph coordinates of the top left cell
	boxes  map[*layout.Node][4]int // drawn node boxes: x0, y0, x1, y1
	cells  []rune
	fg, bg []uint32 // colors per cell, see rgb
	ink    uint32   // color of lines and marks drawn from now on
	font   uint32   // color of text drawn from now on
	lines  []int    // direction mask per cell, for joining edge runs
	heavy  []int    // arms that runs of different edges share
	owner  [][4]int // edge that first drew each arm of a cell
	solid  []bool   // cells covered by a node; edges do not draw there
	dashed bool     // straight runs drawn from now on are dashed
	edge   int      // edge drawn from now on, so that overlaps show
}

// newCanvas returns an empty canvas that fits the graph. One character
// cell is graph.FontSize*0.55 wide and graph.LineHeight tall.
func newCanvas(graph *layout.Graph) *canvas {
	c := &canvas{cellW: graph.FontSize * 0.55, cellH: graph.LineHeight, boxes: map[*layout.Node][4]int{}}
	if c.cellW <= 0 {
		c.cellW = 8
	}
	if c.cellH <= 0 {
		c.cellH = 16
	}
	// the drawing can reach before the origin, with labels nudged there
	// or pinned and force layouts; keep the usual margin otherwise
	topLeft, size := graph.Bounds()
	c.origin = layout.Vector{X: min(topLeft.X, 0), Y: min(topLeft.Y, 0)}
	c.w, c.h = c.col(size.X)+2, c.row(size.Y)+2
	for _, edge := range graph.Edges {
		for _, line := range strings.Split(plain(edge.Label), "\n") {
			c.w = max(c.w, c.col(edge.LabelPos.X-edge.LabelRadius.X)+width(line)+1)
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.Label != "" {
			c.w = max(c.w, c.col(cluster.TopLeft.X)+clusterLabelWidth(cluster)+1)
		}
	}
	c.cells = []rune(strings.Repeat(" ", c.w*c.h))
	c.fg = make([]uint32, c.w*c.h)
	c.bg = make([]uint32, c.w*c.h)
	c.lines = make([]int, c.w*c.h)
	c.heavy = make([]int, c.w*c.h)
	c.owner = make([][4]int, c.w*c.h)
	c.solid = make([]bool, c.w*c.h)
	return c
}

// col and row return the cell column and row of graph coordinates
func (c *canvas) col(v layout.Length) int { return int((v-c.origin.X)/c.cellW + 0.5) }
func (c *canvas) row(v layout.Length) int { return int((v-c.origin.Y)/c.cellH + 0.5) }

func (c *canvas) set(x, y int, r rune) {
	if x >= 0 && x < c.w && y >= 0 && y < c.h {
		c.cells[y*c.w+x] = r
		c.fg[y*c.w+x] = c.ink
		c.lines[y*c.w+x] = 0
		c.heavy[y*c.w+x] = 0
	}
}

func (c *canvas) line(x, y int, mask int) {
	if x < 0 || x >= c.w || y < 0 || y >= c.h || c.solid[y*c.w+x] {
		return
	}
	i := y*c.w + x
	for arm := range 4 {
		bit := 1 << arm
		switch {
		case mask&bit == 0:
		case c.lines[i]&bit == 0:
			c.owner[i][arm] = c.edge
		case c.owner[i][arm] != c.edge:
			c.heavy[i] |= bit
		}
	}
	c.lines[i] |= mask
	c.cells[i] = glyph(c.lines[i], c.heavy[i])
	c.fg[i] = c.ink
	if o := c.owner[i]; c.lines[i] == up|down|left|right && c.heavy[i] == 0 && o[0] == o[1] && o[2] == o[3] && o[0] != o[2] {
		c.cells[i] = '╂' // two edges crossing, not joining
	} else if r, ok := rounded[c.cells[i]]; ok && !c.dashed {
		c.cells[i] = r // bends of edges, unlike cluster frames, are round
	}
	if c.dashed {
		heavy := c.heavy[i] != 0
		switch c.lines[i] {
		case up, down, up | down:
			c.cells[i] = map[bool]rune{false: '┊', true: '┋'}[heavy]
		case left, right, left | right:
			c.cells[i] = map[bool]rune{false: '┈', true: '┉'}[heavy]
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
	ink := c.ink
	c.ink = c.font
	for _, r := range s {
		switch {
		case layout.IsZeroWidth(r):
			// a cell holds one character; marks on it are dropped
		case layout.IsWide(r):
			c.set(x, y, r)
			c.set(x+1, y, covered)
			x += 2
		default:
			c.set(x, y, r)
			x++
		}
	}
	c.ink = ink
}

// clusterLabelWidth returns the columns from a cluster's left corner past
// its label: the label between a space on each side, and the corners
func clusterLabelWidth(cluster *layout.Cluster) int {
	return width(strings.ReplaceAll(plain(cluster.Label), "\n", " ")) + 3
}

// covered marks the cell under the second column of a wide character,
// which writing skips
const covered = 0

// width returns the columns s takes in a terminal: two for wide
// characters, none for marks on the character before
func width(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case layout.IsZeroWidth(r):
		case layout.IsWide(r):
			n += 2
		default:
			n++
		}
	}
	return n
}

// fill sets the background of the cells inside the rectangle, unless
// color is the default
func (c *canvas) fill(x0, y0, x1, y1 int, color uint32) {
	if color == 0 {
		return
	}
	for y := max(y0+1, 0); y < min(y1, c.h); y++ {
		for x := max(x0+1, 0); x < min(x1, c.w); x++ {
			c.bg[y*c.w+x] = color
		}
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
// sibling fields; row maps the top and bottom of fields to the rows of the
// borders and dividers around them
func (c *canvas) record(rec *layout.RecordField, origin layout.Vector, col, row func(layout.Length) int) {
	if len(rec.Fields) == 0 {
		x0, y0 := col(origin.X+rec.TopLeft.X), row(origin.Y+rec.TopLeft.Y)
		x1, y1 := col(origin.X+rec.BottomRight.X), row(origin.Y+rec.BottomRight.Y)
		lines := strings.Split(rec.Text, "\n")
		for i, line := range lines {
			c.text((x0+x1+1-width(line))/2, (y0+y1)/2-(len(lines)-1)/2+i, line)
		}
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
			for y := row(origin.Y+field.TopLeft.Y) + 1; y < row(origin.Y+field.BottomRight.Y); y++ {
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

// marker draws an edge end marker for the segment from a to the end b,
// which lies in cell end. It sits in the gap before the node when the run
// there is straight, else on the node border. It reports whether style
// has a marker.
func (c *canvas) marker(style layout.Arrow, a, b layout.Vector, end [2]int) bool {
	dir := down
	switch dx, dy := b.X-a.X, b.Y-a.Y; {
	case dy < 0 && -dy >= absLength(dx):
		dir = up
	case dx > 0 && dx > absLength(dy):
		dir = right
	case dx < 0 && -dx > absLength(dy):
		dir = left
	}
	r, ok := map[layout.Arrow]rune{
		layout.ArrowNormal: arrow[dir],
		layout.ArrowVee:    map[int]rune{up: '↑', down: '↓', left: '←', right: '→'}[dir],
		layout.ArrowDot:    '●',
		layout.ArrowODot:   '○',
	}[style]
	if !ok {
		return false
	}
	if px, py := end[0]-dx(dir), end[1]-dy(dir); px >= 0 && px < c.w && py >= 0 && py < c.h && c.lines[py*c.w+px] == dir|opposite(dir) {
		end = [2]int{px, py}
	}
	c.set(end[0], end[1], r)
	return true
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

// recordRows returns the rows the fields of a record need inside its
// box: a row per line of text, and one per divider between fields
// stacked vertically
func recordRows(rec *layout.RecordField) int {
	if len(rec.Fields) == 0 {
		return strings.Count(rec.Text, "\n") + 1
	}
	rows := 0
	for _, field := range rec.Fields {
		if rec.Vertical {
			rows += recordRows(field)
		} else {
			rows = max(rows, recordRows(field))
		}
	}
	if rec.Vertical {
		rows += len(rec.Fields) - 1
	}
	return rows
}
