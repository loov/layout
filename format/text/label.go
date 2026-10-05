package text

import (
	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
	"math"
	"slices"
	"strings"
	"unicode"
)

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

// drawLabels writes edge and cluster labels over everything else; paths
// are the cells of the edges
func (c *canvas) drawLabels(paths [][][2]int) {
	graph := c.l.Graph
	for i, edge := range graph.Edges {
		if edge.Label != "" && !edge.Invisible {
			label, x, y := c.edgeLabel(i)
			lines := strings.Split(label, "\n")
			x, y = c.besideEdge(x, y, draw.TextColumns(label), len(lines), paths[i], c.drawn[edge])
			c.pen = pen{font: rgb(edge.FontColor)}
			for k, line := range lines {
				c.text(x, y+k, line)
				for col := x; col < x+draw.TextColumns(line); col++ {
					if p := c.at(col, y+k); p != nil {
						p.text = c.drawn[edge]
					}
				}
			}
			// the blanks around the label keep it beside its edge when
			// carving, see seams
			w := draw.TextColumns(label)
			for row := y - 1; row <= y+len(lines); row++ {
				for col := x - 1; col <= x+w; col++ {
					if p := c.at(col, row); p != nil && p.r == ' ' && !p.solid {
						p.glue = true
					}
				}
			}
		}
	}
	for i, cluster := range graph.Clusters {
		if cluster.Label == "" || cluster.Invisible {
			continue
		}
		// the label goes on the frame after carving, see frameLabels, at
		// the first place along the top, else along the bottom, where the
		// lines that cross the frame fall on spaces of it; carving keeps
		// those cells
		b := c.clusterBox(i)
		text := " " + clusterLabel(cluster) + " "
		var runes []rune
		for _, r := range text {
			switch {
			case draw.IsZeroWidth(r), unicode.IsControl(r):
			case draw.IsWide(r):
				runes = append(runes, r, covered)
			default:
				runes = append(runes, r)
			}
		}
		fits := func(x, y int) bool {
			for j, r := range runes {
				p := c.at(x+j, y)
				if p == nil || x+j >= b[2] || !strings.ContainsRune("┈┉", p.r) && !(r == ' ' && p.lines&(up|down) != 0) {
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
						c.at(x+j, y).need = len(runes)
					}
					c.at(x, y).label = i + 1
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
// carved grid, see drawLabels
func (c *canvas) frameLabels(g grid) {
	for _, row := range g {
		for x := range row {
			id := row[x].label
			if id == 0 {
				continue
			}
			for _, r := range " " + clusterLabel(c.l.Graph.Clusters[id-1]) + " " {
				switch {
				case draw.IsZeroWidth(r), unicode.IsControl(r):
				case row[x].lines&(up|down) != 0:
					x++ // a line that crosses the frame, on a space, stays
				case draw.IsWide(r):
					row[x].r, row[x].fg, row[x+1].r = r, 0, covered
					x += 2
				default:
					row[x].r, row[x].fg = r, 0
					x++
				}
			}
		}
	}
}

// besideEdge places a label w cells wide and h tall at x, y against its
// edge, drawn along path with id: where nearEdge moves it, or against the
// edge from another side where the lines of other edges are further from
// it, so that it reads as its edge's
func (c *canvas) besideEdge(x, y, w, h int, path [][2]int, id int) (int, int) {
	bx, by, _ := c.nearEdge(x, y, w, h, path, [2]int{})
	best := c.apart(bx, by, w, h, id)
	for _, side := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		if sx, sy, ok := c.nearEdge(x, y, w, h, path, side); ok {
			if d := c.apart(sx, sy, w, h, id); d > best {
				bx, by, best = sx, sy, d
			}
		}
	}
	return bx, by
}

// apart returns how many blank cells at least separate a label w cells
// wide and h tall at x, y from the lines of edges other than id, up to 3
func (c *canvas) apart(x, y, w, h, id int) int {
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
// over onto its own edge's line, where carving leaves that a blank cell
// away from it: next to the other line, it should touch its own
func hug(g grid) {
	// the lines of edge id, and of other edges, in the cells
	lines := func(cells [][2]int, id int) (own, other bool) {
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
	// the box of every label
	boxes := map[int][4]int{} // top, left, bottom, right
	for r, row := range g {
		for x, c := range row {
			if c.text == 0 {
				continue
			}
			b, ok := boxes[c.text]
			if !ok {
				b = [4]int{r, x, r, x}
			}
			boxes[c.text] = [4]int{min(b[0], r), min(b[1], x), max(b[2], r), max(b[3], x)}
		}
	}
	// the cells of a side of b, n cells out
	side := func(b [4]int, dir, n int) [][2]int {
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
	for id, b := range boxes {
		var ring [][2]int
		for _, dir := range []int{up, down, left, right} {
			ring = append(ring, side(b, dir, 1)...)
		}
		if _, other := lines(ring, id); !other {
			continue
		}
		for _, dir := range []int{left, right, up, down} {
			if near, _ := lines(side(b, dir, 1), id); near {
				continue
			}
			if own, _ := lines(side(b, dir, 2), id); !own {
				continue
			}
			if slices.ContainsFunc(side(b, dir, 1), func(p [2]int) bool { c := g.at(p[0], p[1]); return c == nil || !free(*c) && !c.glue || c.solid }) {
				continue
			}
			// move the label, from the far end on
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
					g[r+dy(dir)][x+dx(dir)] = cells[i]
					i++
				}
			}
			break
		}
	}
}
