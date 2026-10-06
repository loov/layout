package text

import (
	"math/bits"
	"slices"
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
//
// The grids carving makes reuse the cells in sc of those it is done with;
// the one it returns stays out of sc.
func carve(g grid, sideways bool, sc *scratch) grid {
	// the first grid, the canvas's, isn't carving's to reuse
	var cells []cell
	// seams keep the blanks past the end of lines, trailing rows of them
	// included; keep one of those as the bottom margin
	blank := func(row []cell) bool {
		return !slices.ContainsFunc(row, func(c cell) bool { return c.r != ' ' || c.bg != 0 })
	}
	rows := func(g grid, cells []cell, stubs bool) (grid, []cell) {
		t, tcells := sc.transpose(g)
		sc.release(cells)
		t, tcells = seams(t, tcells, seamOpts{lines: columnLines, turn: sideways, stubs: stubs}, sc)
		g, cells = sc.transpose(t)
		sc.release(tcells)
		for len(g) > 1 && blank(g[len(g)-1]) && blank(g[len(g)-2]) {
			g = g[:len(g)-1]
		}
		return g, cells
	}
	// the stubs of lines leaving a node go last: lines can still run
	// further along them while the columns are carved
	g, cells = rows(g, nil, false)
	g, cells = seams(g, cells, seamOpts{lines: rowLines, turn: !sideways, bend: true}, sc)
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
	g, _ = rows(g, cells, true)
	hug(g)
	return g
}

// seamOpts configure seams
type seamOpts struct {
	lines seamAxis
	turn  bool // seams turn between lines
	bend  bool // seams can bend the lines they cross, see seams
	stubs bool // the first cell of a straight line that goes on may go
}

// seamAxis describes the lines of a grid that seams run across
type seamAxis struct {
	next, prev uint8 // the arms that join a cell to the line after and before it
	lo, hi     uint8 // the directions along a line, toward its start and end
	seam, mark uint8 // the class bits of straight runs and markers, see class
}

// columns reports whether the lines are the columns of the drawing
func (ax seamAxis) columns() bool { return ax.next&(left|right) != 0 }

// straightRun reports whether c is a plain straight run across the lines,
// off nodes, arrowheads and turns, which a jog can bend
func (ax seamAxis) straightRun(c cell) bool {
	return !c.keep && !c.solid && c.r == glyph(ax.next|ax.prev, 0)
}

var (
	// the lines are the columns of the drawing, transposed, so seams
	// take out rows
	columnLines = seamAxis{right, left, up, down, seamV, markV}
	// the lines are the rows, so seams take out columns
	rowLines = seamAxis{down, up, left, right, seamH, markH}
)

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
//
// The lines of g are in cells, or in no block of sc's when that is nil;
// seams returns the block that the lines it returns are in, and gives sc
// the other blocks it is done with.
func seams(g grid, cells []cell, opts seamOpts, sc *scratch) (grid, []cell) {
	p := &seamPass{opts: opts, forbid: map[[2]int]bool{}}
	// the lines run across the ranks when they are columns, see carve;
	// along is a plain line along the lines
	p.across, p.along = opts.lines.columns(), glyph(opts.lines.lo|opts.lines.hi, 0)
	turn, bend := opts.turn, opts.bend
	// buffers reused by every pass, which is the whole grid each time
	var fromBuf, prevBest, curBest, reach, src, path []int
	var joined []bool
	var from [][]int
	// spare holds the next cut, in spareCells
	var spare grid
	var spareCells []cell
	defer func() { sc.release(spareCells) }()
	// the class of every cell, which the passes check over and over;
	// cells seams make get theirs as they are made
	blank := cell{r: ' ', kind: class(' ')}
	for _, line := range g {
		for x := range line {
			line[x].kind = class(line[x].r)
		}
	}
	for {
		for i := range g {
			g[i] = append(g[i], blank)
		}
		n := len(g)
		if n == 0 {
			return g, cells
		}
		m := len(g[0])
		p.g = g
		// the cells past the end of each line are blank
		p.ends = slices.Grow(p.ends[:0], n)[:n]
		for i, line := range g {
			p.ends[i] = 0
			for x := len(line) - 1; x >= 0; x-- {
				if line[x].r != ' ' || line[x].bg != 0 {
					p.ends[i] = x + 1
					break
				}
			}
		}
		p.width = slices.Max(p.ends)
		// best[i][x] is the cost of the cheapest seam through the lines up
		// to i that ends at x, coming from from[i][x] on the line before.
		// Past the last cell that isn't plain blank, every column is like
		// the first one there, which seams prefer as it is further left;
		// leave them out.
		w := 0
		for _, line := range g {
			for x := len(line) - 1; x >= w; x-- {
				if line[x] != blank {
					w = x + 1
					break
				}
			}
		}
		w = min(m, w+1)
		// cuts leave plain blanks at the end of the lines, which every
		// pass would copy again; keep one past the columns seams use
		if w+1 < m {
			for i := range g {
				g[i] = g[i][:w+1]
			}
			m = w + 1
		}
		// only the line before is needed of best; from is kept whole to
		// trace the seam back
		fromBuf = slices.Grow(fromBuf[:0], n*w)[:n*w]
		from = slices.Grow(from[:0], n)[:n]
		prevBest, curBest = slices.Grow(prevBest[:0], w)[:w], slices.Grow(curBest[:0], w)[:w]
		reach, src = slices.Grow(reach[:0], w)[:w], slices.Grow(src[:0], w)[:w]
		joined = slices.Grow(joined[:0], w)[:w]
		p.runs = slices.Grow(p.runs[:0], m)[:m]
		for i := range n {
			from[i] = fromBuf[i*w : (i+1)*w]
			prevBest, curBest = curBest, prevBest
			p.costs(i, curBest)
			if i == 0 {
				continue
			}
			if !turn {
				for x := range w {
					curBest[x] += prevBest[x]
					from[i][x] = x
				}
				continue
			}
			joins(g[i-1][:w], g[i][:w], joined, opts.lines)
			p.sweep(i, prevBest, curBest, from[i], reach, src, joined)
		}
		last := curBest
		end := 0
		for x := range w {
			if last[x] < last[end] {
				end = x
			}
		}
		if last[end] >= n*seamTrailing {
			for i := range g {
				g[i] = g[i][:m-1]
			}
			if bend && turn && !p.jogging {
				p.jogging = true
				continue
			}
			return g, cells
		}
		path = slices.Grow(path[:0], n)[:n]
		for i, x := n-1, end; i >= 0; i-- {
			path[i] = x
			x = from[i][x]
		}
		if spare == nil {
			// lines as long as these, which the cuts make shorter, with
			// room for the blank that the next pass adds
			spareCells = sc.take(n * m)
			spare = make(grid, n)
			for i := range spare {
				spare[i] = spareCells[i*m : i*m : (i+1)*m]
			}
		}
		cut := spare
		for i, x := range path {
			cut[i] = append(append(cut[i][:0], g[i][:x]...), g[i][x+1:]...)
		}
		jogs, failed, ok := bendLines(g, cut, path, opts.lines)
		if ok && jogs > 1 {
			// the cheapest seam left has the fewest jogs; one column is
			// not worth more than one
			for i := range g {
				g[i] = g[i][:m-1]
			}
			return g, cells
		}
		if !ok {
			p.forbid[failed] = true
			for i := range g {
				g[i] = g[i][:m-1]
			}
			continue
		}
		g, spare = cut, g
		cells, spareCells = spareCells, cells
		clear(p.forbid)
		p.jogging = false
	}
}

// the cost of a seam through a cell, and of each cell it moves along
const (
	seamTrailing = 1 << 20 // past the end of the line, which removes nothing
	seamBlocked  = 1 << 40
	seamJog      = 1 << 16 // crossing a straight line, which bends it
	seamExtend   = 4       // crossing a line where it turns
)

// seamPass is what the costs of the seams of one pass of seams depend on.
// The loops over the cells are methods of their own, rather than parts
// of seams, so that each has the registers to itself.
type seamPass struct {
	g       grid
	ends    []int // the cells past the end of each line are blank
	width   int   // the longest line
	runs    []int // the length of the run of equal need around each cell, see costs
	jogging bool  // only seams that cross lines are left
	// crossings that failed to jog, by line and cell, until a seam is cut
	forbid map[[2]int]bool
	opts   seamOpts
	across bool // the lines run across the ranks
	along  rune // a plain line along the lines
}

// costs sets best to the cost of a seam through each cell of line i
func (p *seamPass) costs(i int, best []int) {
	// the length of the run of equal need around every cell that has one;
	// those aren't plain blanks, so they are all before the cells costed
	line := p.g[i][:len(best)]
	for x := 0; x < len(line); {
		if line[x].need == 0 {
			x++
			continue
		}
		k := x + 1
		for k < len(line) && line[k].need == line[x].need {
			k++
		}
		for q := x; q < k; q++ {
			p.runs[q] = k - x
		}
		x = k
	}
	ax, stubs, across, along, runs := p.opts.lines, p.opts.stubs, p.across, p.along, p.runs[:len(best)]
	lo, hi := ax.lo, ax.hi
	end := p.ends[i]
	// past the end, a seam with jogs has to narrow the grid
	past := seamTrailing
	if p.jogging && end == p.width {
		past = seamBlocked
	}
	line = p.g[i]
	text := func(p cell) bool { return p.keep && !p.solid }
	for x := range best {
		c := &line[x]
		if c.keep || c.kind&ax.seam == 0 {
			best[x] = seamBlocked
			continue
		}
		// a run kept for a label stays long enough for it
		if c.need > 0 && runs[x] <= int(c.need) {
			best[x] = seamBlocked
			continue
		}
		if x >= end {
			best[x] = past
			continue
		}
		// with stubs, last, blanks keep one of a run before what
		// follows, and none around the text of labels
		if stubs && c.r == ' ' && c.bg == 0 && x > 0 && x+1 < len(line) {
			if after := line[x+1]; after.r == ' ' && after.bg == 0 || text(after) || text(line[x-1]) {
				best[x] = 0
				continue
			}
		}
		// a line that turns right after leaving what it joins needs no
		// cell between, unless it leaves a node for an arrowhead; and a
		// node needs no blank under it where what follows is blank or
		// drawn by an edge, see leaves
		if across && x > 0 && x+1 < len(line) {
			before, after := line[x-1], line[x+1]
			arrow := after.kind&ax.mark != 0
			if c.r == along && before.kind&hi != 0 && (after.kind&lo != 0 || arrow) && !(arrow && before.solid) &&
				// or, with stubs, that goes on straight, which keeps a
				// cell of it
				(arrow || after.kind&(up|down|left|right) != lo|hi || stubs && after.r == c.r && after.bg == c.bg) {
				best[x] = 0
				continue
			}
			if c.r == ' ' && before.solid && !after.solid && !arrow && leaves(p.g, i, x) {
				best[x] = 0
				continue
			}
		}
		// a marker on a run continues it like the line it sits on
		if b := line[max(x-1, 0)]; x < 1 || !(c.r == b.r && c.bg == b.bg || c.r != ' ' && b.kind&ax.mark != 0) {
			best[x] = seamBlocked
			continue
		}
		best[x] = 0
	}
}

// crossing is the cost of a seam crossing the line at x between lines
// i-1 and i, moving toward hi when toHi is set
func (p *seamPass) crossing(i, x int, toHi bool) int {
	if !p.jogging || p.forbid[[2]int{i, x}] {
		return seamBlocked
	}
	best := seamBlocked
	bends, k := bendsAt(p.g, i, x, toHi, p.opts.lines)
	for _, b := range bends[:k] {
		if b.extends {
			best = min(best, seamExtend)
		} else {
			best = min(best, seamJog)
		}
	}
	return best
}

// joins sets joined to whether each cell of above joins the one below it
// in here: the cell above has the arm next, or the one below the arm
// prev, or one is kept or glued and the other is too or is not blank
func joins(above, here []cell, joined []bool, ax seamAxis) {
	next, prev := ax.next, ax.prev
	above, here = above[:len(joined)], here[:len(joined)]
	for x := range joined {
		a, b := &above[x], &here[x]
		joined[x] = a.kind&next != 0 || b.kind&prev != 0 ||
			(a.keep || a.glue) && (b.keep || b.glue || b.r != ' ') || (b.keep || b.glue) && a.r != ' '
	}
}

// sweep adds to best the cheapest way to each cell of line i from the
// line before, whose costs are in before, moving one cell at a time and
// past a joined cell only where it can jog, and sets from to where each
// comes from; reach and src are room for the sweeps. Crossings cost
// more than nothing, so one is only worth pricing where the move would
// win without it.
func (p *seamPass) sweep(i int, before, best, from, reach, src []int, joined []bool) {
	// all as long as joined, which spares the bounds checks
	n := len(joined)
	before, best, from, reach, src = before[:n], best[:n], from[:n], reach[:n], src[:n]
	// the sweeps keep the cell before in last, which spares reading back
	// what they just stored
	var last, lastSrc int
	for x := range joined {
		cur, curSrc := before[x], x
		if x > 0 {
			if r := before[x-1] + 1; r < cur {
				cur, curSrc = r, x-1
			}
			r := last + 1
			if joined[x-1] && r < cur {
				r += p.crossing(i, x-1, true)
			}
			if r < cur {
				cur, curSrc = r, lastSrc
			}
		}
		reach[x], src[x] = cur, curSrc
		last, lastSrc = cur, curSrc
	}
	for x := n - 2; x >= 0; x-- {
		cur, curSrc := reach[x], src[x]
		if r := before[x+1] + 1; r < cur {
			cur, curSrc = r, x+1
		}
		r := last + 1
		if joined[x+1] && r < cur {
			r += p.crossing(i, x+1, false)
		}
		if r < cur {
			cur, curSrc = r, lastSrc
		}
		reach[x], src[x] = cur, curSrc
		last, lastSrc = cur, curSrc
	}
	for x := range joined {
		best[x] += reach[x]
		from[x] = src[x]
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
	lines, join uint8
	extends     bool // the line turned at at, and now runs one cell further
}

// bendsAt returns the ways that the line at x between lines i-1 and i of
// grid can bend when a seam crosses it toward hi, when toHi is set, or
// toward lo. Either the cell of the line before or the one of the line
// after takes the turn, toward a blank cell beside it.
func bendsAt(g grid, i, x int, toHi bool, ax seamAxis) (out [2]bend, k int) {
	next, prev, lo, hi := ax.next, ax.prev, ax.lo, ax.hi
	a, b := g[i-1], g[i]
	plain := func(c cell) bool { return !c.keep && !c.glue && c.r != ' ' && corner(arms(c.r)) == c.r }
	if x == 0 || x+1 >= len(a) || !plain(a[x]) || !plain(b[x]) || arms(a[x].r)&next == 0 || arms(b[x].r)&prev == 0 {
		return out, 0
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
		return ax.straightRun(g[i][x])
	}
	add := func(row, at, to int, lines, join uint8, near, other int) {
		extends := lines == lo|hi
		if bits.OnesCount(uint(lines)) == 2 && (extends || straight(near) && straight(other)) {
			out[k] = bend{row, at, to, lines, join, extends}
			k++
		}
	}
	if free(a[x+step]) {
		add(i-1, pa, pb, arms(a[x].r)&^next|d, opposite(d)|next, i-2, i)
	}
	if free(b[x-step]) {
		add(i, pb, pa, arms(b[x].r)&^prev|opposite(d), d|prev, i+1, i-1)
	}
	return out, k
}

// bendLines bends the lines that the seam through path crosses between
// two lines of grid, in cut, which is grid with the seam removed; bends
// that run lines further go first. It returns the number of jogs, or the
// line and cell of a crossing that has no room to bend.
func bendLines(g, cut grid, path []int, ax seamAxis) (jogs int, failed [2]int, ok bool) {
	next, prev := ax.next, ax.prev
	// has reports whether the cell at x of line i has the arm, with lines
	// past the grid having none
	has := func(i, x int, arm uint8) bool {
		return i >= 0 && i < len(cut) && x >= 0 && x < len(cut[i]) && arms(cut[i][x].r)&arm != 0
	}
	for i := 1; i < len(path); i++ {
		xa, xb := path[i-1], path[i]
		for x := min(xa, xb) + 1; x < max(xa, xb); x++ {
			a, b := g[i-1][x], g[i][x]
			if arms(a.r)&next == 0 && arms(b.r)&prev == 0 {
				continue
			}
			bends, k := bendsAt(g, i, x, xb > xa, ax)
			if k == 2 && bends[1].extends && !bends[0].extends {
				bends[0], bends[1] = bends[1], bends[0]
			}
			done := false
			for _, bd := range bends[:k] {
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
				if !bd.extends && (near < 0 || near >= len(cut) || !ax.straightRun(cut[near][bd.at]) || !ax.straightRun(cut[other][bd.to])) {
					continue
				}
				// a line along a box stays of the box
				like := cut[bd.row][bd.at]
				id := ownerOf(like)
				owner := [4]edgeID{id, id, id, id}
				cut[bd.row][bd.at] = cell{r: corner(bd.lines), fg: like.fg, bg: like.bg, lines: bd.lines, solid: like.solid, node: like.node, owner: owner, kind: class(corner(bd.lines))}
				cut[bd.row][bd.to] = cell{r: corner(bd.join), fg: like.fg, bg: like.bg, lines: bd.join, solid: like.solid, node: like.node, owner: owner, kind: class(corner(bd.join))}
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
func ownerOf(c cell) edgeID {
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
