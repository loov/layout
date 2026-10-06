package text

import (
	"slices"
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
	boxes  map[*layout.Node][4]int // drawn node boxes: x0, y0, x1, y1
	rows   grid
	pen    pen                    // what is drawn with from now on
	ids    int32                  // edge ids handed out
	merged map[*layout.Edge]int32 // edge ids of merged edges
	drawn  map[*layout.Edge]int32 // edge ids of the edges drawn
	nodes  map[*layout.Node]int32 // node ids, from 1, see cell.node
	loops  map[*layout.Node]int   // self-loops per node
	ended  map[[2]int]bool        // cells where merged edges have ended
	spread bool                   // edge ends on a side keep a cell apart where there is room
}

// pen is what the canvas draws with. Each drawing sets all of it, so
// that nothing carries over from the drawing before.
type pen struct {
	ink    uint32 // color of lines and marks
	font   uint32 // color of text
	dashed bool   // straight runs are dashed
	edge   int32  // edge drawn, so that overlaps show
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
	c.drawn, c.nodes, c.loops = map[*layout.Edge]int32{}, map[*layout.Node]int32{}, map[*layout.Node]int{}
	for i, node := range graph.Nodes {
		c.nodes[node] = int32(i + 1)
	}
	if graph.MergeEdges {
		c.merged = mergedEdges(l)
	}
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
func (c *canvas) put(x, y int, r rune, fg uint32) {
	if p := c.at(x, y); p != nil {
		c.unpair(x, y, r != covered)
		p.r, p.fg, p.lines, p.heavy = r, fg, 0, 0
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
	for i, r := range cellRunes(s) {
		c.put(x+i, y, r, c.pen.font)
		c.hold(x+i, y)
	}
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

// hold keeps the cell at x, y from being carved away
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

// walk draws a run from (x0,y0) to (x1,y1) as a horizontal then a
// vertical leg.
func (c *canvas) walk(x0, y0, x1, y1 int) {
	step := func(x, y int, dir, back uint8) {
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

// cell is a drawn character with its colors, see rgb, and what is
// needed to join the lines drawn through it
//
// Carving copies cells over and over, so they are kept small: ids are
// int32 and line masks bytes, the 4-byte fields go first and the bytes
// last, so that none needs padding.
type cell struct {
	r      rune
	fg, bg uint32
	owner  [4]int32 // edge that first drew each arm
	need   int32    // the length a run of these must keep, for a label on it
	label  int32    // the cluster whose label starts here, from 1
	node   int32    // node whose box covers it, from 1, see canvas.nodes
	text   int32    // edge whose label it is part of, see canvas.drawn
	lines  uint8    // direction mask, for joining edge runs
	heavy  uint8    // arms that runs of different edges share
	kind   uint8    // class of r, which seams keep up to date for their use
	keep   bool     // inside a node or of text, which carving keeps
	glue   bool     // beside a label, which carving keeps beside it
	solid  bool     // covered by a node; edges do not draw there
	frame  bool     // of the frame of a cluster
}

// grid is the cells of a drawing, by row
type grid [][]cell

// at returns the cell at row r and column x, or nil off the grid
func (g grid) at(r, x int) *cell {
	if r < 0 || r >= len(g) || x < 0 || x >= len(g[r]) {
		return nil
	}
	return &g[r][x]
}

// scratch holds the blocks of cells that carving is done with, for the
// grids it makes next
type scratch struct{ free [][]cell }

// take returns a block of n cells, the smallest free one that fits
func (sc *scratch) take(n int) []cell {
	best := -1
	for i, s := range sc.free {
		if cap(s) >= n && (best < 0 || cap(s) < cap(sc.free[best])) {
			best = i
		}
	}
	if best < 0 {
		return make([]cell, n)
	}
	s := sc.free[best]
	sc.free = slices.Delete(sc.free, best, best+1)
	return s[:n]
}

// release gives back a block that no grid uses any more
func (sc *scratch) release(s []cell) {
	if s != nil {
		sc.free = append(sc.free, s)
	}
}

// transpose returns g with its rows as columns, and the block of cells it
// is in
func (sc *scratch) transpose(g grid) (grid, []cell) {
	if len(g) == 0 {
		return nil, nil
	}
	out := make(grid, len(g[0]))
	// one block for all rows, each with room for a cell more, which seams
	// adds to every row
	n := len(g) + 1
	cells := sc.take(len(out) * n)
	for x := range out {
		out[x] = cells[x*n : x*n+n-1 : x*n+n]
		for y := range g {
			out[x][y] = g[y][x]
		}
	}
	return out, cells
}
