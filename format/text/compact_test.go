package text

import (
	"fmt"
	"strings"
	"testing"

	"github.com/loov/layout"
)

// TestCompactedRandom checks that compacted layouts of random graphs, see
// randomGraph, still draw every edge joined and apart, drawn as they are
// and tidied: compaction relies on how text draws.
func TestCompactedRandom(t *testing.T) {
	for seed := range 300 {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			checkCompacted(t, randomGraph(uint64(seed)))
		})
	}
}

func checkCompacted(t *testing.T, graph *layout.Graph) {
	t.Helper()
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	d := drawGraph(l, true, false)
	c, g := drawn(l, false)
	for _, drawing := range []struct {
		c *canvas
		g grid
	}{{d, d.rows}, {c, g}} {
		for _, edge := range lost(drawing.c, drawing.g, l) {
			t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(drawing.g, drawing.c.palette, nil))
		}
		checkApart(t, drawing.c, drawing.g)
		checkAttached(t, encode(drawing.g, drawing.c.palette, nil))
	}
}

// checkAttached checks that no arrowhead comes straight out of a line
// that runs across it, as it does from a border without a tee: text
// draws a run from box to box that has no room for a line that way
func checkAttached(t *testing.T, drawing string) {
	t.Helper()
	lines := strings.Split(drawing, "\n")
	at := func(y, x int) rune {
		if y < 0 || y >= len(lines) {
			return ' '
		}
		row := []rune(lines[y])
		if x < 0 || x >= len(row) {
			return ' '
		}
		return row[x]
	}
	for y, line := range lines {
		for x, r := range []rune(line) {
			var from rune
			across := "─═━"
			switch r {
			case '▼':
				from = at(y-1, x)
			case '▲':
				from = at(y+1, x)
			case '▶':
				from, across = at(y, x-1), "│║┃"
			case '◀':
				from, across = at(y, x+1), "│║┃"
			}
			if from != 0 && strings.ContainsRune(across, from) {
				t.Errorf("%d:%d: arrowhead out of %q:\n%s", y+1, x+1, from, drawing)
				return
			}
		}
	}
}
