package text

import (
	"iter"
	"math/bits"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// canvas is the character grid the graph is drawn on, with what is
// needed to join lines and color the cells
type canvas struct {
	w, h   int
	cellW  layout.Length // size of a cell in graph units
	cellH  layout.Length
	origin layout.Vector // graph coordinates of the top left cell
	l      *layout.Layout
	boxes  map[*layout.Node]cellBox // drawn node boxes: x0, y0, x1, y1
	rows   grid
	pen    pen                     // what is drawn with from now on
	ids    edgeID                  // edge ids handed out
	merged map[*layout.Edge]edgeID // edge ids of merged edges
	drawn  map[*layout.Edge]edgeID // edge ids of the edges drawn
	owners []edgeID                // edge id of each label id, from 1, see cell.text
	nodes  map[*layout.Node]nodeID // node ids, from 1, see cell.node
	loops  map[*layout.Node]int    // self-loops per node
	ended  map[point]bool          // cells where merged edges have ended
	// cells where edges end at fields, by edge id, see reserve
	reserved map[point]edgeID
	// cells out from edge ends where their markers go, see markCell
	markers map[point]edgeID
	// diamonds drawn with a point for each end, see nodeBox
	pointed map[*layout.Node]bool
	// no characters of Symbols for Legacy Computing, see Options
	safe   bool
	spread bool // edge ends on a side keep a cell apart where there is room
	// palette holds the colors cells refer to by index, see color;
	// colors finds the index of a color
	palette []uint32
	colors  map[uint32]colorID
}

// color returns the index of the color, see rgb, in the palette; 0 for
// the default
func (c *canvas) color(rgb uint32) colorID {
	i, ok := c.colors[rgb]
	if !ok {
		i = colorID(len(c.palette))
		c.palette = append(c.palette, rgb)
		c.colors[rgb] = i
	}
	return i
}

// colorOf returns the index of color in the palette, see color
func (c *canvas) colorOf(color layout.Color) colorID { return c.color(rgb(color)) }

// pen is what the canvas draws with. Each drawing sets all of it, so
// that nothing carries over from the drawing before.
type pen struct {
	ink    colorID // color of lines and marks
	font   colorID // color of text
	dashed bool    // straight runs are dashed
	edge   edgeID  // edge drawn, so that overlaps show
	frame  bool    // the frame of a cluster, see cell.frame
}

// newCanvas returns an empty canvas that fits the graph. One character
// cell is graph.FontSize*0.55 wide and graph.LineHeight tall.
func newCanvas(l *layout.Layout, safe bool) *canvas {
	graph := l.Graph
	c := &canvas{
		l: l, safe: safe,
		cellW: graph.FontSize * 0.55, cellH: graph.LineHeight,
		boxes: map[*layout.Node]cellBox{}, ended: map[point]bool{}, pointed: map[*layout.Node]bool{},
		palette: []uint32{0}, colors: map[uint32]colorID{0: 0},
	}
	if c.cellW <= 0 {
		c.cellW = 8
	}
	if c.cellH <= 0 {
		c.cellH = 16
	}

	// the drawing can reach before the origin, with labels nudged there
	// or pinned and force layouts; keep the usual margin otherwise
	var bottomRight layout.Vector
	c.origin, bottomRight = l.Extent()
	c.w, c.h = c.col(bottomRight.X)+2, c.row(bottomRight.Y)+2
	for i := range graph.Edges {
		label, x, _ := c.edgeLabel(i)
		c.w = max(c.w, x+draw.TextColumns(label)+1)
	}
	c.drawn = map[*layout.Edge]edgeID{}
	c.nodes = map[*layout.Node]nodeID{}
	c.loops = map[*layout.Node]int{}
	c.owners = []edgeID{0}
	for i, node := range graph.Nodes {
		c.nodes[node] = nodeID(i + 1)
	}
	c.merged = mergedEdges(l)
	for _, edge := range graph.Edges {
		if edge.From == edge.To {
			c.loops[edge.From]++
		}
	}

	// boxes can be wider than the layout's, see nodeBox and clusterBox
	fit := func(b cellBox) { c.w, c.h = max(c.w, b.Max.X+1), max(c.h, b.Max.Y+1) }
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
	return c.l.Graph.RankDir.Sideways()
}

// reset makes the canvas w by h blank cells
func (c *canvas) reset(w, h int) {
	c.w, c.h = w, h
	c.rows = make(grid, h)
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
func (c *canvas) put(x, y int, r rune, fg colorID) {
	if p := c.at(x, y); p != nil {
		c.unpair(x, y, r != covered)
		p.r = r
		p.fg = fg
		p.lines = 0
		p.heavy = 0
		p.style = 0
	}
}

func (c *canvas) line(x, y int, mask uint8) {
	p := c.at(x, y)
	if p == nil || p.solid {
		return
	}
	c.unpair(x, y, true)
	for arm := range 4 {
		bit := uint8(1) << arm
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
	if p.crossed() && p.heavy == 0 {
		p.r = c.crossing(mask, p) // two lines crossing, not joining
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

// crossing returns the glyph where the line in mask crosses p, with the
// line on top heavy: the edge across a cluster frame, else the vertical one
func (c *canvas) crossing(mask uint8, p *cell) rune {
	across := uint8(up | down) // the axis of the line on top
	switch {
	case c.pen.frame:
		across = (up | down | left | right) &^ axisOf(mask)
	case p.frame:
		across = axisOf(mask)
	}
	if across == left|right {
		return '┿'
	}
	return '╂'
}

// axisOf returns the arms along the axis of the arms in mask
func axisOf(mask uint8) uint8 {
	if mask&(left|right) != 0 {
		return left | right
	}
	return up | down
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

// text writes s from x, y in the color of the pen's font, or of its
// spans, see draw.Spans, with their styles
func (c *canvas) text(x, y int, s string) {
	for _, span := range draw.Spans(s) {
		fg := c.pen.font
		if color, ok := layout.ParseColor(span.Style.Color); ok {
			fg = c.colorOf(color)
		}
		style := textStyle(span.Style)
		for _, r := range cellRunes(span.Text) {
			c.put(x, y, r, fg)
			if p := c.at(x, y); p != nil {
				p.style = style
			}
			c.hold(x, y)
			x++
		}
	}
}

// the bits of cell.style, which the escape codes of colored text show,
// and pinned, which marks the end of an edge at a field, see canvas.pin
const (
	bold = 1 << iota
	italic
	underline
	strike
	pinned = 1 << 7
)

// textStyle returns the bits of cell.style of a style of text
func textStyle(style draw.Style) uint8 {
	var bits uint8
	for _, on := range []struct {
		set bool
		bit uint8
	}{{style.Bold, bold}, {style.Italic, italic}, {style.Underline, underline}, {style.Strike, strike}} {
		if on.set {
			bits |= on.bit
		}
	}

	return bits
}

// cellRunes returns the characters of s by cell, with covered after a
// wide character. A cell holds one character, so marks on it are
// dropped, and so are control characters, which would garble the
// terminal.
func cellRunes(s string) []rune {
	var runes []rune
	for _, r := range s {
		switch {
		case draw.IsZeroWidth(r), unicode.IsControl(r):
		case draw.IsWide(r):
			runes = append(runes, r, covered)
		default:
			runes = append(runes, r)
		}
	}

	return runes
}

// hold keeps lines out of the cell at x, y, see free
func (c *canvas) hold(x, y int) {
	if p := c.at(x, y); p != nil {
		p.keep = true
	}
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
func (c *canvas) fill(x0, y0, x1, y1 int, color colorID) {
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

// walk draws a run from (x0,y0) to (x1,y1) as a horizontal then a
// vertical leg, see drawStep
func (c *canvas) walk(x0, y0, x1, y1 int) {
	c.steps(point{x0, y0}, point{x1, y1}, c.drawStep)
}

// drawStep draws the line of a step from cell to cell, see steps
func (c *canvas) drawStep(from, to point, dir uint8) {
	c.line(from.X, from.Y, dir)
	c.line(to.X, to.Y, opposite(dir))
}

// cell is a drawn character with its colors, see canvas.color, and what
// is needed to join the lines drawn through it
//
// Tidying copies cells, so they are kept to 32 bytes: ids
// are int16, see fits, colors index the palette, and line masks are
// bytes; the fields go from the widest to the narrowest, so that none
// needs padding.
type cell struct {
	r      rune
	owner  [4]edgeID // edge that first drew each arm
	fg, bg colorID
	need   int16     // the length a run of these must keep, for a label on it
	label  clusterID // the cluster whose label starts here
	node   nodeID    // node whose box covers it
	text   edgeID    // label it is part of, see canvas.owners
	lines  uint8     // direction mask, for joining edge runs
	heavy  uint8     // arms that runs of different edges share
	style  uint8     // the style bits of text, see textStyle, or pinned
	keep   bool      // inside a node or of text, which lines keep out of
	glue   bool      // beside a label, which lines keep out of
	solid  bool      // covered by a node; edges do not draw there
	frame  bool      // of the frame of a cluster
}

// ownerOn returns the edge that drew the arm of c toward dir
func (c *cell) ownerOn(dir uint8) edgeID { return c.owner[bits.TrailingZeros8(dir)] }

// drawnBy reports whether c has all the arms in arms, each drawn by id
func (c *cell) drawnBy(id edgeID, arms uint8) bool {
	if c == nil || c.lines&arms != arms {
		return false
	}
	for arm := uint8(up); arm <= right; arm <<= 1 {
		if arms&arm != 0 && c.ownerOn(arm) != id {
			return false
		}
	}
	return true
}

// setOwner sets id as the edge that drew each arm of c in arms
func (c *cell) setOwner(arms uint8, id edgeID) {
	for arm := range 4 {
		if arms&(1<<arm) != 0 {
			c.owner[arm] = id
		}
	}
}

// linesBy reports whether c has an arm drawn by id, and one drawn by
// another edge
func (c *cell) linesBy(id edgeID) (own, other bool) {
	for arm := range 4 {
		if c.lines&(1<<arm) != 0 {
			own = own || c.owner[arm] == id
			other = other || c.owner[arm] != id
		}
	}
	return own, other
}

// crossed reports whether two edges cross in c without joining: it has
// all four arms, up and down drawn by one edge and left and right by
// another
func (c *cell) crossed() bool {
	return c.lines == up|down|left|right && c.ownerOn(up) == c.ownerOn(down) &&
		c.ownerOn(left) == c.ownerOn(right) && c.ownerOn(up) != c.ownerOn(left)
}

// edgeID tells edges apart in cells, from 1, and the frames of clusters,
// which take ids after the edges; edges that merge share a negative id,
// see mergedEdges. Ids are int16 to keep cells small, see fits.
type edgeID int16

// nodeID is the id of a node in cells, from 1, see canvas.nodes
type nodeID int16

// clusterID is the id of a cluster in cells, from 1
type clusterID int16

// colorID indexes canvas.palette, 0 for the default color
type colorID uint16

// grid is the cells of a drawing, by row
type grid [][]cell

// at returns the cell at row r and column x, or nil off the grid
func (g grid) at(r, x int) *cell {
	if r < 0 || r >= len(g) || x < 0 || x >= len(g[r]) {
		return nil
	}
	return &g[r][x]
}

// point is the place of a cell: its column X and its row Y
type point struct{ X, Y int }

func (p point) add(q point) point { return point{p.X + q.X, p.Y + q.Y} }
func (p point) sub(q point) point { return point{p.X - q.X, p.Y - q.Y} }
func (p point) isZero() bool      { return p == point{} }

// dist returns the steps along rows and columns from p to q
func (p point) dist(q point) int { d := p.sub(q); return abs(d.X) + abs(d.Y) }

// step returns the cell next to p toward dir
func (p point) step(dir uint8) point { return point{p.X + dx(dir), p.Y + dy(dir)} }

// at returns the coordinate of p on axis: X for 0, Y for 1
func (p point) at(axis int) int {
	if axis == 0 {
		return p.X
	}
	return p.Y
}

// set sets the coordinate of p on axis to v
func (p *point) set(axis, v int) {
	if axis == 0 {
		p.X = v
	} else {
		p.Y = v
	}
}

// sameRow reports whether a and b are on one row
func sameRow(a, b point) bool { return a.Y == b.Y }

// sameCol reports whether a and b are in one column
func sameCol(a, b point) bool { return a.X == b.X }

// straight reports whether a and b are on one row or in one column
func straight(a, b point) bool { return sameRow(a, b) || sameCol(a, b) }

// inLine reports whether b is no corner between a and c: the steps to and
// from it keep to rows and columns alike
func inLine(a, b, c point) bool {
	return sameCol(a, b) == sameCol(b, c) && sameRow(a, b) == sameRow(b, c)
}

// cellBox is the cells from Min to Max, both included
type cellBox struct{ Min, Max point }

// sizedBox returns the box of w by h cells from x, y
func sizedBox(x, y, w, h int) cellBox { return cellBox{point{x, y}, point{x + w - 1, y + h - 1}} }

// grow returns b with n more cells on every side
func (b cellBox) grow(n int) cellBox {
	return cellBox{b.Min.sub(point{n, n}), b.Max.add(point{n, n})}
}

// contains reports whether p is in b, on its frame or inside it
func (b cellBox) contains(p point) bool {
	return p.X >= b.Min.X && p.X <= b.Max.X && p.Y >= b.Min.Y && p.Y <= b.Max.Y
}

// inside reports whether p is inside the frame of b
func (b cellBox) inside(p point) bool { return b.insideX(p.X) && b.insideY(p.Y) }

// insideX reports whether x is between the left and right sides of b
func (b cellBox) insideX(x int) bool { return x > b.Min.X && x < b.Max.X }

// insideY reports whether y is between the top and bottom sides of b
func (b cellBox) insideY(y int) bool { return y > b.Min.Y && y < b.Max.Y }

// sideOf returns the side of b that p lies straight out from, as the step
// from b toward p, and the cells between them; zero when p is inside b or
// off its corners
func (b cellBox) sideOf(p point) (point, int) {
	overX := p.X >= b.Min.X && p.X <= b.Max.X
	overY := p.Y >= b.Min.Y && p.Y <= b.Max.Y
	switch {
	case overX && p.Y < b.Min.Y:
		return point{0, -1}, b.Min.Y - p.Y - 1
	case overX && p.Y > b.Max.Y:
		return point{0, 1}, p.Y - b.Max.Y - 1
	case overY && p.X < b.Min.X:
		return point{-1, 0}, b.Min.X - p.X - 1
	case overY && p.X > b.Max.X:
		return point{1, 0}, p.X - b.Max.X - 1
	}
	return point{}, 0
}

// overlaps reports whether b and o share a cell
func (b cellBox) overlaps(o cellBox) bool {
	return b.Min.X <= o.Max.X && o.Min.X <= b.Max.X && b.Min.Y <= o.Max.Y && o.Min.Y <= b.Max.Y
}

// cells yields the cells of b, row by row
func (b cellBox) cells() iter.Seq[point] {
	return func(yield func(point) bool) {
		for y := b.Min.Y; y <= b.Max.Y; y++ {
			for x := b.Min.X; x <= b.Max.X; x++ {
				if !yield(point{x, y}) {
					return
				}
			}
		}
	}
}
