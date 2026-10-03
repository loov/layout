package text

import (
	"strings"

	"github.com/loov/layout/internal/draw"
	"golang.org/x/net/html"
)

// plain returns a label as text draws it: an HTML-like label without its
// markup, with a line per <br> and per table row and cells apart
func plain(label string) string {
	if !draw.IsHTMLLabel(label) {
		return label
	}
	nodes, err := html.ParseFragment(strings.NewReader(label[1:len(label)-1]), &html.Node{Type: html.ElementNode})
	if err != nil {
		return label
	}
	var out strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			out.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			out.WriteByte('\n')
		case n.Type == html.ElementNode && (n.Data == "td" || n.Data == "th"):
			out.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			out.WriteByte('\n')
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	// markup whitespace doesn't show, like in a browser
	lines := strings.Split(out.String(), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
