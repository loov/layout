package text

import (
	"strings"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
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

// arrowheads and vees by direction
var (
	arrow = [right + 1]rune{up: '▲', down: '▼', left: '◀', right: '▶'}
	vee   = [right + 1]rune{up: '↑', down: '↓', left: '←', right: '→'}
)

// canvas is the character grid the graph is drawn on, with what is
// needed to join lines and color the cells
type canvas struct {
	w, h   int
	cellW  layout.Length // size of a cell in graph units
	cellH  layout.Length
	origin layout.Vector // graph coordinates of the top left cell
	l      *layout.Layout
	boxes  map[*layout.Node][4]int // drawn node boxes: x0, y0, x1, y1
	rows   [][]cell
	pen    pen                  // what is drawn with from now on
	ids    int                  // edge ids handed out
	merged map[*layout.Edge]int // edge ids of merged edges
	drawn  map[*layout.Edge]int // edge ids of the edges drawn
	nodes  map[*layout.Node]int // node ids, from 1, see cell.node
	loops  map[*layout.Node]int // self-loops per node
	ended  map[[2]int]bool      // cells where merged edges have ended
	spread bool                 // edge ends on a side keep a cell apart where there is room
}

// pen is what the canvas draws with. Each drawing sets all of it, so
// that nothing carries over from the drawing before.
type pen struct {
	ink    uint32 // color of lines and marks
	font   uint32 // color of text
	dashed bool   // straight runs are dashed
	edge   int    // edge drawn, so that overlaps show
	frame  bool   // the frame of a cluster, see cell.frame
}

// newCanvas returns an empty canvas that fits the graph. One character
// cell is graph.FontSize*0.55 wide and graph.LineHeight tall.
func newCanvas(l *layout.Layout) *canvas {
	graph := l.Graph
	c := &canvas{l: l, cellW: graph.FontSize * 0.55, cellH: graph.LineHeight, boxes: map[*layout.Node][4]int{}, ended: map[[2]int]bool{}}
	if c.cellW <= 0 {
		c.cellW = 8
	}
	if c.cellH <= 0 {
		c.cellH = 16
	}
	// the drawing can reach before the origin, with labels nudged there
	// or pinned and force layouts; keep the usual margin otherwise
	topLeft, size := l.Bounds()
	c.origin = layout.Vector{X: min(topLeft.X, 0), Y: min(topLeft.Y, 0)}
	c.w, c.h = c.col(size.X)+2, c.row(size.Y)+2
	for i := range graph.Edges {
		label, x, _ := c.edgeLabel(i)
		c.w = max(c.w, x+draw.TextColumns(label)+1)
	}
	c.loops = map[*layout.Node]int{}
	for _, edge := range graph.Edges {
		if edge.From == edge.To {
			c.loops[edge.From]++
		}
	}
	// boxes can be wider than the layout's, see nodeBox and clusterBox
	fit := func(b [4]int) { c.w, c.h = max(c.w, b[2]+1), max(c.h, b[3]+1) }
	for _, node := range graph.Nodes {
		c.boxes[node] = c.nodeBox(node)
		fit(c.boxes[node])
	}
	for i := range graph.Clusters {
		fit(c.clusterBox(i))
	}
	c.reset(c.w, c.h)
	return c
}

// sideways reports whether the ranks of the graph are columns
func (c *canvas) sideways() bool {
	return c.l.Graph.RankDir == layout.LeftToRight || c.l.Graph.RankDir == layout.RightToLeft
}

// reset makes the canvas w by h blank cells
func (c *canvas) reset(w, h int) {
	c.w, c.h = w, h
	c.rows = make([][]cell, h)
	for y := range c.rows {
		c.rows[y] = make([]cell, w)
		for x := range c.rows[y] {
			c.rows[y][x].r = ' '
		}
	}
}

// at returns the cell at x, y, or nil off the canvas
func (c *canvas) at(x, y int) *cell {
	if x < 0 || x >= c.w || y < 0 || y >= c.h {
		return nil
	}
	return &c.rows[y][x]
}

// col and row return the cell column and row of graph coordinates
func (c *canvas) col(v layout.Length) int { return int((v-c.origin.X)/c.cellW + 0.5) }
func (c *canvas) row(v layout.Length) int { return int((v-c.origin.Y)/c.cellH + 0.5) }

// set writes r in the color of the pen's ink
func (c *canvas) set(x, y int, r rune) { c.put(x, y, r, c.pen.ink) }

// put writes r in the color fg over whatever the cell held
func (c *canvas) put(x, y int, r rune, fg uint32) {
	if p := c.at(x, y); p != nil {
		c.unpair(x, y, r != covered)
		p.r, p.fg, p.lines, p.heavy = r, fg, 0, 0
	}
}

func (c *canvas) line(x, y int, mask int) {
	p := c.at(x, y)
	if p == nil || p.solid {
		return
	}
	c.unpair(x, y, true)
	for arm := range 4 {
		bit := 1 << arm
		switch {
		case mask&bit == 0:
		case p.lines&bit == 0:
			p.owner[arm] = c.pen.edge
		case p.owner[arm] != c.pen.edge:
			p.heavy |= bit
		}
	}
	p.lines |= mask
	p.frame = p.frame || c.pen.frame
	p.r = glyph(p.lines, p.heavy)
	p.fg = c.pen.ink
	if o := p.owner; p.lines == up|down|left|right && p.heavy == 0 && o[0] == o[1] && o[2] == o[3] && o[0] != o[2] {
		p.r = '╂' // two edges crossing, not joining
	} else if r, ok := rounded[p.r]; ok && !c.pen.dashed {
		p.r = r // bends of edges, unlike cluster frames, are round
	}
	if c.pen.dashed {
		dashes := []rune("┊┈")
		if p.heavy != 0 {
			dashes = []rune("┋┉")
		}
		switch p.lines {
		case up, down, up | down:
			p.r = dashes[0]
		case left, right, left | right:
			p.r = dashes[1]
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

// text writes s from x, y in the color of the pen's font
func (c *canvas) text(x, y int, s string) {
	for _, r := range s {
		switch {
		case draw.IsZeroWidth(r), unicode.IsControl(r):
			// a cell holds one character; marks on it are dropped, and
			// control characters would garble the terminal
		case draw.IsWide(r):
			c.put(x, y, r, c.pen.font)
			c.put(x+1, y, covered, c.pen.font)
			c.hold(x, y)
			c.hold(x+1, y)
			x += 2
		default:
			c.put(x, y, r, c.pen.font)
			c.hold(x, y)
			x++
		}
	}
}

// hold keeps the cell at x, y from being carved away
func (c *canvas) hold(x, y int) {
	if p := c.at(x, y); p != nil {
		p.keep = true
	}
}

// clusterLabel returns the label of a cluster on one line, as it is
// drawn along the top of the frame
func clusterLabel(cluster *layout.Cluster) string {
	return strings.ReplaceAll(draw.PlainLabel(cluster.Label), "\n", " ")
}

// clusterLabelWidth returns the columns from a cluster's left corner past
// its label: the label between a space on each side, and the corners
func clusterLabelWidth(cluster *layout.Cluster) int {
	return draw.Columns(clusterLabel(cluster)) + 3
}

// edgeLabel returns the label of the edge at index i as text draws it,
// and the cell the layout puts its top left corner in
func (c *canvas) edgeLabel(i int) (label string, x, y int) {
	at := c.l.Edges[i]
	label = draw.PlainLabel(c.l.Graph.Edges[i].Label)
	return label, c.col(at.LabelCenter.X - at.LabelSize.X/2), c.row(at.LabelCenter.Y) - strings.Count(label, "\n")/2
}

// centered returns the column that centers line between the columns x0
// and x1 of the borders around it
func centered(x0, x1 int, line string) int {
	return (x0 + x1 + 1 - draw.Columns(line)) / 2
}

// unpair blanks the other half of a wide character in the cell at x, y
// before the cell is overwritten, so that no half is left to shift the
// row; before is false when the covered cell is rewritten for the wide
// character just written before it
func (c *canvas) unpair(x, y int, before bool) {
	p := c.at(x, y)
	if prev := c.at(x-1, y); before && p.r == covered && prev != nil {
		prev.r = ' '
	}
	if next := c.at(x+1, y); draw.IsWide(p.r) && next != nil && next.r == covered {
		next.r = ' '
	}
}

// covered marks the cell under the second column of a wide character,
// which writing skips
const covered = 0

// fill sets the background of the cells inside the rectangle, unless
// color is the default
func (c *canvas) fill(x0, y0, x1, y1 int, color uint32) {
	if color == 0 {
		return
	}
	for y := y0 + 1; y < y1; y++ {
		for x := x0 + 1; x < x1; x++ {
			if p := c.at(x, y); p != nil {
				p.bg = color
			}
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
func (c *canvas) record(rec *draw.Record, origin layout.Vector, col, row func(layout.Length) int) {
	if len(rec.Fields) == 0 {
		x0, y0 := col(origin.X+layout.Length(rec.X0)), row(origin.Y+layout.Length(rec.Y0))
		x1, y1 := col(origin.X+layout.Length(rec.X1)), row(origin.Y+layout.Length(rec.Y1))
		lines := strings.Split(rec.Text, "\n")
		for i, line := range lines {
			c.text(centered(x0, x1, line), (y0+y1)/2-(len(lines)-1)/2+i, line)
		}
		return
	}
	// dividers first, so that field texts win when rows are too coarse
	for i, field := range rec.Fields {
		if i == 0 {
			continue
		}
		if rec.Vertical {
			y := row(origin.Y + layout.Length(field.Y0))
			for x := col(origin.X+layout.Length(field.X0)) + 1; x < col(origin.X+layout.Length(field.X1)); x++ {
				c.set(x, y, '─')
			}
		} else {
			x := col(origin.X + layout.Length(field.X0))
			for y := row(origin.Y+layout.Length(field.Y0)) + 1; y < row(origin.Y+layout.Length(field.Y1)); y++ {
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

// marker draws an edge end marker pointing in dir at the end of the edge
// in cell end. It sits in the gap before the node when the run there is
// straight, else on the node border. It reports whether style has a
// marker; styles without a marker of their own draw a normal arrowhead.
func (c *canvas) marker(style layout.Arrow, dir int, end [2]int) bool {
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
func arrival(a, b [2]int, startsAtB bool, p, q layout.Vector) int {
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

func absLength(v layout.Length) layout.Length {
	if v < 0 {
		return -v
	}
	return v
}

func opposite(dir int) int { return [right + 1]int{up: down, down: up, left: right, right: left}[dir] }
func dx(dir int) int       { return [right + 1]int{left: -1, right: 1}[dir] }
func dy(dir int) int       { return [right + 1]int{up: -1, down: 1}[dir] }
func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// layoutRecord computes the record fields of a node, measuring text as
// the layout did
func layoutRecord(graph *layout.Graph, node *layout.Node, box layout.NodeBox) *draw.Record {
	var lineWidth func(string) float64
	if graph.MeasureText != nil {
		lineWidth = func(line string) float64 { return float64(graph.MeasureText(line, node.FontName, box.FontSize)) }
	}
	return draw.LayoutRecord(box.Label, float64(box.Size.X), float64(box.Size.Y), float64(graph.LineHeight), float64(box.FontSize), lineWidth)
}
