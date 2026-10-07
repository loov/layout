// Command dropcarve compares carving text drawings with compacting their
// layouts, see layout.Options.NoCompact, over testdata/graphviz/*.gv and
// the examples (named ex_<name>):
//
//	a: the layout drawn as text.Write draws it, carved
//	b: the compacted layout, drawn without carving, see text.Options.NoCarve
//	c: the compacted layout, carved
//
// It prints the size of each drawing in columns × rows, the drawn stats of
// a and b, the diagnostics of the layout before and after compacting, and
// a summary of b and c against a. With -out it writes the drawings of a,
// b and c to <out>/a, <out>/b and <out>/c, to compare with
// go run ./internal/cmd/testdiff -all <out>/a <out>/b.
//
// Run it from the root of the module.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/text"
	"github.com/loov/layout/internal/draw"
	"github.com/loov/layout/internal/examples"
)

// labelled are the graphs where carving puts labels where rules.md
// says they are ambiguous, which the summary also leaves out
var labelled = []string{"arrows", "ex_narrow_labels"}

func main() {
	out := flag.String("out", "", "write the drawings to this directory")
	tidy := flag.Bool("tidy", true, "tidy b, see text.Options.Tidy")
	verbose := flag.Bool("v", false, "print the stats and diagnostics of every graph")
	flag.Parse()

	type input struct {
		name  string
		graph func() *layout.Graph
		opts  layout.Options
	}
	var inputs []input
	files, _ := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	for _, file := range files {
		inputs = append(inputs, input{strings.TrimSuffix(filepath.Base(file), ".gv"), func() *layout.Graph {
			graphs, err := dot.ParseFile(file)
			check(err)
			return graphs[0]
		}, layout.Options{}})
	}
	names := make([]string, 0, len(examples.Graphs))
	for name := range examples.Graphs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		inputs = append(inputs, input{"ex_" + name, examples.Graphs[name], examples.Options[name]})
	}
	if *out != "" {
		for _, d := range []string{"a", "b", "c"} {
			check(os.MkdirAll(filepath.Join(*out, d), 0o755))
		}
	}

	type result struct {
		name    string
		a, b, c int
		larger  string
	}
	var results []result
	drawn := map[string][3]float64{}
	var worse []string
	for _, in := range inputs {
		opts := in.opts
		opts.ForText, opts.NoCompact = true, true
		l, err := layout.Hierarchical(in.graph(), opts)
		check(err)
		opts.NoCompact = false
		cl, err := layout.Hierarchical(in.graph(), opts)
		check(err)
		da, db := layout.Diagnose(l), layout.Diagnose(cl)

		a := draw1(l, text.Options{})
		b := draw1(cl, text.Options{NoCarve: true, Tidy: *tidy})
		c := draw1(cl, text.Options{})
		wa, ha := size(a)
		wb, hb := size(b)
		wc, hc := size(c)
		r := result{name: in.name, a: wa * ha, b: wb * hb, c: wc * hc}
		if r.b > r.a {
			switch {
			case wb > wa && hb > ha:
				r.larger = "both"
			case wb > wa:
				r.larger = "columns"
			default:
				r.larger = "rows"
			}
		}
		results = append(results, r)
		fmt.Printf("%-24s a %4d×%-4d b %4d×%-4d (%3.0f%%) c %4d×%-4d (%3.0f%%)\n",
			in.name, wa, ha, wb, hb, pct(r.b, r.a), wc, hc, pct(r.c, r.a))
		sa, sb, sc := draw.TextStats(string(a)), draw.TextStats(string(b)), draw.TextStats(string(c))
		if *verbose {
			fmt.Printf("    drawn a %v\n    drawn b %v\n    drawn c %v\n", stats(sa), stats(sb), stats(sc))
			fmt.Printf("    before %v\n    after  %v\n", short(da), short(db))
		}
		for k := range sa {
			drawn[k] = [3]float64{drawn[k][0] + sa[k], drawn[k][1] + sb[k], drawn[k][2] + sc[k]}
		}
		if db.EdgeCrossings > da.EdgeCrossings || db.EdgeThroughNode > da.EdgeThroughNode || db.NodeOverlaps > da.NodeOverlaps ||
			db.EdgeOverlaps > da.EdgeOverlaps || db.LabelOverlaps > da.LabelOverlaps {
			worse = append(worse, in.name)
		}
		if *out != "" {
			for d, drawing := range map[string][]byte{"a": a, "b": b, "c": c} {
				check(os.WriteFile(filepath.Join(*out, d, in.name+".txt"), drawing, 0o644))
			}
		}
	}

	summary := func(title string, skip []string) {
		fmt.Println(title)
		var sum struct{ a, b, c int }
		var rb, rc []float64
		var larger []string
		for _, r := range results {
			if slices.Contains(skip, r.name) {
				continue
			}
			sum.a += r.a
			sum.b += r.b
			sum.c += r.c
			rb = append(rb, float64(r.b)/float64(r.a))
			rc = append(rc, float64(r.c)/float64(r.a))
			if r.b > r.a {
				larger = append(larger, fmt.Sprintf("%s %.0f%% (%s)", r.name, pct(r.b, r.a), r.larger))
			}
		}
		for _, x := range []struct {
			name   string
			sum    int
			ratios []float64
		}{{"b", sum.b, rb}, {"c", sum.c, rc}} {
			smaller, equal := 0, 0
			for _, r := range x.ratios {
				switch {
				case r < 1:
					smaller++
				case r == 1:
					equal++
				}
			}
			slices.Sort(x.ratios)
			n := len(x.ratios)
			fmt.Printf("  %s/a: total %.1f%%, median %.3f; smaller %d, equal %d, larger %d of %d; not larger %.1f%%\n",
				x.name, pct(x.sum, sum.a), x.ratios[n/2], smaller, equal, n-smaller-equal, n, pct(smaller+equal, n))
		}
		fmt.Printf("  b larger: %s\n", strings.Join(larger, ", "))
	}
	summary("all graphs", nil)
	summary("without "+strings.Join(labelled, ", "), labelled)
	for _, k := range []string{"corners", "crossings", "length"} {
		fmt.Printf("drawn %s: a %.0f b %.0f c %.0f\n", k, drawn[k][0], drawn[k][1], drawn[k][2])
	}
	fmt.Printf("diagnostics worse after compacting: %v\n", worse)
}

// draw1 draws l as text
func draw1(l *layout.Layout, opts text.Options) []byte {
	var buf bytes.Buffer
	check(text.WriteOptions(&buf, l, opts))
	return buf.Bytes()
}

// size returns the columns and rows a drawing takes
func size(b []byte) (w, h int) {
	lines := strings.Split(strings.TrimRight(string(b), "\n "), "\n")
	for _, line := range lines {
		w = max(w, draw.Columns(strings.TrimRight(line, " ")))
	}
	return w, len(lines)
}

// stats formats the stats of a drawing, see draw.TextStats
func stats(s map[string]float64) string {
	return fmt.Sprintf("corners=%.0f length=%.0f crossings=%.0f", s["corners"], s["length"], s["crossings"])
}

func pct(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }

// short formats the diagnostics the comparison is about
func short(d layout.Diagnostics) string {
	return fmt.Sprintf("crossings=%d through=%d nodes=%d overlaps=%d labels=%d far=%d near=%d corners=%d length=%.0f",
		d.EdgeCrossings, d.EdgeThroughNode, d.NodeOverlaps, d.EdgeOverlaps, d.LabelOverlaps, d.FarLabels, d.EdgeNearNode, d.Corners, d.EdgeLength)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
