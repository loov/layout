package text

import (
	"math"
	"slices"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// clusterLabel returns the label of a cluster on one line, as it is
// drawn along the top of the frame
func clusterLabel(cluster *layout.Cluster) string {
	var texts []string
	for _, line := range draw.Lines(draw.PlainLabel(cluster.Label)) {
		texts = append(texts, line.Text)
	}
	return strings.Join(texts, " ")
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

// edgeText writes the lines of a label of edge, w cells wide, at x, y
func (c *canvas) edgeText(edge *layout.Edge, lines []draw.Line, x, y, w int) {
	c.pen = pen{font: c.color(rgb(edge.FontColor))}
	// each label its own id, so that hug moves it alone: merged edges,
	// and an edge's head and tail labels, share the edge's id
	c.owners = append(c.owners, c.drawn[edge])
	label := edgeID(len(c.owners) - 1)
	for k, line := range lines {
		// within the block of the lines, as there is no box
		at := x + int(line.Align.Offset(float64(w), float64(draw.Columns(line.Text))))
		c.text(at, y+k, line.Text)
		for col := at; col < at+draw.Columns(line.Text); col++ {
			if p := c.at(col, y+k); p != nil {
				p.text = label
			}
		}
	}
	// the blanks around the label keep lines from running into it, see
	// free
	for row := y - 1; row <= y+len(lines); row++ {
		for col := x - 1; col <= x+w; col++ {
			if p := c.at(col, row); p != nil && p.r == ' ' && !p.solid {
				p.glue = true
			}
		}
	}
}

// endLabel writes a head or tail label of edge right beside its line,
// drawn along path from the end it labels, as near that end as there are
// blank cells for it, on the side nearer center, where the layout put it.
// Where no place along the line has a blank ring around the text, as among
// many lines side by side, the text goes where it has its own cells, and
// failing that in the nearest blank cells.
func (c *canvas) endLabel(edge *layout.Edge, label string, center layout.Vector, path [][2]int) {
	if label == "" {
		return
	}
	label = draw.PlainLabel(label)
	lines := draw.Lines(label)
	w, h := draw.TextColumns(label), len(lines)
	want := [2]int{c.col(center.X), c.row(center.Y)}
	// blank cells for the text, and around them no box nor other line
	// than its own edge's
	id := c.drawn[edge]
	ring := true
	blank := func(x, y int) bool {
		for row := y - 1; row <= y+h; row++ {
			for col := x - 1; col <= x+w; col++ {
				p := c.at(col, row)
				inside := row >= y && row < y+h && col >= x && col < x+w
				switch {
				case p == nil:
					return false
				case inside && (p.r != ' ' || p.solid || p.glue || p.lines != 0):
					return false
				case !inside && ring && (p.solid || p.text != 0):
					return false
				}
				for arm := range 4 {
					if !inside && ring && p.lines&(1<<arm) != 0 && p.owner[arm] != id {
						return false
					}
				}
			}
		}
		return true
	}
	// the cells of the line from the end, past the end on the box
	var cells [][2]int
	for i := 0; i+1 < len(path); i++ {
		c.steps(path[i], path[i+1], func(_, to [2]int, _ uint8) { cells = append(cells, to) })
	}
	// past halfway it reads as the other end's
	cells = cells[:(len(cells)+1)/2]
	for _, ring = range []bool{true, false} {
		if c.endText(edge, lines, w, h, want, cells, blank) {
			return
		}
	}
	// no room beside its line near the end: the nearest blank cells
	x, y := c.nearestBlank(want[0]-w/2, want[1]-h/2, w, h)
	c.edgeText(edge, lines, x, y, w)
}

// endText writes lines, w cells wide and h tall, beside the first of cells
// where blank has room for them, the side nearer want first; it reports
// whether it found one
func (c *canvas) endText(edge *layout.Edge, lines []draw.Line, w, h int, want [2]int, cells [][2]int, blank func(x, y int) bool) bool {
	for k, p := range cells {
		if k+1 < len(cells) && k > 0 && cells[k-1][0] != cells[k+1][0] && cells[k-1][1] != cells[k+1][1] {
			continue // a turn
		}
		// beside the line: on either side of a column, above or below
		// a row, the side nearer where the layout put it first
		var spots [][2]int
		if k+1 < len(cells) && cells[k+1][0] == p[0] || k > 0 && cells[k-1][0] == p[0] {
			spots = [][2]int{{p[0] + 1, p[1] - h/2}, {p[0] - w, p[1] - h/2}}
		} else {
			spots = [][2]int{{p[0] - w/2, p[1] - h}, {p[0] - w/2, p[1] + 1}}
		}
		dist := func(s [2]int) int {
			return max(s[0]+w/2-want[0], want[0]-s[0]-w/2) + max(s[1]+h/2-want[1], want[1]-s[1]-h/2)
		}
		if dist(spots[1]) < dist(spots[0]) {
			spots[0], spots[1] = spots[1], spots[0]
		}
		for _, s := range spots {
			if blank(s[0], s[1]) {
				c.edgeText(edge, lines, s[0], s[1], w)
				return true
			}
		}
	}
	return false
}

// reversed returns the cells of a path from its last to its first
func reversed(path [][2]int) [][2]int {
	out := slices.Clone(path)
	slices.Reverse(out)
	return out
}

// centered returns the column that centers line between the columns x0
// and x1 of the borders around it
func centered(x0, x1 int, line string) int {
	return (x0 + x1 + 1 - draw.Columns(line)) / 2
}

// placed returns the column that line starts at between the columns x0
// and x1 of the borders around it: centered, or a space in from the side
// it goes against
func placed(x0, x1 int, line draw.Line) int {
	switch line.Align {
	case draw.Left:
		return x0 + 2
	case draw.Right:
		return x1 - 1 - draw.Columns(line.Text)
	}
	return centered(x0, x1, line.Text)
}

// drawLabels writes edge and cluster labels over everything else; paths
// are the cells of the edges
func (c *canvas) drawLabels(paths [][][2]int) {
	graph := c.l.Graph
	for i, edge := range graph.Edges {
		if edge.Label != "" && !edge.Invisible {
			label, x, y := c.edgeLabel(i)
			lines := draw.Lines(label)
			w := draw.TextColumns(label)
			x, y = c.besideEdge(x, y, w, len(lines), paths[i], c.drawn[edge])
			c.edgeText(edge, lines, x, y, w)
		}
	}
	for i, edge := range graph.Edges {
		if edge.Invisible || len(paths[i]) < 2 {
			continue
		}
		at := c.l.Edges[i]
		c.endLabel(edge, edge.HeadLabel, at.HeadLabelCenter, reversed(paths[i]))
		c.endLabel(edge, edge.TailLabel, at.TailLabelCenter, paths[i])
	}
	for i, cluster := range graph.Clusters {
		if cluster.Label == "" || cluster.Invisible {
			continue
		}
		// the label goes on the frame after tidying, see frameLabels, at
		// the first place along the top, else along the bottom, where the
		// lines that cross the frame fall on spaces of it; tidying keeps
		// those cells
		b := c.clusterBox(i)
		text := " " + clusterLabel(cluster) + " "
		runes := cellRunes(text)
		fits := func(x, y int) bool {
			for j, r := range runes {
				p := c.at(x+j, y)
				if p == nil || x+j >= b[2] || !dashedHorizontal(p.r) && !(r == ' ' && p.lines&(up|down) != 0) {
					return false
				}
			}
			return true
		}
		placed := false
		for _, y := range []int{b[1], b[3]} {
			for x := b[0] + 1; x < b[2] && !placed; x++ {
				if fits(x, y) {
					for j := range runes {
						c.at(x+j, y).need = int16(len(runes))
					}
					c.at(x, y).label = clusterID(i + 1)
					placed = true
				}
			}
		}
		if placed {
			continue
		}
		c.pen = pen{}
		c.text(b[0]+1, b[1], text)
	}
}

// frameLabels writes the cluster labels at the cells marked for them on the
// tidied grid, see drawLabels
func (c *canvas) frameLabels(g grid) {
	for _, row := range g {
		for x := range row {
			id := row[x].label
			if id == 0 {
				continue
			}
			for _, r := range cellRunes(" " + clusterLabel(c.l.Graph.Clusters[id-1]) + " ") {
				if row[x].lines&(up|down) == 0 {
					// a line that crosses the frame, on a space, stays
					row[x].r, row[x].fg = r, 0
				}
				x++
			}
		}
	}
}

// besideEdge places a label w cells wide and h tall at x, y against its
// edge, drawn along path with id: where nearEdge moves it, or against the
// edge from another side where the lines of other edges are further from
// it, so that it reads as its edge's
func (c *canvas) besideEdge(x, y, w, h int, path [][2]int, id edgeID) (int, int) {
	bx, by, against := c.nearEdge(x, y, w, h, path, [2]int{})
	best := c.apart(bx, by, w, h, id)
	for _, side := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		if against && best >= 2 {
			// against its edge with two blank cells to any other, which
			// is clearly its own; moving it gains nothing and can keep
			// tidying from narrowing the drawing
			break
		}
		if sx, sy, ok := c.nearEdge(x, y, w, h, path, side); ok {
			if d := c.apart(sx, sy, w, h, id); d > best {
				bx, by, best = sx, sy, d
			}
		}
	}
	if !c.blank(bx, by, w, h) {
		// the text would cover lines, which cuts the edges: the nearest
		// blank place right beside its own edge instead, see freeBeside
		bx, by = c.freeBeside(x, y, w, h, path)
	}
	return bx, by
}

// freeBeside returns the place for a label w cells wide and h tall,
// nearest x, y, where it covers nothing: right beside its edge, drawn
// along path, on either side of a column of it or above or below a row,
// or else anywhere; x, y when there is none
func (c *canvas) freeBeside(x, y, w, h int, path [][2]int) (int, int) {
	var cells [][2]int
	for i := 0; i+1 < len(path); i++ {
		c.steps(path[i], path[i+1], func(from, _ [2]int, _ uint8) { cells = append(cells, from) })
	}
	type spot struct{ x, y, dist int }
	var spots []spot
	add := func(sx, sy int) {
		spots = append(spots, spot{sx, sy, max(sx-x, x-sx) + max(sy-y, y-sy)})
	}
	for k, p := range cells {
		vertical := k+1 < len(cells) && cells[k+1][0] == p[0] || k > 0 && cells[k-1][0] == p[0]
		if vertical {
			for dy := range h {
				add(p[0]+1, p[1]-dy)
				add(p[0]-w, p[1]-dy)
			}
		} else {
			for dx := range w {
				add(p[0]-dx, p[1]-h)
				add(p[0]-dx, p[1]+1)
			}
		}
	}
	slices.SortStableFunc(spots, func(a, b spot) int { return a.dist - b.dist })
	for _, s := range spots {
		if c.blank(s.x, s.y, w, h) {
			return s.x, s.y
		}
	}
	// nowhere beside the edge
	return c.nearestBlank(x, y, w, h)
}

// nearestBlank returns the place for a text w cells wide and h tall
// nearest x, y where it covers nothing; x, y when there is none
func (c *canvas) nearestBlank(x, y, w, h int) (int, int) {
	for d := 0; d < c.w+c.h; d++ {
		for dy := -d; dy <= d; dy++ {
			for _, dx := range []int{d - max(dy, -dy), max(dy, -dy) - d} {
				if c.blank(x+dx, y+dy, w, h) {
					return x + dx, y + dy
				}
			}
		}
	}
	return x, y
}

// apart returns how many blank cells at least separate a label w cells
// wide and h tall at x, y from the lines of edges other than id, up to 3
func (c *canvas) apart(x, y, w, h int, id edgeID) int {
	for d := range 3 {
		for row := y - d - 1; row <= y+h+d; row++ {
			for col := x - d - 1; col <= x+w+d; col++ {
				if row >= y-d && row < y+h+d && col >= x-d && col < x+w+d {
					continue // nearer, see the rings before
				}
				p := c.at(col, row)
				if p == nil || p.lines == 0 || p.frame {
					continue
				}
				for arm := range 4 {
					if p.lines&(1<<arm) != 0 && p.owner[arm] != id {
						return d
					}
				}
			}
		}
	}
	return 3
}

// nearEdge moves a label w cells wide and h tall at x, y toward its edge,
// drawn along path, toward side when it is set, while a blank row or
// column separates them and the cells it moves into are blank. It reports
// whether the label ends against the edge. The layout keeps labels an
// edge padding from their edge, which is a whole row in text, and
// rounding can leave a blank row between them.
func (c *canvas) nearEdge(x, y, w, h int, path [][2]int, side [2]int) (int, int, bool) {
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
			if side != [2]int{} {
				// only the cells on that side
				above, below := p[0] >= x && p[0] < x+w && p[1] < y, p[0] >= x && p[0] < x+w && p[1] >= y+h
				left, right := beside && p[0] < x, beside && p[0] >= x+w
				if !(side[1] < 0 && above || side[1] > 0 && below || side[0] < 0 && left || side[0] > 0 && right) {
					continue
				}
			}
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
				if d := x - p[0] - 1; d < gap {
					dx, dy, gap = -1, 0, d
				}
			case beside && p[0] >= x+w:
				if d := p[0] - (x + w); d < gap {
					dx, dy, gap = 1, 0, d
				}
			}
		}
		if gap <= 0 || gap == math.MaxInt || !c.blank(x+dx, y+dy, w, h) {
			return x, y, gap <= 0
		}
		x, y = x+dx, y+dy
	}
	return x, y, false
}

// blank reports whether the w by h cells from x, y are empty
func (c *canvas) blank(x, y, w, h int) bool {
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			if p := c.at(col, row); p == nil || p.r != ' ' || p.solid {
				return false
			}
		}
	}
	return true
}

// hug moves a label that another edge's line runs right beside a cell
// over onto its own edge's line, where tidying leaves that a blank cell
// away from it: next to the other line, it should touch its own. A label
// that no line of its own edge comes within two cells of, as straighten
// slides lines without their labels, moves across blanks to the nearest
// one, a blank short of it. owners holds the edge of each label id.
func hug(g grid, owners []edgeID) {
	// the lines of edge id, and of other edges, in the cells
	lines := func(cells [][2]int, id edgeID) (own, other bool) {
		for _, p := range cells {
			c := g.at(p[0], p[1])
			if c == nil || c.frame || c.text != 0 {
				continue
			}
			for arm := range 4 {
				if c.lines&(1<<arm) != 0 {
					own = own || c.owner[arm] == id
					other = other || c.owner[arm] != id
				}
			}
		}
		return own, other
	}
	// the box of every label, and of its characters without the blanks
	// around them, and the labels in reading order, which is the order
	// they move in, as one can take the cell another wants
	boxes := map[edgeID][4]int{} // top, left, bottom, right
	inked := map[edgeID][4]int{}
	grow := func(boxes map[edgeID][4]int, id edgeID, r, x int) {
		b, ok := boxes[id]
		if !ok {
			b = [4]int{r, x, r, x}
		}
		boxes[id] = [4]int{min(b[0], r), min(b[1], x), max(b[2], r), max(b[3], x)}
	}
	var ids []edgeID
	for r, row := range g {
		for x, c := range row {
			if c.text == 0 {
				continue
			}
			if _, ok := boxes[c.text]; !ok {
				ids = append(ids, c.text)
			}
			grow(boxes, c.text, r, x)
			if c.r != ' ' {
				grow(inked, c.text, r, x)
			}
		}
	}
	// the cells of a side of b, n cells out
	side := func(b [4]int, dir uint8, n int) [][2]int {
		var cells [][2]int
		switch dir {
		case left, right:
			x := b[1] - n
			if dir == right {
				x = b[3] + n
			}
			for r := b[0]; r <= b[2]; r++ {
				cells = append(cells, [2]int{r, x})
			}
		default:
			r := b[0] - n
			if dir == down {
				r = b[2] + n
			}
			for x := b[1]; x <= b[3]; x++ {
				cells = append(cells, [2]int{r, x})
			}
		}
		return cells
	}
	// shift moves the cells of b n cells toward dir
	shift := func(b [4]int, dir uint8, n int) {
		cells := []cell{}
		for r := b[0]; r <= b[2]; r++ {
			for x := b[1]; x <= b[3]; x++ {
				cells = append(cells, g[r][x])
				g[r][x] = cell{r: ' ', glue: true}
			}
		}
		i := 0
		for r := b[0]; r <= b[2]; r++ {
			for x := b[1]; x <= b[3]; x++ {
				g[r+n*dy(dir)][x+n*dx(dir)] = cells[i]
				i++
			}
		}
	}
	// passable reports whether a label in b can move over cells
	passable := func(cells [][2]int, b [4]int) bool {
		return !slices.ContainsFunc(cells, func(p [2]int) bool {
			if p[0] >= b[0] && p[0] <= b[2] && p[1] >= b[1] && p[1] <= b[3] {
				return false // the blanks of the label itself
			}
			c := g.at(p[0], p[1])
			return c == nil || !free(*c) && !c.glue || c.solid
		})
	}
	for _, id := range ids {
		b, edge := boxes[id], owners[id]
		if v, ok := inked[id]; ok {
			near := false
			for _, dir := range []uint8{up, down, left, right} {
				for n := 1; n <= 2 && !near; n++ {
					near, _ = lines(side(v, dir, n), edge)
				}
			}
			if !near {
				to, dist := uint8(0), 0
				for _, dir := range []uint8{left, right, up, down} {
					for n := 1; dist == 0 || n < dist; n++ {
						cells := side(v, dir, n)
						if own, other := lines(cells, edge); own && !other {
							to, dist = dir, n
							break
						}
						if !passable(cells, b) {
							break
						}
					}
				}
				if dist > 0 {
					shift(b, to, dist-2)
				}
				continue
			}
		}
		var ring [][2]int
		for _, dir := range []uint8{up, down, left, right} {
			ring = append(ring, side(b, dir, 1)...)
		}
		if _, other := lines(ring, edge); !other {
			continue
		}
		for _, dir := range []uint8{left, right, up, down} {
			if near, _ := lines(side(b, dir, 1), edge); near {
				continue
			}
			if own, _ := lines(side(b, dir, 2), edge); !own {
				continue
			}
			if !passable(side(b, dir, 1), b) {
				continue
			}
			shift(b, dir, 1)
			break
		}
	}
}
