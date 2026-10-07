package text

import (
	"math/rand/v2"
	"testing"
)

// TestSweepBruteForce compares the cost of the cheapest seam that the
// sweeps find, and of the seam they trace back, with the cheapest seam
// among all of them on small random grids, half of them jogging, for the
// seams that are cheap enough to cut.
func TestSweepBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	glyphs := []rune{' ', ' ', '─', '│', '╭', '╯', '┼', 'x'}
	for iter := range 2000 {
		lines, n := 1+rng.IntN(4), 1+rng.IntN(6)
		g := make(grid, lines)
		codes := make([][]costCode, lines)
		for i := range g {
			// a blank past the cells seams consider, as seams adds one
			g[i] = make([]cell, n+1)
			codes[i] = make([]costCode, n)
			for x := range g[i] {
				c := cell{r: glyphs[rng.IntN(len(glyphs))]}
				c.keep = c.r == 'x'
				if x == n {
					c = cell{r: ' '}
				}
				c.kind = class(c.r)
				g[i][x] = c
			}
			for x := range codes[i] {
				codes[i][x] = costCode(rng.IntN(3))
			}
		}
		p := &seamPass{g: g, opts: seamOpts{lines: rowLines, turn: true, bend: true}, forbid: map[[2]int]bool{}}
		p.jogging = iter%2 == 0
		joined := make([][]bool, lines)
		for i := 1; i < lines; i++ {
			joined[i] = make([]bool, n)
			joinCells(g[i-1], g[i], joined[i], 0, n, rowLines)
		}

		best, before := make([]int, n), make([]int, n)
		reach, src := make([]int, n), make([]int, n)
		from := make([][]int, lines)
		for x := range best {
			best[x] = costOf[codes[0][x]]
		}
		for i := 1; i < lines; i++ {
			copy(before, best)
			from[i] = make([]int, n)
			p.sweep(i, before, best, codes[i], from[i], reach, src, joined[i])
		}
		end := 0
		for x := range best {
			if best[x] < best[end] {
				end = x
			}
		}
		path := make([]int, lines)
		for i, x := lines-1, end; i >= 0; i-- {
			path[i] = x
			if i > 0 {
				x = from[i][x]
			}
		}

		// the cost of a seam: its cells, a cell moved along, and the
		// crossings of the lines it moves past; moving off a joined cell
		// or onto one is not allowed
		cost := func(path []int) (int, bool) {
			total := costOf[codes[0][path[0]]]
			for i := 1; i < lines; i++ {
				a, b := path[i-1], path[i]
				total += costOf[codes[i][b]]
				if a == b {
					continue
				}
				if joined[i][a] || joined[i][b] {
					return 0, false
				}
				total += max(a-b, b-a)
				for x := min(a, b) + 1; x < max(a, b); x++ {
					if joined[i][x] {
						total += p.crossing(i, x, b > a)
					}
				}
			}
			return total, true
		}
		got, ok := cost(path)
		if !ok || got != best[end] {
			t.Fatalf("iter %d: seam %v costs %v (allowed %v), sweeps say %v", iter, path, got, ok, best[end])
		}
		cheapest := -1
		try := make([]int, lines)
		var enumerate func(i int)
		enumerate = func(i int) {
			if i == lines {
				if c, ok := cost(try); ok && (cheapest < 0 || c < cheapest) {
					cheapest = c
				}
				return
			}
			for x := range n {
				try[i] = x
				enumerate(i + 1)
			}
		}
		enumerate(0)
		// a seam through a blocked cell is never cut, so the sweeps drop
		// the ways that cost that much
		if cheapest < seamBlocked && got != cheapest || got < seamBlocked && cheapest >= seamBlocked {
			t.Fatalf("iter %d: seam %v costs %v, cheapest %v", iter, path, got, cheapest)
		}
	}
}
