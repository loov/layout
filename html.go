package layout

import (
	"strings"

	"golang.org/x/net/html"
)

// htmlLabelRadius estimates the half size of an HTML-like label: tables
// sum their cell texts per row, everything else is lines split at <br>.
func (graph *Graph) htmlLabelRadius(label, fontName string, fontSize Length) Vector {
	root := &html.Node{Type: html.ElementNode}
	nodes, err := html.ParseFragment(strings.NewReader(label[1:len(label)-1]), root)
	if err != nil {
		return graph.textRadius(label, fontName, fontSize)
	}
	pad := fontSize * 0.5 // cell padding on each side
	var size func(n *html.Node) Vector
	size = func(n *html.Node) Vector {
		switch {
		case n.Type == html.TextNode:
			return graph.textRadius(n.Data, fontName, fontSize)
		case n.Type == html.ElementNode && n.Data == "br":
			return Vector{0, graph.textRadius("", fontName, fontSize).Y}
		case n.Type == html.ElementNode && n.Data == "tr":
			var row Vector
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				s := size(c)
				row.X += s.X + pad
				row.Y = max(row.Y, s.Y)
			}
			return row.Add(Vector{0, pad / 2})
		case n.Type == html.ElementNode && (n.Data == "table" || n.Data == "tbody"):
			var box Vector
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				s := size(c)
				box.X = max(box.X, s.X)
				box.Y += s.Y
			}
			return box
		}
		// inline content: text runs side by side, <br> starts a new line
		var box, line Vector
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "br" {
				box.X, box.Y = max(box.X, line.X), box.Y+max(line.Y, size(c).Y)
				line = Vector{}
				continue
			}
			s := size(c)
			line.X += s.X
			line.Y = max(line.Y, s.Y)
		}
		return Vector{max(box.X, line.X), box.Y + line.Y}
	}
	parent := &html.Node{Type: html.ElementNode, Data: "div"}
	for _, n := range nodes {
		parent.AppendChild(n)
	}
	return size(parent)
}
