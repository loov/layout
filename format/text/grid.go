package text

import (
	"cmp"
	"math/bits"
	"slices"
	"strings"
)

// cell is a drawn character with its colors, see rgb, and what is
// needed to join the lines drawn through it
type cell struct {
	r      rune
	fg, bg uint32
	keep   bool   // inside a node or of text, which carving keeps
	glue   bool   // beside a label, which carving keeps beside it
	need   int    // the length a run of these must keep, for a label on it
	label  int    // the cluster whose label starts here, from 1
	solid  bool   // covered by a node; edges do not draw there
	lines  int    // direction mask, for joining edge runs
	heavy  int    // arms that runs of different edges share
	owner  [4]int // edge that first drew each arm
	frame  bool   // of the frame of a cluster
	node   int    // node whose box covers it, from 1, see canvas.nodes
}

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
func carve(grid [][]cell, sideways bool) [][]cell {
	// seams keep the blanks past the end of lines, trailing rows of them
	// included; keep one of those as the bottom margin
	blank := func(row []cell) bool {
		return !slices.ContainsFunc(row, func(c cell) bool { return c.r != ' ' || c.bg != 0 })
	}
	rows := func(grid [][]cell, stubs bool) [][]cell {
		grid = transpose(seams(transpose(grid), " │┃┊┋┆", "▲▼●○", 1, sideways, false, stubs, right, left))
		for len(grid) > 1 && blank(grid[len(grid)-1]) && blank(grid[len(grid)-2]) {
			grid = grid[:len(grid)-1]
		}
		return grid
	}
	// the stubs of lines leaving a node go last: lines can still run
	// further along them while the columns are carved
	grid = rows(grid, false)
	grid = seams(grid, " ─━┈┉┄", "◀▶●○", 1, !sideways, true, false, down, up)
	// straightening a line can clear the way for another
	for !sideways && unjog(grid) {
	}
	// rows merge first, as taking a step out can block that, and again
	// after, as it can clear the way
	for merged := !sideways; merged; {
		grid, merged = mergeRows(grid)
	}
	for !sideways && straighten(grid) {
	}
	for merged := !sideways; merged; {
		grid, merged = mergeRows(grid)
	}
	return rows(grid, true)
}

// mergeRows merges the first two neighboring rows with runs along them
// into one, where the lines of the lower one move up without meeting
// those of the upper: no cell gets runs along from both, and a line that
// leaves the upper row up and the lower row down runs on through both.
// Nodes, markers, text and the frames of clusters stay where they are. It
// returns whether it merged two rows.
func mergeRows(grid [][]cell) ([][]cell, bool) {
	along := func(row []cell) bool {
		return slices.ContainsFunc(row, func(c cell) bool { return c.lines&(left|right) != 0 })
	}
	for r := 0; r+1 < len(grid); r++ {
		if !along(grid[r]) || !along(grid[r+1]) {
			continue
		}
		if row, ok := mergeRow(grid[r], grid[r+1]); ok {
			grid[r] = row
			return slices.Delete(grid, r+1, r+2), true
		}
		// a line that turns along one of the rows can slide over to
		// clear the way, a few cells at most
		for _, end := range []struct{ r, dir int }{{r, up}, {r + 1, down}} {
			for x := range grid[end.r] {
				for _, dx := range []int{1, -1, 2, -2, 3, -3} {
					moves, ok := slide(grid, end.r, x, end.dir, dx)
					if !ok {
						continue
					}
					a, b := slices.Clone(grid[r]), slices.Clone(grid[r+1])
					for _, m := range moves {
						switch m.r {
						case r:
							a[m.x] = m.c
						case r + 1:
							b[m.x] = m.c
						}
					}
					if row, ok := mergeRow(a, b); ok {
						for _, m := range moves {
							grid[m.r][m.x] = m.c
						}
						grid[r] = row
						return slices.Delete(grid, r+1, r+2), true
					}
				}
			}
		}
	}
	return grid, false
}

// straighten takes the first step of a line aside out, sliding the line
// before or after it over, see slide. It returns whether it took one out.
func straighten(grid [][]cell) bool {
	for r, row := range grid {
		for x, c := range row {
			// the turn at the left end of a step
			if c.lines&right == 0 || bits.OnesCount(uint(c.lines)) != 2 || c.lines&(up|down) == 0 {
				continue
			}
			end := x + 1
			for end < len(row) && row[end].lines == left|right {
				end++
			}
			if end == len(row) || row[end].lines != left|opposite(c.lines&(up|down)) {
				continue
			}
			for _, from := range []struct{ x, dx int }{{x, end - x}, {end, x - end}} {
				if moves, ok := slide(grid, r, from.x, grid[r][from.x].lines&(up|down), from.dx); ok {
					for _, m := range moves {
						grid[m.r][m.x] = m.c
					}
					return true
				}
			}
		}
	}
	return false
}

// move is a cell to put at r, x
type move struct {
	r, x int
	c    cell
}

// slide returns the cells that move the line leaving the turn at r, x
// toward dir dx cells over, along with the runs it turns from at both
// ends, or into an arrowhead or a port on a box at the far end, which
// moves along the box. The line moves only through blanks, of one edge, and its runs
// only grow over blanks and shrink over themselves.
func slide(grid [][]cell, r, x, dir, dx int) ([]move, bool) {
	at := func(r, x int) *cell {
		if r < 0 || r >= len(grid) || x < 0 || x >= len(grid[r]) {
			return nil
		}
		return &grid[r][x]
	}
	blank := func(r, x int) bool { c := at(r, x); return c != nil && free(*c) }
	solid := func(r, x int) bool { c := at(r, x); return c != nil && c.solid }
	turn := at(r, x)
	if turn == nil || turn.lines&dir == 0 || bits.OnesCount(uint(turn.lines)) != 2 || turn.lines&(left|right) == 0 {
		return nil, false
	}
	id := turn.owner[bits.TrailingZeros(uint(dir))]
	// a dashed line, whose corners are square, stays dashed, see canvas.line
	dashed := turn.r == glyph(turn.lines, 0)
	draw := func(lines int) rune {
		switch {
		case !dashed:
			return corner(lines)
		case lines == up|down:
			return '┊'
		case lines == left|right:
			return '┈'
		}
		return glyph(lines, 0)
	}
	step := map[int]int{up: -1, down: 1}[dir]
	var moves []move
	// turns moves the turn at row y from x along its run, which grows or
	// shrinks to meet it at x+dx
	turns := func(y int, arms int) bool {
		run := arms & (left | right)
		side := map[int]int{left: -1, right: 1}[run]
		lo, hi := min(x, x+dx), max(x, x+dx)
		for c := lo; c <= hi; c++ {
			if c == x {
				continue
			}
			p := at(y, c)
			// toward the run it shrinks over its own run, onto the turn at
			// its far end where the line goes on straight; away from it,
			// it grows over blanks
			end := c == x+dx && p != nil && p.lines == opposite(run)|opposite(arms&(up|down)) && p.owner[bits.TrailingZeros(uint(opposite(run)))] == id
			if dx*side > 0 && !end && (p == nil || p.lines != left|right || p.owner[2] != id) || dx*side < 0 && !blank(y, c) {
				return false
			}
			if end {
				arms = up | down
			}
		}
		line := cell{r: draw(left | right), fg: turn.fg, lines: left | right, owner: [4]int{id, id, id, id}}
		for c := lo; c <= hi; c++ {
			switch {
			case c == x+dx:
				moves = append(moves, move{y, c, cell{r: draw(arms), fg: turn.fg, lines: arms, owner: [4]int{id, id, id, id}}})
			case dx*side > 0:
				moves = append(moves, move{y, c, cell{r: ' '}})
			default:
				moves = append(moves, move{y, c, line})
			}
		}
		return true
	}
	if !turns(r, turn.lines) {
		return nil, false
	}
	vertical := cell{r: draw(up | down), fg: turn.fg, lines: up | down, owner: [4]int{id, id, id, id}}
	for y := r + step; ; y += step {
		p := at(y, x)
		switch {
		case p == nil:
			return nil, false
		case p.lines&(up|down) == up|down && p.owner[0] == id && p.owner[1] == id && !p.solid:
			// a line it crosses it crosses where it goes too, and leaves
			// whole where it was; a cell off nodes, as unjog keeps
			q := at(y, x+dx)
			across := p.lines != up|down && q != nil && !q.solid && !q.keep && !q.glue && q.lines == left|right && q.owner[2] != id
			if (p.lines != up|down && p.lines != up|down|left|right) || !blank(y, x+dx) && !across || solid(y, x+dx-1) || solid(y, x+dx+1) {
				return nil, false
			}
			was := cell{r: ' '}
			if p.lines != up|down {
				was = *p
				was.r, was.lines, was.owner[0], was.owner[1] = '─', left|right, 0, 0
			}
			to := vertical
			if across {
				to = *q
				to.r, to.lines, to.owner[0], to.owner[1] = '╂', up|down|left|right, id, id
			}
			moves = append(moves, move{y, x, was}, move{y, x + dx, to})
			continue
		case p.solid && p.node == 0 && dir == down:
			// an arrowhead onto the straight top of a box
			below, to := at(y+1, x), at(y+1, x+dx)
			if !blank(y, x+dx) || below == nil || to == nil || below.node == 0 || to.node != below.node || to.r != below.r {
				return nil, false
			}
			moves = append(moves, move{y, x, cell{r: ' '}}, move{y, x + dx, *p})
			return moves, true
		case p.lines&opposite(dir) != 0 && bits.OnesCount(uint(p.lines)) == 2 && p.lines&(left|right) != 0 && p.owner[bits.TrailingZeros(uint(opposite(dir)))] == id:
			return moves, turns(y, p.lines)
		case p.solid && p.node != 0 && arms(p.r)&opposite(dir) != 0:
			// a port on the side of a box moves along its straight side
			to := at(y, x+dx)
			if to == nil || to.node != p.node || arms(to.r) != left|right {
				return nil, false
			}
			port, side := *p, *to
			port.r, side.r = p.r, to.r
			moves = append(moves, move{y, x, side}, move{y, x + dx, port})
			return moves, true
		default:
			return nil, false
		}
	}
}

// mergeRow returns the cells of a and b in one row, see mergeRows
func mergeRow(a, b []cell) ([]cell, bool) {
	out := make([]cell, max(len(a), len(b)))
	for x := range out {
		p, q := cell{r: ' '}, cell{r: ' '}
		if x < len(a) {
			p = a[x]
		}
		if x < len(b) {
			q = b[x]
		}
		fixed := func(c cell) bool {
			return c.solid || c.keep || c.glue || c.need > 0 || c.label > 0 || c.frame || c.r != ' ' && c.lines == 0
		}
		link := p.lines&down != 0 && q.lines&up != 0
		switch {
		case fixed(p) || fixed(q) || p.bg != q.bg:
			if p.r != ' ' || q.r != ' ' || p.bg != q.bg {
				return nil, false
			}
		case p.lines&down != 0 != (q.lines&up != 0):
			return nil, false // a line that ends between them
		case p.lines&up != 0 && q.lines&down != 0 && !link:
			return nil, false // two lines would join
		case p.lines&(left|right) != 0 && q.lines&(left|right) != 0:
			return nil, false
		}
		lines := p.lines&^down | q.lines&^up
		c := cell{r: ' ', fg: p.fg, bg: p.bg, lines: lines}
		for arm := range 4 {
			switch bit := 1 << arm; {
			case lines&bit == 0:
			case bit == down:
				c.owner[arm] = q.owner[arm]
			case bit == up || p.lines&bit != 0:
				c.owner[arm] = p.owner[arm]
			default:
				c.owner[arm], c.fg = q.owner[arm], q.fg
			}
		}
		switch {
		case lines == 0:
		case lines == p.lines:
			c.r, c.fg = p.r, p.fg
		case lines == q.lines:
			c.r, c.fg = q.r, q.fg
		case lines == up|down|left|right && c.owner[0] != c.owner[2]:
			c.r = '╂' // a line across another
		default:
			c.r = corner(lines)
		}
		out[x] = c
	}
	return out, true
}

// unjog straightens lines down the grid: two steps aside in a row become
// one, on the row of the first or of the second, and a last step right
// before an arrowhead takes the arrowhead along, onto the same box; a
// turn of the layout and a jog of carving can add up so. A line leaving a
// junction along its row, where lines merge, is a first step too, which
// becomes one on the row of the junction, as the junction stays. A step also
// slides along its line to where the line crosses fewer lines. Steps are lines
// of their own, which nothing joins. The new line crosses no more lines
// than the old, keeps a cell off nodes, and lines with a label beside
// them stay.
func unjog(grid [][]cell) (changed bool) {
	type pos struct{ r, c int }
	at := func(p pos) *cell {
		if p.r < 0 || p.r >= len(grid) || p.c < 0 || p.c >= len(grid[p.r]) {
			return nil
		}
		return &grid[p.r][p.c]
	}
	is := func(p pos, set string) bool { q := at(p); return q != nil && strings.ContainsRune(set, q.r) }
	// run returns where a line leaving p along the row in dir runs to, and
	// whether it goes on down there; when it does not, it runs into
	// something there, such as an arrowhead
	run := func(p pos, dir int) (to int, turns bool) {
		c := p.c + dir
		for is(pos{p.r, c}, "─╂") {
			c += dir
		}
		return c, is(pos{p.r, c}, map[int]string{-1: "╭", 1: "╮"}[dir])
	}
	// step returns the direction of the step of a line that comes down to
	// p, where it runs along to and whether it goes on down there
	step := func(p pos) (dir, to int, turns bool) {
		switch {
		case is(p, "╰"):
			dir = 1
		case is(p, "╯"):
			dir = -1
		default:
			return 0, 0, false
		}
		to, turns = run(p, dir)
		return dir, to, turns
	}
	// bottom returns the row where a line going down from p turns or ends
	bottom := func(p pos) int {
		r := p.r + 1
		for is(pos{r, p.c}, "│╂") {
			r++
		}
		return r
	}
	// cells returns the cells along the points, with the arms of each in
	// the line: in into the first, out out of the last
	cells := func(in, out int, points ...pos) ([]pos, []int) {
		var path []pos
		for i, p := range points {
			if i == 0 {
				path = append(path, p)
				continue
			}
			q := path[len(path)-1]
			dr, dc := cmp.Compare(p.r, q.r), cmp.Compare(p.c, q.c)
			for q != p {
				q.r, q.c = q.r+dr, q.c+dc
				path = append(path, q)
			}
		}
		arms := make([]int, len(path))
		toward := func(from, to pos) int {
			switch {
			case to.r < from.r:
				return up
			case to.r > from.r:
				return down
			case to.c < from.c:
				return left
			}
			return right
		}
		for i, p := range path {
			if i == 0 {
				arms[i] |= in
			} else {
				arms[i] |= toward(p, path[i-1])
			}
			if i == len(path)-1 {
				arms[i] |= out
			} else {
				arms[i] |= toward(p, path[i+1])
			}
		}
		return path, arms
	}
	// reroute moves a line from the old cells to the new ones, both with
	// the arm in into the first, ending in head when it is set, if it can
	// fewer has reroute take only a new line that crosses fewer lines
	fewer := false
	reroute := func(in int, old, path []pos, arms []int, head *cell) bool {
		olds := map[pos]int{}
		_, oldArms := cells(in, down, old...)
		for i, p := range old {
			olds[p] = oldArms[i]
		}
		news := map[pos]bool{}
		for _, p := range path {
			news[p] = true
		}
		added, removed := 0, 0
		for p := range olds {
			if news[p] {
				continue
			}
			for _, d := range []pos{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				// a label beside the line keeps it
				if q := at(pos{p.r + d.r, p.c + d.c}); q != nil && !q.solid && (q.glue || q.keep) {
					return false
				}
			}
			if at(p).r == '╂' {
				removed++
			}
		}
		for i, p := range path {
			q := at(p)
			if q == nil {
				return false
			}
			if _, ok := olds[p]; ok {
				// a crossing on the old line stays one on the new
				if q.r == '╂' && arms[i] != up|down && arms[i] != left|right {
					return false
				}
				continue
			}
			straight := arms[i] == up|down || arms[i] == left|right
			across := straight && (arms[i] == up|down && q.r == '─' || arms[i] == left|right && q.r == '│')
			blank := q.r == ' ' && q.bg == 0 && !q.keep && !q.glue && q.need == 0 && q.label == 0
			if q.solid || !blank && !across {
				return false
			}
			if across {
				added++
			}
			for _, dc := range []int{-1, 1} {
				// a cell off nodes, other than an arrowhead that moves
				n := pos{p.r, p.c + dc}
				if q := at(n); q != nil && q.solid && olds[n] == 0 {
					return false
				}
			}
		}
		if added > removed || fewer && added == removed {
			return false
		}
		// the edge of the line, by the arms it draws, not of lines it crosses
		fg, id := at(old[0]).fg, 0
		for p, a := range olds {
			for arm := range 4 {
				if a&(1<<arm) != 0 && at(p).lines&(1<<arm) != 0 && at(p).owner[arm] != 0 {
					id = at(p).owner[arm]
				}
			}
		}
		for p, a := range olds {
			if news[p] {
				continue
			}
			if q := at(p); q.r == '╂' {
				// the line it crossed goes on
				q.lines = (up | down | left | right) &^ a
				q.r = corner(q.lines)
			} else {
				*q = cell{r: ' '}
			}
		}
		for i, p := range path {
			q := at(p)
			switch {
			case i == len(path)-1 && head != nil:
				*q = *head
			case q.r == '╂' || q.r != ' ' && olds[p] == 0:
				q.r, q.lines = '╂', up|down|left|right
				for arm := range 4 {
					if arms[i]&(1<<arm) != 0 {
						q.owner[arm] = id
					}
				}
			default:
				*q = cell{r: corner(arms[i]), fg: fg, lines: arms[i], owner: [4]int{id, id, id, id}}
			}
		}
		return true
	}
	for r := range grid {
		for c := range grid[r] {
			top := pos{r, c}
			if q := at(top); is(top, "├┤┬┴┼") && !q.solid {
				// a line leaving a junction along the row and a step after
				// it: one, on the row of the junction, which stays
				for _, dir := range []int{-1, 1} {
					side, back := right, left
					if dir < 0 {
						side, back = left, right
					}
					first := pos{r, c + dir}
					b, ok := run(top, dir)
					if arms(q.r)&side == 0 || !ok || at(first).solid {
						continue
					}
					mid := pos{bottom(pos{r, b}), b}
					_, c2, turns := step(mid)
					if !turns || (c2-c)*dir <= 0 {
						continue
					}
					end := pos{mid.r, c2}
					old, _ := cells(back, down, first, pos{r, b}, mid, end)
					path, arms := cells(back, down, first, pos{r, c2}, end)
					if reroute(back, old, path, arms, nil) {
						changed = true
					}
				}
				continue
			}
			_, b, ok := step(top)
			if !ok {
				continue
			}
			mid := pos{bottom(pos{r, b}), b}
			if dir, c2, turns := step(mid); dir != 0 {
				// two steps: one, on the lower row or else the upper;
				// a second that runs into something ends before it
				end, out := pos{mid.r, c2}, down
				if !turns {
					end, out = pos{mid.r, c2 - dir}, map[int]int{-1: left, 1: right}[dir]
				}
				old, _ := cells(up, out, top, pos{r, b}, mid, end)
				vias := []pos{{mid.r, c}, {r, c2}}
				if !turns {
					vias = vias[:1] // the run goes on along the lower row
				}
				for _, via := range vias {
					path, arms := cells(up, out, top, via, end)
					if reroute(up, old, path, arms, nil) {
						changed = true
						break
					}
				}
				continue
			}
			if !is(mid, "▼") {
				continue
			}
			// a last step before an arrowhead into a box: the arrowhead
			// goes along when the box goes on under the line, with no
			// corner between
			box := true
			for x := min(c, b); x <= max(c, b); x++ {
				p := at(pos{mid.r + 1, x})
				box = box && p != nil && p.solid && (x == b || strings.ContainsRune("─━═┬┴╤╥", p.r))
			}
			if !box {
				continue
			}
			head := *at(mid)
			old, _ := cells(up, down, top, pos{r, b}, mid)
			path, arms := cells(up, down, top, pos{mid.r, c})
			if reroute(up, old, path, arms, &head) {
				changed = true
			}
		}
	}
	// a step slides along its line to where the line crosses fewer
	// lines, as two lines can cross twice to swap back
	fewer = true
	for r := range grid {
		for c := range grid[r] {
			top := pos{r, c}
			_, b, ok := step(top)
			if !ok {
				continue
			}
			first := r
			for is(pos{first - 1, c}, "│╂") {
				first--
			}
			last := bottom(pos{r, b}) - 1
			old, _ := cells(up, down, pos{first, c}, top, pos{r, b}, pos{last, b})
			for row := first; row <= last; row++ {
				if row == r {
					continue
				}
				path, arms := cells(up, down, pos{first, c}, pos{row, c}, pos{row, b}, pos{last, b})
				if reroute(up, old, path, arms, nil) {
					changed = true
					break
				}
			}
		}
	}
	return changed
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
func seams(grid [][]cell, straight, markers string, keep int, turn, bend, stubs bool, next, prev int) [][]cell {
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
		for i := range grid {
			grid[i] = append(grid[i], cell{r: ' '})
		}
		n := len(grid)
		if n == 0 {
			return grid
		}
		m := len(grid[0])
		// the cells past the end of each line are blank
		ends := make([]int, n)
		for i, line := range grid {
			for x, c := range line {
				if c.r != ' ' || c.bg != 0 {
					ends[i] = x + 1
				}
			}
		}
		width := slices.Max(ends)
		cost := func(i, x int) int {
			line := grid[i]
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
					if c.r == ' ' && before.solid && !after.solid && !arrow && leaves(grid, i, x) {
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
			for _, b := range bendsAt(grid, i, x, toHi, next, prev, lo, hi) {
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
				a, b := grid[i-1][x], grid[i][x]
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
			for i := range grid {
				grid[i] = grid[i][:m-1]
			}
			if bend && turn && !jogging {
				jogging = true
				continue
			}
			return grid
		}
		path := make([]int, n)
		for i, x := n-1, end; i >= 0; i-- {
			path[i] = x
			x = from[i][x]
		}
		cut := make([][]cell, n)
		for i, x := range path {
			cut[i] = slices.Delete(slices.Clone(grid[i]), x, x+1)
		}
		jogs, failed, ok := bendLines(grid, cut, path, next, prev, lo, hi)
		if ok && jogs > 1 {
			// the cheapest seam left has the fewest jogs; one column is
			// not worth more than one
			for i := range grid {
				grid[i] = grid[i][:m-1]
			}
			return grid
		}
		if !ok {
			forbid[failed] = true
			for i := range grid {
				grid[i] = grid[i][:m-1]
			}
			continue
		}
		grid = cut
		clear(forbid)
		jogging = false
	}
}

// leaves reports whether the cell after x on line i, below a node in the
// cell before x, is blank or drawn by an edge, which can run right along
// the node; the frame of a cluster keeps a cell off it
func leaves(grid [][]cell, i, x int) bool {
	after := grid[i][x+1]
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
func bendsAt(grid [][]cell, i, x int, toHi bool, next, prev, lo, hi int) []bend {
	a, b := grid[i-1], grid[i]
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
		if i < 0 || i >= len(grid) {
			return false
		}
		c := grid[i][x]
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
func bendLines(grid, cut [][]cell, path []int, next, prev, lo, hi int) (jogs int, failed [2]int, ok bool) {
	// has reports whether the cell at x of line i has the arm, with lines
	// past the grid having none
	has := func(i, x, arm int) bool {
		return i >= 0 && i < len(cut) && x >= 0 && x < len(cut[i]) && arms(cut[i][x].r)&arm != 0
	}
	for i := 1; i < len(path); i++ {
		xa, xb := path[i-1], path[i]
		for x := min(xa, xb) + 1; x < max(xa, xb); x++ {
			a, b := grid[i-1][x], grid[i][x]
			if arms(a.r)&next == 0 && arms(b.r)&prev == 0 {
				continue
			}
			bends := bendsAt(grid, i, x, xb > xa, next, prev, lo, hi)
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

// corner returns the line character with the arms in lines, with rounded
// corners as edges are drawn
func corner(lines int) rune {
	r := glyph(lines, 0)
	if c, ok := rounded[r]; ok {
		return c
	}
	return r
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

// arms returns the directions that the line character r joins toward;
// marks and dots join every way
func arms(r rune) int {
	if i := slices.Index(glyphs, r); i > 0 {
		mask := 0
		for _, arm := range []int{right, left, down, up} {
			if i%3 != 0 {
				mask |= arm
			}
			i /= 3
		}
		return mask
	}
	switch r {
	case '╭':
		return down | right
	case '╮':
		return down | left
	case '╰':
		return up | right
	case '╯':
		return up | left
	case '┊', '┋', '┆', '║', '▲', '▼', '↑', '↓':
		return up | down
	case '┈', '┉', '┄', '═', '◀', '▶', '←', '→':
		return left | right
	case '╔':
		return down | right
	case '╗':
		return down | left
	case '╚':
		return up | right
	case '╝':
		return up | left
	case '╟':
		return up | down | right
	case '╢':
		return up | down | left
	case '╥':
		return left | right | down
	case '╨':
		return left | right | up
	case '●', '○':
		return up | down | left | right
	}
	return 0
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

// encode writes the grid as lines of text without trailing blanks,
// colored with escape codes unless opts is nil
func encode(grid [][]cell, opts *Options) string {
	// the escape code parameters of each pair of cell colors
	codes := map[[2]uint32][2]string{}
	code := func(x cell) (string, string) {
		if opts == nil {
			return "39", "49"
		}
		k := [2]uint32{x.fg, x.bg}
		fb, ok := codes[k]
		if !ok {
			fb[0], fb[1] = opts.codes(x.fg, x.bg)
			codes[k] = fb
		}
		return fb[0], fb[1]
	}
	var out strings.Builder
	for _, line := range grid {
		end := 0
		for i, x := range line {
			if _, bg := code(x); x.r != ' ' || bg != "49" {
				end = i + 1
			}
		}
		f, b := "39", "49"
		for _, x := range line[:end] {
			want, bg := code(x)
			if x.r == ' ' {
				want = f // blanks show only the background
			}
			if want != f || bg != b {
				f, b = want, bg
				out.WriteString("\x1b[" + f + ";" + b + "m")
			}
			if x.r != covered {
				out.WriteRune(x.r)
			}
		}
		if f != "39" || b != "49" {
			out.WriteString("\x1b[0m")
		}
		out.WriteByte('\n')
	}
	return out.String()
}
