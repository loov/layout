package layout_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/svg"
)

// TestDeterministic checks that laying out the same input twice, from
// freshly parsed graphs, gives byte-identical output.
func TestDeterministic(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	render := func(file string) []byte {
		graphs, err := dot.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := layout.Hierarchical(graphs[0]); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := svg.Write(&out, graphs[0]); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	for _, file := range files {
		if !bytes.Equal(render(file), render(file)) {
			t.Errorf("%s: layout is not deterministic", file)
		}
	}
}
