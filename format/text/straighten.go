package text

import (
	"cmp"
	"math/bits"
	"slices"
)

// mergeRows merges the first two neighboring rows with runs along them
// into one, where the lines of the lower one move up without meeting
// those of the upper: no cell gets runs along from both, and a line that
// leaves the upper row up and the lower row down runs on through both.
// Nodes, markers, text and the frames of clusters stay where they are. It
// returns whether it merged two rows.
func mergeRows(g grid) (grid, bool) {
	along := func(row []cell) bool {
		return slices.ContainsFunc(row, func(c cell) bool { return c.lines&(left|right) != 0 })
	}
	// most tries fail; they reuse these rows, and the one that merges
	// keeps its row
	var out, a, b []cell
	for r := 0; r+1 < len(g); r++ {
		if !along(g[r]) || !along(g[r+1]) {
			continue
		}
		row, ok := mergeRow(out, g[r], g[r+1])
		if ok {
			g[r] = row
			return slices.Delete(g, r+1, r+2), true
		}
		out = row
		// a line that turns along one of the rows can slide over to
		// clear the way, a few cells at most
		for _, end := range []struct{ r, dir int }{{r, up}, {r + 1, down}} {
			for x := range g[end.r] {
				for _, dx := range []int{1, -1, 2, -2, 3, -3} {
					moves, ok := slide(g, end.r, x, end.dir, dx)
					if !ok {
						continue
					}
					a, b = append(a[:0], g[r]...), append(b[:0], g[r+1]...)
					for _, m := range moves {
						switch m.r {
						case r:
							a[m.x] = m.c
						case r + 1:
							b[m.x] = m.c
						}
					}
					row, ok := mergeRow(out, a, b)
					out = row
					if ok {
						for _, m := range moves {
							g[m.r][m.x] = m.c
						}
						g[r] = row
						return slices.Delete(g, r+1, r+2), true
					}
				}
			}
		}
	}
	return g, false
}

// straighten takes the first step of a line aside out, sliding the line
// before or after it over, see slide. It returns whether it took one out.
func straighten(g grid) bool {
	for r, row := range g {
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
				if moves, ok := slide(g, r, from.x, g[r][from.x].lines&(up|down), from.dx); ok {
					for _, m := range moves {
						g[m.r][m.x] = m.c
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
func slide(g grid, r, x, dir, dx int) ([]move, bool) {
	blank := func(r, x int) bool { c := g.at(r, x); return c != nil && free(*c) }
	solid := func(r, x int) bool { c := g.at(r, x); return c != nil && c.solid }
	turn := g.at(r, x)
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
	turns := func(y int, dirs int) bool {
		run := dirs & (left | right)
		side := map[int]int{left: -1, right: 1}[run]
		lo, hi := min(x, x+dx), max(x, x+dx)
		for c := lo; c <= hi; c++ {
			if c == x {
				continue
			}
			p := g.at(y, c)
			// toward the run it shrinks over its own run, onto the turn at
			// its far end where the line goes on straight; away from it,
			// it grows over blanks
			end := c == x+dx && p != nil && p.lines == opposite(run)|opposite(dirs&(up|down)) && p.owner[bits.TrailingZeros(uint(opposite(run)))] == id
			if dx*side > 0 && !end && (p == nil || p.lines != left|right || p.owner[2] != id) || dx*side < 0 && !blank(y, c) {
				return false
			}
			if end {
				dirs = up | down
			}
		}
		line := cell{r: draw(left | right), fg: turn.fg, lines: left | right, owner: [4]int{id, id, id, id}}
		for c := lo; c <= hi; c++ {
			switch {
			case c == x+dx:
				moves = append(moves, move{y, c, cell{r: draw(dirs), fg: turn.fg, lines: dirs, owner: [4]int{id, id, id, id}}})
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
		p := g.at(y, x)
		switch {
		case p == nil:
			return nil, false
		case p.lines&(up|down) == up|down && p.owner[0] == id && p.owner[1] == id && !p.solid:
			// a line it crosses it crosses where it goes too, and leaves
			// whole where it was; a cell off nodes, as unjog keeps
			q := g.at(y, x+dx)
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
			below, to := g.at(y+1, x), g.at(y+1, x+dx)
			if !blank(y, x+dx) || below == nil || to == nil || below.node == 0 || to.node != below.node || to.r != below.r {
				return nil, false
			}
			moves = append(moves, move{y, x, cell{r: ' '}}, move{y, x + dx, *p})
			return moves, true
		case p.lines&opposite(dir) != 0 && bits.OnesCount(uint(p.lines)) == 2 && p.lines&(left|right) != 0 && p.owner[bits.TrailingZeros(uint(opposite(dir)))] == id:
			return moves, turns(y, p.lines)
		case p.solid && p.node != 0 && arms(p.r)&opposite(dir) != 0:
			// a port on the side of a box moves along its straight side
			to := g.at(y, x+dx)
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

// mergeRow returns the cells of a and b in one row, in out's storage when
// it has room, see mergeRows; when they don't merge, it returns that
// storage for the next try
func mergeRow(out, a, b []cell) ([]cell, bool) {
	out = slices.Grow(out[:0], max(len(a), len(b)))[:max(len(a), len(b))]
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
				return out, false
			}
		case p.lines&down != 0 != (q.lines&up != 0):
			return out, false // a line that ends between them
		case p.lines&up != 0 && q.lines&down != 0 && !link:
			return out, false // two lines would join
		case p.lines&(left|right) != 0 && q.lines&(left|right) != 0:
			return out, false
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
func unjog(g grid) (changed bool) {
	at := func(p pos) *cell { return g.at(p.r, p.c) }
	// reroute moves a line from the old cells to the new ones, both with
	// the arm in into the first, ending in head when it is set, if it can
	// fewer has reroute take only a new line that crosses fewer lines
	fewer := false
	reroute := func(in int, old, path []pos, dirs []int, head *cell) bool {
		olds := map[pos]int{}
		_, oldArms := route(in, down, old...)
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
				if q.r == '╂' && dirs[i] != up|down && dirs[i] != left|right {
					return false
				}
				continue
			}
			straight := dirs[i] == up|down || dirs[i] == left|right
			across := straight && (dirs[i] == up|down && q.r == '─' || dirs[i] == left|right && q.r == '│')
			if q.solid || !free(*q) && !across {
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
					if dirs[i]&(1<<arm) != 0 {
						q.owner[arm] = id
					}
				}
			default:
				*q = cell{r: corner(dirs[i]), fg: fg, lines: dirs[i], owner: [4]int{id, id, id, id}}
			}
		}
		return true
	}
	for r := range g {
		for c := range g[r] {
			top := pos{r, c}
			if q := at(top); junction(g.runeAt(top)) && !q.solid {
				// a line leaving a junction along the row and a step after
				// it: one, on the row of the junction, which stays
				for _, dir := range []int{-1, 1} {
					side, back := right, left
					if dir < 0 {
						side, back = left, right
					}
					first := pos{r, c + dir}
					b, ok := g.run(top, dir)
					if arms(q.r)&side == 0 || !ok || at(first).solid {
						continue
					}
					mid := pos{g.bottom(pos{r, b}), b}
					_, c2, turns := g.step(mid)
					if !turns || (c2-c)*dir <= 0 {
						continue
					}
					end := pos{mid.r, c2}
					old, _ := route(back, down, first, pos{r, b}, mid, end)
					path, dirs := route(back, down, first, pos{r, c2}, end)
					if reroute(back, old, path, dirs, nil) {
						changed = true
					}
				}
				continue
			}
			_, b, ok := g.step(top)
			if !ok {
				continue
			}
			mid := pos{g.bottom(pos{r, b}), b}
			if dir, c2, turns := g.step(mid); dir != 0 {
				// two steps: one, on the lower row or else the upper;
				// a second that runs into something ends before it
				end, out := pos{mid.r, c2}, down
				if !turns {
					end, out = pos{mid.r, c2 - dir}, map[int]int{-1: left, 1: right}[dir]
				}
				old, _ := route(up, out, top, pos{r, b}, mid, end)
				vias := []pos{{mid.r, c}, {r, c2}}
				if !turns {
					vias = vias[:1] // the run goes on along the lower row
				}
				for _, via := range vias {
					path, dirs := route(up, out, top, via, end)
					if reroute(up, old, path, dirs, nil) {
						changed = true
						break
					}
				}
				continue
			}
			if g.runeAt(mid) != '▼' {
				continue
			}
			// a last step before an arrowhead into a box: the arrowhead
			// goes along when the box goes on under the line, with no
			// corner between
			box := true
			for x := min(c, b); x <= max(c, b); x++ {
				p := at(pos{mid.r + 1, x})
				box = box && p != nil && p.solid && (x == b || boxRowSide(p.r))
			}
			if !box {
				continue
			}
			head := *at(mid)
			old, _ := route(up, down, top, pos{r, b}, mid)
			path, dirs := route(up, down, top, pos{mid.r, c})
			if reroute(up, old, path, dirs, &head) {
				changed = true
			}
		}
	}
	// a step slides along its line to where the line crosses fewer
	// lines, as two lines can cross twice to swap back
	fewer = true
	for r := range g {
		for c := range g[r] {
			top := pos{r, c}
			_, b, ok := g.step(top)
			if !ok {
				continue
			}
			first := r
			for verticalOrCrossing(g.runeAt(pos{first - 1, c})) {
				first--
			}
			last := g.bottom(pos{r, b}) - 1
			old, _ := route(up, down, pos{first, c}, top, pos{r, b}, pos{last, b})
			for row := first; row <= last; row++ {
				if row == r {
					continue
				}
				path, dirs := route(up, down, pos{first, c}, pos{row, c}, pos{row, b}, pos{last, b})
				if reroute(up, old, path, dirs, nil) {
					changed = true
					break
				}
			}
		}
	}
	return changed
}

// pos is the row and column of a cell
type pos struct{ r, c int }

// runeAt returns the character of the cell at p, 0 off the grid
func (g grid) runeAt(p pos) rune {
	if q := g.at(p.r, p.c); q != nil {
		return q.r
	}
	return 0
}

// run returns where a line leaving p along the row in dir runs to, and
// whether it goes on down there; when it does not, it runs into
// something there, such as an arrowhead
func (g grid) run(p pos, dir int) (to int, turns bool) {
	c := p.c + dir
	for horizontalOrCrossing(g.runeAt(pos{p.r, c})) {
		c += dir
	}
	corner := '╭'
	if dir > 0 {
		corner = '╮'
	}
	return c, g.runeAt(pos{p.r, c}) == corner
}

// step returns the direction of the step of a line that comes down to
// p, where it runs along to and whether it goes on down there
func (g grid) step(p pos) (dir, to int, turns bool) {
	switch {
	case g.runeAt(p) == '╰':
		dir = 1
	case g.runeAt(p) == '╯':
		dir = -1
	default:
		return 0, 0, false
	}
	to, turns = g.run(p, dir)
	return dir, to, turns
}

// bottom returns the row where a line going down from p turns or ends
func (g grid) bottom(p pos) int {
	r := p.r + 1
	for verticalOrCrossing(g.runeAt(pos{r, p.c})) {
		r++
	}
	return r
}

// route returns the cells along the points, with the arms of each in
// the line: in into the first, out out of the last
func route(in, out int, points ...pos) ([]pos, []int) {
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
	dirs := make([]int, len(path))
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
			dirs[i] |= in
		} else {
			dirs[i] |= toward(p, path[i-1])
		}
		if i == len(path)-1 {
			dirs[i] |= out
		} else {
			dirs[i] |= toward(p, path[i+1])
		}
	}
	return path, dirs
}
