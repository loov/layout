package layout_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
	"github.com/loov/layout/internal/examples"
)

var update = flag.Bool("update", false, "update testdata golden files")

// TestExamples renders each example and compares it to testdata/<name>.svg.
// Run `go test -update` to regenerate the golden files.
func TestExamples(t *testing.T) {
	for name, build := range examples.Graphs {
		if strings.HasSuffix(name, "_merged") {
			continue // merging needs ortho edges, which the svg doesn't use
		}
		t.Run(name, func(t *testing.T) {
			checkGolden(t, filepath.Join("testdata", name+".svg"), build(), examples.Options[name])
		})
	}
}

// TestExamplesText renders each example with ortho edges as text and
// compares it to testdata/<name>.txt, and colored to testdata/<name>.ans
// with the basic colors and testdata/<name>.truecolor.ans with 24-bit
// colors, when the colors make a difference.
func TestExamplesText(t *testing.T) {
	for name, build := range examples.Graphs {
		t.Run(name, func(t *testing.T) {
			opts := examples.Options[name]
			opts.ForText = true
			l, err := layout.Hierarchical(build(), opts)
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := text.Write(&got, l); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, filepath.Join("testdata", name+".txt"), got.Bytes())

			plain := bytes.Clone(got.Bytes())
			for suffix, palette := range map[string]text.Palette{".ans": text.ANSI16, ".truecolor.ans": text.TrueColor} {
				got.Reset()
				if err := text.WriteColor(&got, l, text.Options{Palette: palette}); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join("testdata", name+suffix)
				if !bytes.Equal(got.Bytes(), plain) {
					compareGolden(t, path, got.Bytes())
				} else if *update {
					_ = os.Remove(path)
				} else if _, err := os.Stat(path); err == nil {
					t.Errorf("%s is the same as the text output; run `go test -update`", path)
				}
			}
		})
	}
}

// TestGraphviz lays out the classic Graphviz directed examples in
// testdata/graphviz/*.gv and compares them to the .svg next to them, and
// laid out for text to the .txt, and with merged edges to the _merged.txt.
func TestGraphviz(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".gv"), func(t *testing.T) {
			graphs, err := dot.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if len(graphs) != 1 {
				t.Fatalf("expected one graph, got %d", len(graphs))
			}
			checkGolden(t, strings.TrimSuffix(file, ".gv")+".svg", graphs[0], layout.Options{})

			for suffix, merge := range map[string]bool{".txt": false, "_merged.txt": true} {
				graph := *graphs[0]
				graph.MergeEdges = merge
				l, err := layout.Hierarchical(&graph, layout.Options{ForText: true})
				if err != nil {
					t.Fatal(err)
				}
				var got bytes.Buffer
				if err := text.Write(&got, l); err != nil {
					t.Fatal(err)
				}
				compareGolden(t, strings.TrimSuffix(file, ".gv")+suffix, got.Bytes())
			}
		})
	}
}

// TestWriteDot checks the dot writer against testdata/minimal.dot.
func TestWriteDot(t *testing.T) {
	graph := examples.Graphs["minimal"]()
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := dot.Write(&got, l); err != nil {
		t.Fatal(err)
	}
	// round trip: the output must parse again with the same nodes and edges
	parsed, err := dot.Parse(bytes.NewReader(got.Bytes()))
	if err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, got.Bytes())
	}
	if len(parsed) != 1 || len(parsed[0].Nodes) != len(graph.Nodes) || len(parsed[0].Edges) != len(graph.Edges) {
		t.Fatalf("round trip mismatch\n%s", got.Bytes())
	}
	compareGolden(t, filepath.Join("testdata", "minimal.dot"), got.Bytes())
}

func checkGolden(t *testing.T, path string, graph *layout.Graph, opts layout.Options) {
	t.Helper()
	l, err := layout.Hierarchical(graph, opts)
	if err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	if err := svg.Write(&got, l); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, path, got.Bytes())
}

// compareGolden compares got with the file at path, rewriting it with -update
func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden file; run `go test -update`\n%s", path, diffLines(want, got))
	}
}

// TestDiagnostics records layout.Diagnose for every example and fixture in
// testdata/diagnostics.txt, and laid out for text, as the .txt drawings
// are, in testdata/diagnostics_text.txt, so that changes in quality show up
// in diffs.
func TestDiagnostics(t *testing.T) {
	names := make([]string, 0, len(examples.Graphs))
	for name := range examples.Graphs {
		names = append(names, name)
	}
	sort.Strings(names)
	files, _ := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	parse := func(file string) *layout.Graph {
		graphs, err := dot.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return graphs[0]
	}

	var out, text bytes.Buffer
	diagnose := func(out *bytes.Buffer, name string, graph *layout.Graph, opts layout.Options) {
		l, err := layout.Hierarchical(graph, opts)
		if err != nil {
			t.Fatal(err)
		}
		d := layout.Diagnose(l)
		fmt.Fprintf(out, "%-12s %v\n", name, d)
		for _, line := range d.Details {
			t.Logf("%s: %s", name, line)
		}
	}
	for _, name := range names {
		diagnose(&out, name, examples.Graphs[name](), examples.Options[name])
	}
	for _, file := range files {
		diagnose(&out, strings.TrimSuffix(filepath.Base(file), ".gv"), parse(file), layout.Options{})
	}
	compareGolden(t, filepath.Join("testdata", "diagnostics.txt"), out.Bytes())

	// as the text drawings: see TestExamplesText and TestGraphviz
	for _, name := range names {
		opts := examples.Options[name]
		opts.ForText = true
		diagnose(&text, name, examples.Graphs[name](), opts)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".gv")
		diagnose(&text, name, parse(file), layout.Options{ForText: true})
		merged := parse(file)
		merged.MergeEdges = true
		diagnose(&text, name+"_merged", merged, layout.Options{ForText: true})
	}
	compareGolden(t, filepath.Join("testdata", "diagnostics_text.txt"), text.Bytes())
}

// diffLines reports the mismatching lines between want and got.
func diffLines(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	var out strings.Builder
	for i := range max(len(wantLines), len(gotLines)) {
		w, g := "<missing>", "<missing>"
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			fmt.Fprintf(&out, "line %d:\n\twant: %s\n\tgot:  %s\n", i+1, w, g)
		}
	}
	return out.String()
}

// knownBad lists the text drawings that fail a check, by the check and
// the drawing's path in testdata, with what goes wrong; they are skipped
// until the bug is fixed. Remove an entry with its fix.
var knownBad = map[string]map[string]string{}

// eachDrawing runs check on every text drawing in testdata, as a subtest
// named by its path in testdata, skipping the known bad ones of the test
func eachDrawing(t *testing.T, check func(t *testing.T, file string, lines []string)) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	graphviz, err := filepath.Glob(filepath.Join("testdata", "graphviz", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range append(files, graphviz...) {
		if strings.HasPrefix(filepath.Base(file), "diagnostics") {
			continue
		}
		name := filepath.ToSlash(strings.TrimPrefix(file, "testdata"+string(filepath.Separator)))
		t.Run(name, func(t *testing.T) {
			if reason, ok := knownBad[strings.Split(t.Name(), "/")[0]][name]; ok {
				t.Skip("known bad: " + reason)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			check(t, file, strings.Split(string(data), "\n"))
		})
	}
}

// TestArrowsBesideNodes checks the text drawings for arrowheads drawn on
// the top or bottom border of a node, where an edge has no room to end
// before the node, and for arrowheads at a corner of a box, which read as
// reaching for the corner.
func TestArrowsBesideNodes(t *testing.T) {
	border := regexp.MustCompile(`[╭┌╔╰└╚][─═┬┴╥╨]*[▼▲][─═┬┴╥╨]*[╮┐╗╯┘╝]`)
	sideways := regexp.MustCompile(`▶[╭┌╔╰└╚]|[╮┐╗╯┘╝]◀`)
	const (
		top    = "╭┌╔╮┐╗"
		bottom = "╰└╚╯┘╝"
	)
	eachDrawing(t, func(t *testing.T, file string, lines []string) {
		for i, line := range lines {
			if m := border.FindString(line); m != "" {
				t.Errorf("%s:%d: arrow on a border: %s", file, i+1, m)
			}
			if m := sideways.FindString(line); m != "" {
				t.Errorf("%s:%d: arrow at a corner: %s", file, i+1, m)
			}
			for x, r := range []rune(line) {
				if r == '▼' && i+1 < len(lines) && strings.ContainsRune(top, at(lines[i+1], x)) ||
					r == '▲' && i > 0 && strings.ContainsRune(bottom, at(lines[i-1], x)) {
					t.Errorf("%s:%d: arrow at a corner, column %d", file, i+1, x+1)
				}
			}
		}
	})
}

// at returns the rune at column x of line, a space past its end
func at(line string, x int) rune {
	if r := []rune(line); x < len(r) {
		return r[x]
	}
	return ' '
}
