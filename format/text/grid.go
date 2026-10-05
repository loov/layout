package text

import (
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
}

// carve tightens the drawing along seams: it removes a cell from every
// column, and then from every row, as long as some seam can go through
// cells that only continue the straight lines across it or blanks, and
// repeat the cells before them. At most one such row and two such
// columns stay of every run, as cells are about twice as tall as wide.
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
	grid = seams(grid, " ─━┈┉┄", "◀▶●○", 2, !sideways, true, false, down, up)
	return rows(grid, true)
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
	// the directions along the lines, toward the start and the end
	lo, hi := left, right
	if next&(left|right) != 0 {
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
				// a line that turns right after leaving what it joins
				// needs no cell between, unless an arrowhead follows; and
				// a node needs no blank under it where what follows is
				// blank or drawn by an edge that leaves the node there
				if keep == 1 && x > 0 && x+1 < len(line) && !strings.ContainsRune(markers, line[x+1].r) {
					// or, with stubs, that goes on straight, which keeps a
					// cell of it
					if c.r == glyph(lo|hi, 0) && arms(line[x-1].r)&hi != 0 && arms(line[x+1].r)&lo != 0 &&
						(arms(line[x+1].r) != lo|hi || stubs && line[x+1].r == c.r && line[x+1].bg == c.bg) {
						return 0
					}
					if c.r == ' ' && line[x-1].solid && !line[x+1].solid && leaves(grid, i, x, lo) {
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
// cell before x, is blank or drawn by an edge that leaves the node: one
// that starts at a cell beside the node, toward lo, on a neighboring line
// along the node
func leaves(grid [][]cell, i, x, lo int) bool {
	after := grid[i][x+1]
	if after.r == ' ' {
		return after.bg == 0 && !after.keep && !after.glue
	}
	if after.lines == 0 {
		return false // text or a mark
	}
	back := bits.TrailingZeros(uint(lo))
	for _, step := range []int{-1, 1} {
		for j := i; j >= 0 && j < len(grid) && x < len(grid[j]) && grid[j][x-1].solid; j += step {
			start := grid[j][x]
			if start.lines&lo == 0 {
				continue
			}
			for arm := range 4 {
				if after.lines&(1<<arm) != 0 && after.owner[arm] == start.owner[back] {
					return true
				}
			}
		}
	}
	return false
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
				like := cut[bd.row][bd.at]
				cut[bd.row][bd.at] = cell{r: corner(bd.lines), fg: like.fg, bg: like.bg, lines: bd.lines}
				cut[bd.row][bd.to] = cell{r: corner(bd.join), fg: like.fg, bg: like.bg, lines: bd.join}
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
