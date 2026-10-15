package text

import (
	"cmp"
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
	var moves []move
	for r := 0; r+1 < len(g); r++ {
		if !along(g[r]) || !along(g[r+1]) {
			continue
		}
		row, stuck, ok := mergeRow(out, g[r], g[r+1])
		if ok {
			g[r] = row
			return slices.Delete(g, r+1, r+2), true
		}
		out = row
		// a line that turns along one of the rows can slide over to
		// clear the way, a few cells at most
		for _, end := range []struct {
			r   int
			dir uint8
		}{{r, up}, {r + 1, down}} {
			for x := range g[end.r] {
				for _, dx := range []int{1, -1, 2, -2, 3, -3} {
					var ok bool
					moves, ok = slide(moves, g, end.r, x, end.dir, dx)
					// the rows merge cell by cell, so a slide that leaves
					// the cell they don't merge at as it is can't help
					if !ok || !slices.ContainsFunc(moves, func(m move) bool { return m.x == stuck && (m.r == r || m.r == r+1) }) {
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
					row, _, ok := mergeRow(out, a, b)
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
	var moves []move
	for r, row := range g {
		for x, c := range row {
			// the turn at the left end of a step
			if c.lines&right == 0 || !isTurn(c.lines) {
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
				var ok bool
				if moves, ok = slide(moves, g, r, from.x, g[r][from.x].lines&(up|down), from.dx); ok {
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

// slide returns the cells, in moves' storage, that move the line leaving the turn at r, x
// toward dir shift cells over, along with the runs it turns from at both
// ends, or into an arrowhead or a port on a box at the far end, which
// moves along the box. The line moves only through blanks, of one edge, and its runs
// only grow over blanks and shrink over themselves.
func slide(moves []move, g grid, r, x int, dir uint8, shift int) ([]move, bool) {
	blank := func(r, x int) bool { c := g.at(r, x); return c != nil && free(*c) }
	solid := func(r, x int) bool { c := g.at(r, x); return c != nil && c.solid }
	turn := g.at(r, x)
	if turn == nil || turn.lines&dir == 0 || !isTurn(turn.lines) {
		return moves, false
	}
	id := turn.ownerOn(dir)

	// a dashed line, whose corners are square, stays dashed, see canvas.line
	dashed := turn.r == glyph(turn.lines, 0)
	draw := func(lines uint8) rune {
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

	// a cell of the line in lines, drawn as the turn is
	lineCell := func(lines uint8) cell {
		return cell{r: draw(lines), fg: turn.fg, lines: lines, owner: [4]edgeID{id, id, id, id}}
	}

	step := dy(dir)
	moves = moves[:0]

	// turns moves the turn at row y from x along its run, which grows or
	// shrinks to meet it at x+shift
	turns := func(y int, dirs uint8) bool {
		run := dirs & (left | right)
		side := dx(run)

		// toward the run it shrinks, away from it it grows
		toward, away := shift*side > 0, shift*side < 0
		lo, hi := min(x, x+shift), max(x, x+shift)
		for c := lo; c <= hi; c++ {
			if c == x {
				continue
			}
			p := g.at(y, c)
			// toward the run it shrinks over its own run, onto the turn at
			// its far end where the line goes on straight; away from it,
			// it grows over blanks
			back := opposite(run)
			end := c == x+shift && p.drawnBy(id, back) && p.lines == back|opposite(dirs&(up|down))
			ownRun := p.drawnBy(id, left) && p.lines == left|right
			if toward && !end && !ownRun || away && !blank(y, c) {
				return false
			}
			if end {
				dirs = up | down
			}
		}

		line := lineCell(left | right)
		for c := lo; c <= hi; c++ {
			switch {
			case c == x+shift:
				moves = append(moves, move{y, c, lineCell(dirs)})
			case toward:
				moves = append(moves, move{y, c, cell{r: ' '}})
			default:
				moves = append(moves, move{y, c, line})
			}
		}

		return true
	}

	if !turns(r, turn.lines) {
		return moves, false
	}
	vertical := lineCell(up | down)
	for y := r + step; ; y += step {
		p := g.at(y, x)
		switch {
		case p == nil:
			return moves, false
		case p.drawnBy(id, up|down) && !p.solid:
			// a line it crosses it crosses where it goes too, and leaves
			// whole where it was; a cell off nodes, as unjog keeps
			q := g.at(y, x+shift)
			movable := q != nil && !q.solid && !q.keep && !q.glue && q.need == 0
			other := movable && q.lines == left|right && q.ownerOn(left) != id
			across := p.lines != up|down && other
			upright := p.lines == up|down || p.lines == up|down|left|right
			besideBox := solid(y, x+shift-1) || solid(y, x+shift+1)
			if !upright || !blank(y, x+shift) && !across || besideBox {
				return moves, false
			}
			was := cell{r: ' '}
			if p.lines != up|down {
				was = *p
				was.r, was.lines = '─', left|right
				was.setOwner(up|down, 0)
			}
			to := vertical
			if across {
				to = *q
				to.r, to.lines = '╂', up|down|left|right
				to.setOwner(up|down, id)
			}
			moves = append(moves, move{y, x, was}, move{y, x + shift, to})
			continue
		case p.solid && p.node == 0 && dir == down:
			// an arrowhead onto the straight top of a box
			below, to := g.at(y+1, x), g.at(y+1, x+shift)
			sameBox := below != nil && to != nil && below.node != 0 && to.node == below.node && to.r == below.r
			if p.style&pinned != 0 || !blank(y, x+shift) || !sameBox {
				return moves, false
			}
			moves = append(moves, move{y, x, cell{r: ' '}}, move{y, x + shift, *p})
			return moves, true
		case p.lines&opposite(dir) != 0 && isTurn(p.lines) && p.ownerOn(opposite(dir)) == id:
			return moves, turns(y, p.lines)
		case p.solid && p.node != 0 && arms(p.r)&opposite(dir) != 0:
			// a port on the side of a box moves along its straight side
			to := g.at(y, x+shift)
			if p.style&pinned != 0 || to == nil || to.node != p.node || arms(to.r) != left|right {
				return moves, false
			}
			port, side := *p, *to
			port.r, side.r = p.r, to.r
			moves = append(moves, move{y, x, side}, move{y, x + shift, port})
			return moves, true
		default:
			return moves, false
		}
	}
}

// mergeRow returns the cells of a and b in one row, in out's storage when
// it has room, see mergeRows; when they don't merge, it returns that
// storage for the next try, and the first cell they don't merge at
func mergeRow(out, a, b []cell) ([]cell, int, bool) {
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
				return out, x, false
			}
		case p.lines&down != 0 != (q.lines&up != 0):
			return out, x, false // a line that ends between them
		case p.lines&up != 0 && q.lines&down != 0 && !link:
			return out, x, false // two lines would join
		case p.lines&(left|right) != 0 && q.lines&(left|right) != 0:
			return out, x, false
		}

		lines := p.lines&^down | q.lines&^up
		c := cell{r: ' ', fg: p.fg, bg: p.bg, lines: lines}
		for arm := range 4 {
			switch bit := uint8(1) << arm; {
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
		case lines == up|down|left|right && c.ownerOn(up) != c.ownerOn(left):
			c.r = '╂' // a line across another
		default:
			c.r = corner(lines)
		}

		out[x] = c
	}

	return out, -1, true
}

// rerouteMarks marks the cells of the lines unjog reroutes, see unjog; it
// is kept across the calls of unjog on one grid, as the tries go on
// counting, so that the marks of earlier tries never need clearing
type rerouteMarks struct {
	rows [][]rerouteMark
	try  uint32
}

// rerouteMark marks a cell of the old line of a try, with its arms, and of
// the new
type rerouteMark struct {
	old, new uint32
	arms     uint8
}

// unjog straightens lines down the grid: two steps aside in a row become
// one, on the row of the first or of the second, and a last step right
// before an arrowhead takes the arrowhead along, onto the same box; a
// turn of the layout and a jog of compaction can add up so. A line leaving a
// junction along its row, where lines merge, is a first step too, which
// becomes one on the row of the junction, as the junction stays. A step also
// slides along its line to where the line crosses fewer lines. Steps are lines
// of their own, which nothing joins. The new line crosses no more lines
// than the old, keeps a cell off nodes, and lines with a label beside
// them stay.
func unjog(g grid, mk *rerouteMarks) (changed bool) {
	u := &unjogger{g: g, mk: mk}
	if len(mk.rows) < len(g) {
		mk.rows = append(mk.rows, make([][]rerouteMark, len(g)-len(mk.rows))...)
	}

	// one sweep tries both on every cell, so that a step that can't be
	// joined to the next slides in the same sweep
	for r := range g {
		for c := range g[r] {
			if u.jog(r, c) || u.slide(r, c) {
				changed = true
			}
		}
	}

	return changed
}

// unjogger reroutes the lines of a grid, see unjog. The cells of the old
// line, with their arms, and of the new are marked with the try they are
// of, so that every reroute marks them anew without clearing.
type unjogger struct {
	g  grid
	mk *rerouteMarks
	// the try that marked the old line, which the tries of new lines for
	// one old keep
	oldTry uint32
	// the cells of the old line
	olds []point
	// the corners of the old line, from oldDirs, which leave the last cell
	// the way the new line does, unlike the arms of the old
	oldCorners int
	// whether a label is beside each of olds, and whether it is a crossing
	labeled, crossed []bool
	// the old line, the new one and the arms of the old in reroute, which
	// every try routes anew
	oldPath, newPath, armsPath []point
	oldDirs, newDirs, armsDirs []uint8
}

func (u *unjogger) at(p point) *cell { return u.g.at(p.Y, p.X) }

func (u *unjogger) markAt(p point) *rerouteMark {
	g, mk := u.g, u.mk
	if p.Y < 0 || p.Y >= len(g) || p.X < 0 || p.X >= len(g[p.Y]) {
		return nil
	}
	if len(mk.rows[p.Y]) < len(g[p.Y]) {
		mk.rows[p.Y] = append(mk.rows[p.Y], make([]rerouteMark, len(g[p.Y])-len(mk.rows[p.Y]))...)
	}
	return &mk.rows[p.Y][p.X]
}

func (u *unjogger) isNew(p point) bool { m := u.markAt(p); return m != nil && m.new == u.mk.try }

func (u *unjogger) isOld(p point) bool { m := u.markAt(p); return m != nil && m.old == u.oldTry }

// oldArms returns the arms of the old line at p, none off it
func (u *unjogger) oldArms(p point) uint8 {
	if m := u.markAt(p); m != nil && m.old == u.oldTry {
		return m.arms
	}
	return 0
}

// corners returns how many of dirs turn
func corners(dirs []uint8) (n int) {
	for _, d := range dirs {
		if !isStraight(d) {
			n++
		}
	}
	return n
}

// reroute moves a line from the old cells to the new ones, both with the
// arm in into the first, ending in head when it is set, if it can and the
// new line has fewer crossings, or as many and fewer corners; again tries
// another new line for the old of the try before
func (u *unjogger) reroute(in uint8, old []point, again bool, path []point, dirs []uint8, head *cell) bool {
	u.mk.try++
	if !again {
		u.markOld(in, old)
	}
	for _, p := range path {
		if m := u.markAt(p); m != nil {
			m.new = u.mk.try
		}
	}
	if !u.accepts(path, dirs) {
		return false
	}
	u.draw(old, path, dirs, head)
	return true
}

// markOld marks old as the line of this try, with its arms, and notes
// which of its cells have a label beside them and which are crossings
func (u *unjogger) markOld(in uint8, old []point) {
	u.oldTry = u.mk.try
	u.olds = u.olds[:0]
	u.armsPath, u.armsDirs = route(u.armsPath, u.armsDirs, in, down, old...)
	u.oldCorners = corners(u.oldDirs)
	for i, p := range old {
		if m := u.markAt(p); m != nil {
			if m.old != u.oldTry {
				m.old = u.oldTry
				u.olds = append(u.olds, p)
			}
			m.arms = u.armsDirs[i]
		}
	}

	u.labeled, u.crossed = u.labeled[:0], u.crossed[:0]
	for _, p := range u.olds {
		label := false
		for _, d := range []point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			if q := u.at(p.add(d)); q != nil && !q.solid && (q.glue || q.keep) {
				label = true
			}
		}
		u.labeled, u.crossed = append(u.labeled, label), append(u.crossed, u.at(p).r == '╂')
	}
}

// accepts reports whether the new line, marked, can replace the old: it
// leaves no label behind, runs over blanks or across lines, keeps a cell
// off nodes, and crosses fewer lines, or as many with fewer corners
func (u *unjogger) accepts(path []point, dirs []uint8) bool {
	added, removed := 0, 0
	for i, p := range u.olds {
		if u.isNew(p) {
			continue
		}

		// a label beside the line keeps it
		if u.labeled[i] {
			return false
		}
		if u.crossed[i] {
			removed++
		}
	}

	for i, p := range path {
		q := u.at(p)
		if q == nil {
			return false
		}
		if u.isOld(p) {
			// a crossing on the old line stays one on the new
			if q.r == '╂' && !isStraight(dirs[i]) {
				return false
			}
			continue
		}
		across := dirs[i] == up|down && q.r == '─' || dirs[i] == left|right && q.r == '│'
		if q.solid || !free(*q) && !across {
			return false
		}
		if across {
			added++
		}
		for _, dc := range []int{-1, 1} {
			// a cell off nodes, other than an arrowhead that moves
			n := point{p.X + dc, p.Y}
			if q := u.at(n); q != nil && q.solid && u.oldArms(n) == 0 {
				return false
			}
		}
	}

	return cmp.Compare(added, removed) < 0 || added == removed && corners(dirs) < u.oldCorners
}

// draw clears the old line, but for the lines it crossed, and draws the
// new, ending in head when it is set
func (u *unjogger) draw(old []point, path []point, dirs []uint8, head *cell) {
	// the edge of the line, by the arms it draws, not of lines it crosses
	fg, id := u.at(old[0]).fg, edgeID(0)
	for _, p := range u.olds {
		a := u.oldArms(p)
		for arm := range 4 {
			if a&(1<<arm) != 0 && u.at(p).lines&(1<<arm) != 0 && u.at(p).owner[arm] != 0 {
				id = u.at(p).owner[arm]
			}
		}
	}

	for _, p := range u.olds {
		a := u.oldArms(p)
		if u.isNew(p) {
			continue
		}
		if q := u.at(p); q.r == '╂' {
			// the line it crossed goes on
			q.lines = (up | down | left | right) &^ a
			q.r = corner(q.lines)
		} else {
			*q = cell{r: ' '}
		}
	}

	for i, p := range path {
		q := u.at(p)
		switch {
		case i == len(path)-1 && head != nil:
			*q = *head
		case q.r == '╂' || q.r != ' ' && u.oldArms(p) == 0:
			q.r, q.lines = '╂', up|down|left|right
			q.setOwner(dirs[i], id)
		default:
			*q = cell{r: corner(dirs[i]), fg: fg, lines: dirs[i], owner: [4]edgeID{id, id, id, id}}
		}
	}
}

// jog makes a step at r, c and the one after it one, see unjog
func (u *unjogger) jog(r, c int) (moved bool) {
	top := point{c, r}
	if q := u.at(top); junction(u.g.runeAt(top)) && !q.solid {
		// a line leaving a junction along the row and a step after
		// it: one, on the row of the junction, which stays
		for _, dir := range []int{-1, 1} {
			side := rowDir(dir)
			back := opposite(side)
			first := point{c + dir, r}
			b, ok := u.g.run(top, dir)
			if arms(q.r)&side == 0 || !ok || u.at(first).solid {
				continue
			}
			mid := point{b, u.g.bottom(point{b, r})}
			_, c2, turns := u.g.step(mid)
			if !turns || (c2-c)*dir <= 0 {
				continue
			}
			end := point{c2, mid.Y}
			u.oldPath, u.oldDirs = route(u.oldPath, u.oldDirs, back, down, first, point{b, r}, mid, end)
			u.newPath, u.newDirs = route(u.newPath, u.newDirs, back, down, first, point{c2, r}, end)
			if u.reroute(back, u.oldPath, false, u.newPath, u.newDirs, nil) {
				moved = true
			}
		}

		return moved
	}
	_, b, ok := u.g.step(top)
	if !ok {
		return false
	}
	mid := point{b, u.g.bottom(point{b, r})}
	if dir, c2, turns := u.g.step(mid); dir != 0 {
		// two steps: one, on the lower row or else the upper;
		// a second that runs into something ends before it
		end, out := point{c2, mid.Y}, uint8(down)
		if !turns {
			end, out = point{c2 - dir, mid.Y}, rowDir(dir)
		}
		u.oldPath, u.oldDirs = route(u.oldPath, u.oldDirs, up, out, top, point{b, r}, mid, end)
		vias := []point{{c, mid.Y}, {c2, r}}
		if !turns {
			vias = vias[:1] // the run goes on along the lower row
		}
		for i, via := range vias {
			u.newPath, u.newDirs = route(u.newPath, u.newDirs, up, out, top, via, end)
			if u.reroute(up, u.oldPath, i > 0, u.newPath, u.newDirs, nil) {
				moved = true
				break
			}
		}

		return moved
	}
	if u.g.runeAt(mid) != '▼' || u.at(mid).style&pinned != 0 {
		return false
	}

	// a last step before an arrowhead into a box: the arrowhead
	// goes along when the box goes on under the line, with no
	// corner between
	box := true
	for x := min(c, b); x <= max(c, b); x++ {
		p := u.at(point{x, mid.Y + 1})
		box = box && p != nil && p.solid && (x == b || boxRowSide(p.r))
	}
	if !box {
		return false
	}
	head := *u.at(mid)
	u.oldPath, u.oldDirs = route(u.oldPath, u.oldDirs, up, down, top, point{b, r}, mid)
	u.newPath, u.newDirs = route(u.newPath, u.newDirs, up, down, top, point{c, mid.Y})
	if u.reroute(up, u.oldPath, false, u.newPath, u.newDirs, &head) {
		moved = true
	}
	return moved
}

// slide moves a step along its line to where the line crosses fewer
// lines, as two lines can cross twice to swap back
func (u *unjogger) slide(r, c int) (moved bool) {
	top := point{c, r}
	_, b, ok := u.g.step(top)
	if !ok {
		return false
	}
	first := r
	for verticalOrCrossing(u.g.runeAt(point{c, first - 1})) {
		first--
	}
	last := u.g.bottom(point{b, r}) - 1
	u.oldPath, u.oldDirs = route(u.oldPath, u.oldDirs, up, down, point{c, first}, top, point{b, r}, point{b, last})

	// a line that crosses none can't cross fewer
	if !slices.ContainsFunc(u.oldPath, func(p point) bool { q := u.at(p); return q != nil && q.r == '╂' }) {
		return false
	}
	again := false
	for row := first; row <= last; row++ {
		if row == r {
			continue
		}
		u.newPath, u.newDirs = route(u.newPath, u.newDirs, up, down, point{c, first}, point{c, row}, point{b, row}, point{b, last})
		if u.reroute(up, u.oldPath, again, u.newPath, u.newDirs, nil) {
			moved = true
			break
		}
		again = true
	}

	return moved
}

// runeAt returns the character of the cell at p, 0 off the grid
func (g grid) runeAt(p point) rune {
	if q := g.at(p.Y, p.X); q != nil {
		return q.r
	}
	return 0
}

// run returns where a line leaving p along the row in dir runs to, and
// whether it goes on down there; when it does not, it runs into
// something there, such as an arrowhead
func (g grid) run(p point, dir int) (to int, turns bool) {
	c := p.X + dir
	for horizontalOrCrossing(g.runeAt(point{c, p.Y})) {
		c += dir
	}
	corner := '╭'
	if dir > 0 {
		corner = '╮'
	}
	return c, g.runeAt(point{c, p.Y}) == corner
}

// step returns the direction of the step of a line that comes down to
// p, where it runs along to and whether it goes on down there
func (g grid) step(p point) (dir, to int, turns bool) {
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
func (g grid) bottom(p point) int {
	r := p.Y + 1
	for verticalOrCrossing(g.runeAt(point{p.X, r})) {
		r++
	}
	return r
}

// route returns the cells along the points, with the arms of each in
// the line: in into the first, out out of the last; they are in the
// storage of path and dirs
func route(path []point, dirs []uint8, in, out uint8, points ...point) ([]point, []uint8) {
	n := 1
	for i := 1; i < len(points); i++ {
		n += points[i].dist(points[i-1])
	}
	path = slices.Grow(path[:0], n)
	for i, p := range points {
		if i == 0 {
			path = append(path, p)
			continue
		}
		q := path[len(path)-1]
		dr, dc := cmp.Compare(p.Y, q.Y), cmp.Compare(p.X, q.X)
		for q != p {
			q.Y, q.X = q.Y+dr, q.X+dc
			path = append(path, q)
		}
	}

	dirs = slices.Grow(dirs[:0], len(path))[:len(path)]
	clear(dirs)
	toward := func(from, to point) uint8 {
		switch {
		case to.Y < from.Y:
			return up
		case to.Y > from.Y:
			return down
		case to.X < from.X:
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

// free reports whether c is an empty cell that a line can be drawn in
func free(c cell) bool {
	return c.r == ' ' && c.bg == 0 && !c.keep && !c.glue && c.need == 0 && c.label == 0
}
