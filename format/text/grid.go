package text

import "strings"

// cell is a drawn character with its colors, see rgb, and what is
// needed to join the lines drawn through it
type cell struct {
	r      rune
	fg, bg uint32
	keep   bool   // inside a node or of text, which carving keeps
	solid  bool   // covered by a node; edges do not draw there
	lines  int    // direction mask, for joining edge runs
	heavy  int    // arms that runs of different edges share
	owner  [4]int // edge that first drew each arm
}

// carve removes rows and then columns that only continue straight lines
// or blanks and repeat the one before them, keeping at most one such row
// and two such columns of every run, as cells are about twice as tall as
// wide. Removing them keeps the drawing connected, just tighter. Rows and
// columns through nodes or text stay, so that their blanks keep their
// size.
func carve(grid [][]cell) [][]cell {
	w := 0
	if len(grid) > 0 {
		w = len(grid[0])
	}
	rows := carved(len(grid), w, func(i, j int) cell { return grid[i][j] }, " │┃┊┋┆", "▲▼●○", 1)
	var out [][]cell
	for i, row := range grid {
		if rows[i] {
			out = append(out, row)
		}
	}
	cols := carved(w, len(out), func(i, j int) cell { return out[j][i] }, " ─━┈┉┄", "◀▶●○", 2)
	for y, row := range out {
		kept := row[:0]
		for x, c := range row {
			if cols[x] {
				kept = append(kept, c)
			}
		}
		out[y] = kept
	}
	return out
}

// carved reports which of n lines of m cells to keep, with at returning
// cell j of line i: a line that only continues the straight lines across
// it or blanks and repeats the line before it is kept only for the first
// keep of every run of such lines
func carved(n, m int, at func(i, j int) cell, straight, markers string, keep int) []bool {
	kept := make([]bool, n)
	run := 0
	for i := range n {
		repeat := i > 0
		for j := 0; j < m && repeat; j++ {
			x, prev := at(i, j), at(i-1, j)
			// a marker on a run continues it like the line it sits on
			repeat = !x.keep && strings.ContainsRune(straight, x.r) &&
				(x.r == prev.r || x.r != ' ' && strings.ContainsRune(markers, prev.r))
		}
		if repeat {
			run++
		} else {
			run = 0
		}
		kept[i] = run < keep
	}
	return kept
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
