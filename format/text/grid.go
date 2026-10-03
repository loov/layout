package text

import (
	"slices"
	"strings"
)

// cell is a drawn character with its colors, see rgb
type cell struct {
	r      rune
	fg, bg uint32
	keep   bool // inside a node or of text, see canvas.keep
}

// grid returns the drawn cells by row
func (c *canvas) grid() [][]cell {
	grid := make([][]cell, c.h)
	for y := range grid {
		grid[y] = make([]cell, c.w)
		for x := range grid[y] {
			i := y*c.w + x
			grid[y][x] = cell{c.cells[i], c.fg[i], c.bg[i], c.keep[i]}
		}
	}
	return grid
}

// carve removes rows that only continue straight lines or blanks and
// repeat the row before them, keeping at most keep of every such run.
// Removing them keeps the drawing connected, just tighter. Rows through
// nodes or text stay, so that their blanks keep their size.
func carve(grid [][]cell, straight, markers string, keep int) [][]cell {
	out := grid[:0:0]
	run := 0
	for i, row := range grid {
		plain := true
		for _, x := range row {
			if x.keep || !strings.ContainsRune(straight, x.r) {
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
