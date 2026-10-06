package layout

import "github.com/loov/layout/internal/draw"

// tableRecord makes a node with an HTML-like label that is a table with
// borders a record, which draws the borders as its dividers, see
// draw.TableRecord; the writers draw other HTML-like labels as they are
func tableRecord(graph *lgraph, node *lnode) {
	if rec, ok := draw.TableRecord(node.DefaultLabel(), sideways(graph.RankDir)); ok {
		node.Label, node.Shape = rec, Record
	}
}

// htmlLabelRadius estimates the half size of an HTML-like label from its
// text, see draw.PlainLabel, with room around it for the padding of cells
func (graph *Graph) htmlLabelRadius(label, fontName string, fontSize Length) Vector {
	r := graph.textRadius(draw.PlainLabel(label), fontName, fontSize)
	return r.Add(Vector{fontSize * 0.5, fontSize * 0.25})
}
