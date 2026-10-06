package text

import (
	"slices"
)

// line direction bits of a cell
const (
	up = 1 << iota
	down
	left
	right
)

// glyphs holds the line character for every combination of arm weights
// (0 none, 1 light, 2 heavy), indexed by up*27 + down*9 + left*3 + right.
var glyphs = []rune(" ╶╺╴─╼╸╾━╷┌┍┐┬┮┑┭┯╻┎┏┒┰┲┓┱┳╵└┕┘┴┶┙┵┷│├┝┤┼┾┥┽┿╽┟┢┧╁╆┪╅╈╹┖┗┚┸┺┛┹┻╿┞┡┦╀╄┩╃╇┃┠┣┨╂╊┫╉╋")

var rounded = map[rune]rune{'┌': '╭', '┐': '╮', '└': '╰', '┘': '╯'}

// glyph returns the line character with the arms in lines, drawing the
// arms in the mask heavy as heavy lines. A lone arm is drawn as a full
// straight line.
func glyph(lines, heavy uint8) rune {
	if lines&(lines-1) == 0 {
		lines |= opposite(lines)
		if heavy != 0 {
			heavy = lines
		}
	}
	i := 0
	for _, arm := range []uint8{up, down, left, right} {
		w := 0
		if lines&arm != 0 {
			w = 1
		}
		if heavy&arm != 0 {
			w = 2
		}
		i = i*3 + w
	}
	return glyphs[i]
}

// arrowheads and vees by direction
var (
	arrow = [right + 1]rune{up: '▲', down: '▼', left: '◀', right: '▶'}
	vee   = [right + 1]rune{up: '↑', down: '↓', left: '←', right: '→'}
)

func opposite(dir uint8) uint8 {
	return [right + 1]uint8{up: down, down: up, left: right, right: left}[dir]
}
func dx(dir uint8) int { return [right + 1]int{left: -1, right: 1}[dir] }
func dy(dir uint8) int { return [right + 1]int{up: -1, down: 1}[dir] }
func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// corner returns the line character with the arms in lines, with rounded
// corners as edges are drawn
func corner(lines uint8) rune {
	r := glyph(lines, 0)
	if c, ok := rounded[r]; ok {
		return c
	}
	return r
}

// arms returns the directions that the line character r joins toward;
// marks and dots join every way
func arms(r rune) uint8 {
	if i := uint32(r - armsFirst); i < uint32(len(armsTable)) {
		return armsTable[i]
	}
	return 0
}

// armsTable holds arms from the arrows to the end of the shapes, past the
// box drawing characters, where every character with arms is; a table
// keeps arms small enough to inline, as carving asks for every cell
const armsFirst = '\u2190'

var armsTable = func() (table [0x2600 - armsFirst]uint8) {
	for i := range table {
		table[i] = lookupArms(armsFirst + rune(i))
	}
	return table
}()

// lookupArms returns arms for any character, see arms
func lookupArms(r rune) uint8 {
	if i := slices.Index(glyphs, r); i > 0 {
		mask := uint8(0)
		for _, arm := range []uint8{right, left, down, up} {
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

// verticalSeam reports whether r is a blank or a straight vertical line,
// which a seam can take out of a column
func verticalSeam(r rune) bool {
	// blanks first, which most cells are
	if r == ' ' {
		return true
	}
	switch r {
	case '│', '┃', '┊', '┋', '┆':
		return true
	}
	return false
}

// horizontalSeam reports whether r is a blank or a straight horizontal
// line, which a seam can take out of a row
func horizontalSeam(r rune) bool {
	// blanks first, which most cells are
	if r == ' ' {
		return true
	}
	switch r {
	case '─', '━', '┈', '┉', '┄':
		return true
	}
	return false
}

// verticalMarker reports whether r is a marker that continues a vertical
// line it sits on
func verticalMarker(r rune) bool {
	switch r {
	case '▲', '▼', '●', '○':
		return true
	}
	return false
}

// horizontalMarker reports whether r is a marker that continues a
// horizontal line it sits on
func horizontalMarker(r rune) bool {
	switch r {
	case '◀', '▶', '●', '○':
		return true
	}
	return false
}

// dashedHorizontal reports whether r is a dashed horizontal line, as the
// frame of a cluster draws
func dashedHorizontal(r rune) bool { return r == '┈' || r == '┉' }

// boxRowSide reports whether r is part of the top or bottom side of a
// box: a horizontal line, or where a line joins it
func boxRowSide(r rune) bool {
	switch r {
	case '─', '━', '═', '┬', '┴', '╤', '╥':
		return true
	}
	return false
}

// roundedCorner reports whether r is a rounded corner, which edges turn
// with
func roundedCorner(r rune) bool {
	switch r {
	case '╭', '╮', '╰', '╯':
		return true
	}
	return false
}

// junction reports whether r is where three or four lines meet
func junction(r rune) bool {
	switch r {
	case '├', '┤', '┬', '┴', '┼':
		return true
	}
	return false
}

// verticalOrCrossing reports whether r is a vertical line, or one that a
// horizontal line crosses
func verticalOrCrossing(r rune) bool { return r == '│' || r == '╂' }

// horizontalOrCrossing reports whether r is a horizontal line, or where it
// crosses a vertical line
func horizontalOrCrossing(r rune) bool { return r == '─' || r == '╂' }
