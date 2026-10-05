package main

import (
	"strings"

	"github.com/loov/layout/internal/draw"
)

// textStats counts the edges of a text drawing as drawn, after carving:
// the bends of the edges, their length in cells and their crossings. The
// cells of boxes, closed rectangles of lines, are not edges; node boxes
// are not edges inside either, cluster frames, which are dashed, only on
// their frame.
func textStats(content string) map[string]float64 {
	var grid [][]rune
	for _, line := range strings.Split(escape.ReplaceAllString(content, ""), "\n") {
		var row []rune
		for _, r := range line {
			row = append(row, r)
			if draw.IsWide(r) {
				row = append(row, 0) // the second column of a wide rune
			}
		}
		grid = append(grid, row)
	}
	at := func(y, x int) rune {
		if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
			return ' '
		}
		return grid[y][x]
	}
	const (
		topLeft     = "╭┌╔┏"
		topRight    = "╮┐╗┓"
		bottomLeft  = "╰└╚┗"
		bottomRight = "╯┘╝┛"
		vertical    = "│┃║┊┋┆┇├┤┼╂┝┥┠┨┣┫╟╢╫╬"
		horizontal  = "─━═┈┉┄┅┬┴┼╂┯┷┰┸┳┻╤╧╥╨╪╬"
		dashed      = "┊┋┆┇┈┉┄┅"
		marks       = "▲▼◀▶↑↓←→●○"
	)
	in := func(set string, r rune) bool { return r != 0 && strings.ContainsRune(set, r) }

	box := map[[2]int]bool{}
	for y, row := range grid {
		for x, r := range row {
			if !in(topLeft, r) {
				continue
			}
			y2 := y + 1
			for in(vertical, at(y2, x)) {
				y2++
			}
			if !in(bottomLeft, at(y2, x)) {
				continue
			}
			x2 := x + 1
			for in(horizontal, at(y2, x2)) {
				x2++
			}
			if !in(bottomRight, at(y2, x2)) || !in(topRight, at(y, x2)) {
				continue
			}
			side := true
			for yy := y + 1; yy < y2 && side; yy++ {
				side = in(vertical, at(yy, x2))
			}
			if !side {
				continue
			}
			// the top is not checked, as cluster labels sit on it
			frame := in(dashed, at(y+1, x)) || in(dashed, at(y2, x+1))
			for yy := y; yy <= y2; yy++ {
				for xx := x; xx <= x2; xx++ {
					if !frame || yy == y || yy == y2 || xx == x || xx == x2 {
						box[[2]int{yy, xx}] = true
					}
				}
			}
		}
	}

	var bends, length, crossings float64
	for y, row := range grid {
		for x, r := range row {
			if box[[2]int{y, x}] || r == 0 {
				continue
			}
			line := r >= '─' && r <= '╿'
			if !line && !in(marks, r) {
				continue
			}
			length++
			if in(topLeft+topRight+bottomLeft+bottomRight, r) {
				bends++
			}
			if r == '╂' {
				crossings++
			}
		}
	}
	return map[string]float64{"corners": bends, "length": length, "crossings": crossings}
}
