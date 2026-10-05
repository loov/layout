package text

import (
	"math/bits"
	"slices"
	"strings"
)

// carve tightens the drawing along seams: it removes a cell from every
// column, and then from every row, as long as some seam can go through
// cells that only continue the straight lines across it or blanks, and
// repeat the cells before them. At most one such row or column stays of
// every run.
// Seams across the ranks go straight, rows when ranks are rows and
// columns when they are sideways, so that the nodes of a rank stay in
// line. Seams along the ranks turn, but not between cells that are
// joined, so that what is drawn stays connected. Nodes and text stay
// whole.
func carve(g grid, sideways bool) grid {
	// seams keep the blanks past the end of lines, trailing rows of them
	// included; keep one of those as the bottom margin
	blank := func(row []cell) bool {
		return !slices.ContainsFunc(row, func(c cell) bool { return c.r != ' ' || c.bg != 0 })
	}
	rows := func(g grid, stubs bool) grid {
		g = seams(g.transpose(), " │┃┊┋┆", "▲▼●○", 1, sideways, false, stubs, right, left).transpose()
		for len(g) > 1 && blank(g[len(g)-1]) && blank(g[len(g)-2]) {
			g = g[:len(g)-1]
		}
		return g
	}
	// the stubs of lines leaving a node go last: lines can still run
	// further along them while the columns are carved
	g = rows(g, false)
	g = seams(g, " ─━┈┉┄", "◀▶●○", 1, !sideways, true, false, down, up)
	// straightening a line can clear the way for another
	for !sideways && unjog(g) {
	}
	// rows merge first, as taking a step out can block that, and again
	// after, as it can clear the way
	for merged := !sideways; merged; {
		g, merged = mergeRows(g)
	}
	for !sideways && straighten(g) {
	}
	for merged := !sideways; merged; {
		g, merged = mergeRows(g)
	}
	g = rows(g, true)
	hug(g)
	return g
}

// seams removes seams from the lines of grid, one cell from every line,
// while there is one that removes more than the blanks past the end of
// lines; see carve. Seams turn between lines when turn is set. Cells of
// neighboring lines are joined when the cell of the line before has the
// arm next, or the cell of the line after has the arm prev, or one is
// kept or glued and the other is too or is not blank.
//
// With stubs, the first cell of a straight line that goes on past it may
// go too.
//
// With bend set, once no seam is left, a seam that turns can also cross
// lines joining two lines, as long as it narrows the grid: a line that
// turns there runs one cell further along, and a straight line jogs over
// in a blank cell beside it, see bendsAt. Seams prefer the first, which
// adds no corners.
func seams(g grid, straight, markers string, keep int, turn, bend, stubs bool, next, prev int) grid {
	// the cost of a seam through a cell, and of each cell it moves along
	const (
		trailing = 1 << 20 // past the end of the line, which removes nothing
		blocked  = 1 << 40
		jog      = 1 << 16 // crossing a straight line, which bends it
		extend   = 4       // crossing a line where it turns
	)
	// the directions along the lines, toward the start and the end; the
	// lines run across the ranks when they are columns, see carve
	lo, hi := left, right
	across := next&(left|right) != 0
	if across {
		lo, hi = up, down
	}
	// crossings that failed to jog, by line and cell, until a seam is cut
	forbid := map[[2]int]bool{}
	jogging := false // only seams that cross lines are left
	for {
		for i := range g {
			g[i] = append(g[i], cell{r: ' '})
		}
		n := len(g)
		if n == 0 {
			return g
		}
		m := len(g[0])
		// the cells past the end of each line are blank
		ends := make([]int, n)
		for i, line := range g {
			for x, c := range line {
				if c.r != ' ' || c.bg != 0 {
					ends[i] = x + 1
				}
			}
		}
		width := slices.Max(ends)
		cost := func(i, x int) int {
			line := g[i]
			c := line[x]
			if c.keep || !strings.ContainsRune(straight, c.r) {
				return blocked
			}
			if c.need > 0 {
				// a run kept for a label stays long enough for it
				run := 1
				for k := x - 1; k >= 0 && line[k].need == c.need; k-- {
					run++
				}
				for k := x + 1; k < len(line) && line[k].need == c.need; k++ {
					run++
				}
				if run <= c.need {
					return blocked
				}
			}
			if x < ends[i] {
				// with stubs, last, blanks keep one of a run before what
				// follows, and none around the text of labels
				if stubs && c.r == ' ' && c.bg == 0 && x > 0 && x+1 < len(line) {
					text := func(p cell) bool { return p.keep && !p.solid }
					if after := line[x+1]; after.r == ' ' && after.bg == 0 || text(after) || text(line[x-1]) {
						return 0
					}
				}
				// a line that turns right after leaving what it joins
				// needs no cell between, unless it leaves a node for an
				// arrowhead; and a node needs no blank under it where what
				// follows is blank or drawn by an edge, see leaves
				if across && x > 0 && x+1 < len(line) {
					before, after := line[x-1], line[x+1]
					arrow := strings.ContainsRune(markers, after.r)
					if c.r == glyph(lo|hi, 0) && arms(before.r)&hi != 0 && (arms(after.r)&lo != 0 || arrow) && !(arrow && before.solid) &&
						// or, with stubs, that goes on straight, which keeps a
						// cell of it
						(arrow || arms(after.r) != lo|hi || stubs && after.r == c.r && after.bg == c.bg) {
						return 0
					}
					if c.r == ' ' && before.solid && !after.solid && !arrow && leaves(g, i, x) {
						return 0
					}
				}
				for k := 1; k <= keep; k++ {
					// a marker on a run continues it like the line it sits on
					if b := line[max(x-k, 0)]; x-k < 0 || !(c.r == b.r && c.bg == b.bg || c.r != ' ' && strings.ContainsRune(markers, b.r)) {
						return blocked
					}
				}
				return 0
			}
			if jogging && ends[i] == width {
				// a seam with jogs has to narrow the grid
				return blocked
			}
			return trailing
		}
		// crossing is the cost of a seam crossing the line at x between
		// lines i-1 and i, moving toward hi when toHi is set
		crossing := func(i, x int, toHi bool) int {
			if !jogging || forbid[[2]int{i, x}] {
				return blocked
			}
			best := blocked
			for _, b := range bendsAt(g, i, x, toHi, next, prev, lo, hi) {
				if b.extends {
					best = min(best, extend)
				} else {
					best = min(best, jog)
				}
			}
			return best
		}
		// best[i][x] is the cost of the cheapest seam through the lines up
		// to i that ends at x, coming from from[i][x] on the line before
		best, from := make([][]int, n), make([][]int, n)
		for i := range n {
			best[i], from[i] = make([]int, m), make([]int, m)
			for x := range m {
				best[i][x] = cost(i, x)
			}
			if i == 0 {
				continue
			}
			joined := make([]bool, m)
			for x := range m {
				a, b := g[i-1][x], g[i][x]
				sticky := func(c cell) bool { return c.keep || c.glue }
				joined[x] = arms(a.r)&next != 0 || arms(b.r)&prev != 0 ||
					sticky(a) && (sticky(b) || b.r != ' ') || sticky(b) && a.r != ' '
			}
			if !turn {
				for x := range m {
					best[i][x] += best[i-1][x]
					from[i][x] = x
				}
				continue
			}
			// the cheapest way to x from the line before, moving one cell
			// at a time and past a joined cell only where it can jog
			reach, src := make([]int, m), make([]int, m)
			for x := range m {
				reach[x], src[x] = best[i-1][x], x
				if x > 0 {
					if r := best[i-1][x-1] + 1; r < reach[x] {
						reach[x], src[x] = r, x-1
					}
					r := reach[x-1] + 1
					if joined[x-1] {
						r += crossing(i, x-1, true)
					}
					if r < reach[x] {
						reach[x], src[x] = r, src[x-1]
					}
				}
			}
			for x := m - 2; x >= 0; x-- {
				if r := best[i-1][x+1] + 1; r < reach[x] {
					reach[x], src[x] = r, x+1
				}
				r := reach[x+1] + 1
				if joined[x+1] {
					r += crossing(i, x+1, false)
				}
				if r < reach[x] {
					reach[x], src[x] = r, src[x+1]
				}
			}
			for x := range m {
				best[i][x] += reach[x]
				from[i][x] = src[x]
			}
		}
		end := 0
		for x := range m {
			if best[n-1][x] < best[n-1][end] {
				end = x
			}
		}
		if best[n-1][end] >= n*trailing {
			for i := range g {
				g[i] = g[i][:m-1]
			}
			if bend && turn && !jogging {
				jogging = true
				continue
			}
			return g
		}
		path := make([]int, n)
		for i, x := n-1, end; i >= 0; i-- {
			path[i] = x
			x = from[i][x]
		}
		cut := make(grid, n)
		for i, x := range path {
			cut[i] = slices.Delete(slices.Clone(g[i]), x, x+1)
		}
		jogs, failed, ok := bendLines(g, cut, path, next, prev, lo, hi)
		if ok && jogs > 1 {
			// the cheapest seam left has the fewest jogs; one column is
			// not worth more than one
			for i := range g {
				g[i] = g[i][:m-1]
			}
			return g
		}
		if !ok {
			forbid[failed] = true
			for i := range g {
				g[i] = g[i][:m-1]
			}
			continue
		}
		g = cut
		clear(forbid)
		jogging = false
	}
}

// leaves reports whether the cell after x on line i, below a node in the
// cell before x, is blank or drawn by an edge, which can run right along
// the node; the frame of a cluster keeps a cell off it
func leaves(g grid, i, x int) bool {
	after := g[i][x+1]
	if after.r == ' ' {
		return after.bg == 0 && !after.keep && !after.glue
	}
	return after.lines != 0 && !after.frame // not text or a mark
}

// bend is a way for a line crossed by a seam to get from line i-1 to
// line i, where the seam moved it one cell over on one of them: on line
// row, the cell at at that the line runs through gets the arms lines, and
// the blank cell at to beside it the arms join. Cells are where they are
// once the seam is cut.
type bend struct {
	row, at, to int
	lines, join int
	extends     bool // the line turned at at, and now runs one cell further
}

// bendsAt returns the ways that the line at x between lines i-1 and i of
// grid can bend when a seam crosses it toward hi, when toHi is set, or
// toward lo. Either the cell of the line before or the one of the line
// after takes the turn, toward a blank cell beside it.
func bendsAt(g grid, i, x int, toHi bool, next, prev, lo, hi int) []bend {
	a, b := g[i-1], g[i]
	plain := func(c cell) bool { return !c.keep && !c.glue && c.r != ' ' && corner(arms(c.r)) == c.r }
	if x == 0 || x+1 >= len(a) || !plain(a[x]) || !plain(b[x]) || arms(a[x].r)&next == 0 || arms(b[x].r)&prev == 0 {
		return nil
	}
	// the line ends up at pa on line i-1 and at pb on line i, d from pa
	pa, pb, d, step := x, x-1, lo, -1
	if toHi {
		pa, pb, d, step = x-1, x, hi, 1
	}
	// a jog keeps a straight line on both of its ends, off nodes,
	// arrowheads and other turns
	straight := func(i int) bool {
		if i < 0 || i >= len(g) {
			return false
		}
		c := g[i][x]
		return !c.keep && !c.solid && c.r == glyph(next|prev, 0)
	}
	var out []bend
	add := func(row, at, to, lines, join int, near, other int) {
		extends := lines == lo|hi
		if bits.OnesCount(uint(lines)) == 2 && (extends || straight(near) && straight(other)) {
			out = append(out, bend{row, at, to, lines, join, extends})
		}
	}
	if free(a[x+step]) {
		add(i-1, pa, pb, arms(a[x].r)&^next|d, opposite(d)|next, i-2, i)
	}
	if free(b[x-step]) {
		add(i, pb, pa, arms(b[x].r)&^prev|opposite(d), d|prev, i+1, i-1)
	}
	return out
}

// bendLines bends the lines that the seam through path crosses between
// two lines of grid, in cut, which is grid with the seam removed; bends
// that run lines further go first. It returns the number of jogs, or the
// line and cell of a crossing that has no room to bend.
func bendLines(g, cut grid, path []int, next, prev, lo, hi int) (jogs int, failed [2]int, ok bool) {
	// has reports whether the cell at x of line i has the arm, with lines
	// past the grid having none
	has := func(i, x, arm int) bool {
		return i >= 0 && i < len(cut) && x >= 0 && x < len(cut[i]) && arms(cut[i][x].r)&arm != 0
	}
	for i := 1; i < len(path); i++ {
		xa, xb := path[i-1], path[i]
		for x := min(xa, xb) + 1; x < max(xa, xb); x++ {
			a, b := g[i-1][x], g[i][x]
			if arms(a.r)&next == 0 && arms(b.r)&prev == 0 {
				continue
			}
			bends := bendsAt(g, i, x, xb > xa, next, prev, lo, hi)
			slices.SortStableFunc(bends, func(p, q bend) int {
				if p.extends == q.extends {
					return 0
				} else if p.extends {
					return -1
				}
				return 1
			})
			done := false
			for _, bd := range bends {
				// the lines before and after keep joining the bend
				out, back := next, prev
				if bd.row == i {
					out, back = prev, next
				}
				near := bd.row - 1
				if bd.row == i {
					near = bd.row + 1
				}
				if bd.lines&back != 0 && !has(near, bd.at, out) || has(near, bd.to, out) {
					continue
				}
				// the line before or after may have moved with a crossing
				// of its own, see bendsAt
				other := i
				if bd.row == i {
					other = i - 1
				}
				line := func(c cell) bool { return !c.keep && !c.solid && c.r == glyph(next|prev, 0) }
				if !bd.extends && (near < 0 || near >= len(cut) || !line(cut[near][bd.at]) || !line(cut[other][bd.to])) {
					continue
				}
				// a line along a box stays of the box
				like := cut[bd.row][bd.at]
				id := ownerOf(like)
				owner := [4]int{id, id, id, id}
				cut[bd.row][bd.at] = cell{r: corner(bd.lines), fg: like.fg, bg: like.bg, lines: bd.lines, solid: like.solid, node: like.node, owner: owner}
				cut[bd.row][bd.to] = cell{r: corner(bd.join), fg: like.fg, bg: like.bg, lines: bd.join, solid: like.solid, node: like.node, owner: owner}
				if !bd.extends {
					jogs++
				}
				done = true
				break
			}
			if !done {
				return 0, [2]int{i, x}, false
			}
		}
	}
	return jogs, [2]int{}, true
}

// ownerOf returns the edge that drew a line of c, 0 for none
func ownerOf(c cell) int {
	for arm := range 4 {
		if c.lines&(1<<arm) != 0 && c.owner[arm] != 0 {
			return c.owner[arm]
		}
	}
	return 0
}

// free reports whether c is an empty cell that a line can be drawn in
func free(c cell) bool {
	return c.r == ' ' && c.bg == 0 && !c.keep && !c.glue && c.need == 0 && c.label == 0
}
