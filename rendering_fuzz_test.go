package layout_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
)

// FuzzRendering checks crashes through both layout algorithms and renderers.
// Graphs and labels are bounded so malformed text cannot exhaust the canvas.
func FuzzRendering(f *testing.F) {
	for mode := range 4 {
		f.Add([]byte{byte(mode), 3, 0, 1, 1, 2, 2, 0, 0, 0}, "漢字 é 👩‍💻\nsecond")
		f.Add([]byte{byte(mode), 2, 5, 4, 0, 1, 1, 0}, "<TABLE><TR><TD>cell</TD></TR></TABLE>")
	}
	f.Fuzz(func(t *testing.T, data []byte, label string) {
		if len(data) < 2 {
			return
		}
		data = data[:min(len(data), 128)]
		label = label[:min(len(label), 128)]
		graph := layout.NewDigraph()
		graph.RankDir = []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft}[int(data[0]>>2)%4]
		shapes := []layout.Shape{layout.Box, layout.Ellipse, layout.Circle, layout.None, layout.PointShape, layout.Record}
		labels := []string{label, "first\nsecond", "<" + label + ">", "<p> " + label + "|{left|right}", "漢字", "é 👩‍💻"}
		ports := []layout.Compass{layout.CompassAuto, layout.North, layout.South, layout.East, layout.West}
		for i := range int(data[1])%8 + 1 {
			node := graph.Node(fmt.Sprint("n", i))
			b := data[(i+2)%len(data)]
			node.Shape = shapes[int(b)%len(shapes)]
			node.Label = labels[int(b>>3)%len(labels)]
			node.Peripheries = int(b>>6) + 1
		}
		for i := 2; i+1 < len(data) && i < 66; i += 2 {
			a, b := data[i], data[i+1]
			edge := graph.Edge(graph.Nodes[int(a)%len(graph.Nodes)].ID, graph.Nodes[int(b)%len(graph.Nodes)].ID)
			edge.FromPort = ports[int(a>>4)%len(ports)]
			edge.ToPort = ports[int(b>>4)%len(ports)]
			if a&0x80 != 0 {
				edge.Label = labels[int(b>>3)%len(labels)]
			}
		}
		if data[0]&2 != 0 {
			graph.ForText = true
		}
		lay := layout.Hierarchical
		if data[0]&1 != 0 {
			lay = layout.Force
		}
		if err := lay(graph); err != nil {
			t.Fatal(err)
		}
		write := svg.Write
		if data[0]&2 != 0 {
			write = text.Write
		}
		if err := write(io.Discard, graph); err != nil {
			t.Fatal(err)
		}
	})
}
