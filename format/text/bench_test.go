package text

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
)

// BenchmarkWrite draws Graphviz examples laid out for text, from small to
// large; the layout is done once, outside the timing.
func BenchmarkWrite(b *testing.B) {
	for _, name := range []string{"unix", "ninja", "siblings"} {
		b.Run(name, func(b *testing.B) {
			graphs, err := dot.ParseFile(filepath.Join("..", "..", "testdata", "graphviz", name+".gv"))
			if err != nil {
				b.Fatal(err)
			}
			l, err := layout.Hierarchical(graphs[0], layout.Options{ForText: true})
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := Write(io.Discard, l); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
