// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Call Prepare before laying out for output with room for
// the runs; the extra room is carved away again when writing.
package text

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
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

// glyphs holds the line character for every combination of arm weights
// (0 none, 1 light, 2 heavy), indexed by up*27 + down*9 + left*3 + right.
var glyphs = []rune(" ╶╺╴─╼╸╾━╷┌┍┐┬┮┑┭┯╻┎┏┒┰┲┓┱┳╵└┕┘┴┶┙┵┷│├┝┤┼┾┥┽┿╽┟┢┧╁╆┪╅╈╹┖┗┚┸┺┛┹┻╿┞┡┦╀╄┩╃╇┃┠┣┨╂╊┫╉╋")

var rounded = map[rune]rune{'┌': '╭', '┐': '╮', '└': '╰', '┘': '╯'}

// glyph returns the line character with the arms in lines, drawing the
// arms in heavy heavy. A lone arm is drawn as a full straight line.
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

var arrow = map[int]rune{up: '▲', down: '▼', left: '◀', right: '▶'}

type canvas struct {
	w, h   int
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
	for i, r := range []rune(s) {
		c.set(x+i, y, r)
	}
	c.ink = ink
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

// rgb packs a color for a cell: 0 is the terminal default, anything else
// is 1<<24 | red<<16 | green<<8 | blue. Transparent colors are the default.
func rgb(color layout.Color) uint32 {
	if color == nil {
		return 0
	}
	r, g, b, a := color.RGBA8()
	if a == 0 {
		return 0
	}
	return 1<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// Options configure WriteColor.
type Options struct {
	// Palette selects how colors are written.
	Palette Palette
	// Background fills the whole drawing; nil leaves it to the terminal.
	Background layout.Color
}

// Palette selects the colors that WriteColor writes.
type Palette int

const (
	// ANSI16 maps colors to the 16 basic ANSI colors, which terminals draw
	// from their theme, so that the drawing fits light and dark themes.
	// Grays and washed out fills are left to the terminal.
	ANSI16 Palette = iota
	// TrueColor writes colors as given, with 24-bit escape codes.
	TrueColor
)

// codes returns the escape code parameters that select the cell colors
// fg and bg. Lines and text without a color are black or white on a fill
// or background, whichever stands out.
func (opts *Options) codes(fg, bg uint32) (string, string) {
	if opts.Palette == TrueColor {
		code := func(set int, color uint32) string {
			return fmt.Sprintf("%d;2;%d;%d;%d", set, color>>16&0xFF, color>>8&0xFF, color&0xFF)
		}
		if bg == 0 {
			bg = rgb(opts.Background)
		}
		f, b := "39", "49"
		if bg != 0 {
			b = code(48, bg)
			if fg == 0 && lightness(bg) >= 0.5 {
				fg = 1 << 24
			} else if fg == 0 {
				fg = 1<<24 | 0xFFFFFF
			}
		}
		if fg != 0 {
			f = code(38, fg)
		}
		return f, b
	}
	under := bg
	b := ansi16(bg, fill)
	if b == 0 {
		under = rgb(opts.Background)
		b = ansi16(under, ground)
	}
	f := ansi16(fg, ink)
	if b != 0 {
		f = ansi16(fg, inkOnFill)
		if f == 0 && lightness(under) >= 0.5 {
			f = 30
		} else if f == 0 {
			f = 97
		}
	}
	code := func(n int, reset string) string {
		if n == 0 {
			return reset
		}
		return strconv.Itoa(n)
	}
	return code(f, "39"), code(b, "49")
}

// role is what a color is used for when mapping it to the basic colors
type role int

const (
	ink       role = iota // lines and text on the terminal background
	inkOnFill             // lines and text on a fill or background
	fill                  // inside nodes and clusters
	ground                // the background of the whole drawing
)

// ansi16 returns the escape code of the basic ANSI color closest to color
// in its role, or 0 for the terminal default. Themes change the shades
// but keep the hues, so the hue picks the color and lightness picks
// between normal and bright. Grays on the terminal background become the
// default or bright black, which themes keep readable, and dark or light
// ones on a fill become black or bright white. Gray and washed out fills
// are dropped; a gray background picks the closest gray.
func ansi16(color uint32, role role) int {
	if color == 0 {
		return 0
	}
	r := float64(color>>16&0xFF) / 0xFF
	g := float64(color>>8&0xFF) / 0xFF
	b := float64(color&0xFF) / 0xFF
	hi, lo := max(r, g, b), min(r, g, b)
	light := (hi + lo) / 2
	base := 30
	if role >= fill {
		base = 40
	}
	if hi-lo < 0.15 || role == fill && light > 0.85 {
		switch {
		case role == fill:
			return 0
		case role == ground:
			return base + [4]int{0, 60, 7, 67}[min(int(light*4), 3)]
		case role == inkOnFill && light < 0.3:
			return 30
		case role == inkOnFill && light >= 0.7:
			return 97
		case light < 0.3 || light >= 0.7:
			return 0
		}
		return 90
	}
	var hue float64 // in sixths of the circle: red, yellow, green, cyan, blue, magenta
	switch hi {
	case r:
		hue = math.Mod((g-b)/(hi-lo)+6, 6)
	case g:
		hue = (b-r)/(hi-lo) + 2
	default:
		hue = (r-g)/(hi-lo) + 4
	}
	if light > 0.6 {
		base += 60
	}
	return base + [6]int{1, 3, 2, 6, 4, 5}[int(hue+0.5)%6]
}

// lightness returns the HSL lightness of a cell color, in [0, 1]
func lightness(color uint32) float64 {
	r, g, b := color>>16&0xFF, color>>8&0xFF, color&0xFF
	return float64(max(r, g, b)+min(r, g, b)) / (2 * 0xFF)
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
func Write(w io.Writer, graph *layout.Graph) error { return write(w, graph, nil) }

// WriteColor draws the graph like Write, with the colors set on nodes,
// edges and clusters as ANSI escape codes, see Options. Unset colors are
// left to the terminal, except for lines and text on a fill or
// background, which are black or white, whichever stands out.
func WriteColor(w io.Writer, graph *layout.Graph, opts Options) error {
	return write(w, graph, &opts)
}

// write draws the graph, colored unless opts is nil
func write(w io.Writer, graph *layout.Graph, opts *Options) error {
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
	c.fg = make([]uint32, c.w*c.h)
	c.bg = make([]uint32, c.w*c.h)
	c.lines = make([]int, c.w*c.h)
	c.heavy = make([]int, c.w*c.h)
	c.owner = make([][4]int, c.w*c.h)
	c.solid = make([]bool, c.w*c.h)

	c.dashed = true
	for _, cluster := range graph.Clusters {
		x0, y0 := col(cluster.TopLeft.X), row(cluster.TopLeft.Y)
		x1, y1 := col(cluster.BottomRight.X), row(cluster.BottomRight.Y)
		c.edge++
		c.ink = rgb(cluster.LineColor)
		c.fill(x0, y0, x1, y1, rgb(cluster.FillColor))
		c.frame(x0, y0, x1, y1)
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
		last := len(cells) - 1
		cells[0] = border(cells[0], cells[1], edge.From)
		cells[last] = border(cells[last], cells[last-1], edge.To)
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
		// an end without a marker on a box side joins the border, so that
		// the edge visibly leaves the node
		join := func(end [2]int, node *layout.Node) {
			b := boxes[node]
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
		if !c.marker(head, path[len(path)-2], path[len(path)-1], cells[len(cells)-1]) {
			join(cells[len(cells)-1], edge.To)
		}
		if !c.marker(edge.ArrowTail, path[1], path[0], cells[0]) {
			join(cells[0], edge.From)
		}
	}
	for _, edge := range graph.Edges {
		if edge.Label != "" {
			c.font = rgb(edge.FontColor)
			c.text(col(edge.LabelPos.X-edge.LabelRadius.X), row(edge.LabelPos.Y), edge.Label)
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.Label != "" {
			c.font = 0
			c.text(col(cluster.TopLeft.X)+1, row(cluster.TopLeft.Y), " "+cluster.Label+" ")
		}
	}

	grid := make([][]cell, c.h)
	for y := range grid {
		grid[y] = make([]cell, c.w)
		for x := range grid[y] {
			i := y*c.w + x
			grid[y][x] = cell{c.cells[i], c.fg[i], c.bg[i]}
		}
	}
	grid = carve(grid, " │┃┊┋┆", "▲▼●○", 1)
	grid = transpose(carve(transpose(grid), " ─━┈┉┄", "◀▶●○", 2))

	var out strings.Builder
	for _, line := range grid {
		fs, bs := make([]string, len(line)), make([]string, len(line))
		end := 0
		for i, x := range line {
			fs[i], bs[i] = "39", "49"
			if opts != nil {
				fs[i], bs[i] = opts.codes(x.fg, x.bg)
			}
			if x.r != ' ' || bs[i] != "49" {
				end = i + 1
			}
		}
		f, b := "39", "49"
		for i, x := range line[:end] {
			want := fs[i]
			if x.r == ' ' {
				want = f // blanks show only the background
			}
			if want != f || bs[i] != b {
				f, b = want, bs[i]
				out.WriteString("\x1b[" + f + ";" + b + "m")
			}
			out.WriteRune(x.r)
		}
		if f != "39" || b != "49" {
			out.WriteString("\x1b[0m")
		}
		out.WriteByte('\n')
	}
	_, err := io.WriteString(w, out.String())
	return err
}

// carve removes rows that only continue straight lines or blanks and
// repeat the row before them, keeping at most keep of every such run.
// Removing them keeps the drawing connected, just tighter.
func carve(grid [][]cell, straight, markers string, keep int) [][]cell {
	out := grid[:0:0]
	run := 0
	for i, row := range grid {
		plain := true
		for _, x := range row {
			if !strings.ContainsRune(straight, x.r) {
				plain = false
				break
			}
		}
		// a marker on a run continues it like the line it sits on
		same := func(x, prev cell) bool { return x.r == prev.r || x.r != ' ' && strings.ContainsRune(markers, prev.r) }
		if plain && i > 0 && slices.EqualFunc(row, grid[i-1], same) {
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

// cell is a drawn character with its colors, see rgb
type cell struct {
	r      rune
	fg, bg uint32
}

func transpose(grid [][]cell) [][]cell {
	if len(grid) == 0 {
		return nil
	}
	out := make([][]cell, len(grid[0]))
	for x := range out {
		out[x] = make([]cell, len(grid))
		for y := range grid {
			out[x][y] = grid[y][x]
		}
	}
	return out
}
