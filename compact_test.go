package layout_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/examples"
)

// TestCompact checks that compacting the examples and the Graphviz files,
// laid out for text and with ortho splines, with edges merged and not,
// takes no more room and adds no crossings nor overlaps, and leaves the
// ends of edges on their nodes' outlines.
func TestCompact(t *testing.T) {
	check := func(t *testing.T, build func() *layout.Graph, opts layout.Options) {
		for _, text := range []bool{true, false} {
			for _, merge := range []bool{false, true} {
				graph := build()
				graph.Splines = layout.SplinesOrtho
				graph.MergeEdges = merge
				opts := opts
				opts.ForText = text
				l, err := layout.Hierarchical(graph, opts)
				if err != nil {
					t.Fatal(err)
				}
				opts.Compact = true
				c, err := layout.Hierarchical(graph, opts)
				if err != nil {
					t.Fatal(err)
				}
				before, after := layout.Diagnose(l), layout.Diagnose(c)
				if after.Area > before.Area {
					t.Errorf("text %v, merged %v: area grew from %v to %v", text, merge, before.Area, after.Area)
				}
				for _, k := range []struct {
					name          string
					before, after int
				}{
					{"node overlaps", before.NodeOverlaps, after.NodeOverlaps},
					{"edges through nodes", before.EdgeThroughNode, after.EdgeThroughNode},
					{"crossings", before.EdgeCrossings, after.EdgeCrossings},
					{"edge overlaps", before.EdgeOverlaps, after.EdgeOverlaps},
					{"label overlaps", before.LabelOverlaps, after.LabelOverlaps},
					{"ends inside their nodes", before.Shafts, after.Shafts},
				} {
					if k.after > k.before {
						t.Errorf("text %v, merged %v: %s grew from %d to %d", text, merge, k.name, k.before, k.after)
					}
				}
			}
		}
	}
	for name, build := range examples.Graphs {
		t.Run(name, func(t *testing.T) { check(t, build, examples.Options[name]) })
	}
	files, _ := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".gv"), func(t *testing.T) {
			check(t, func() *layout.Graph {
				graphs, err := dot.ParseFile(file)
				if err != nil {
					t.Fatal(err)
				}
				return graphs[0]
			}, layout.Options{})
		})
	}
}
