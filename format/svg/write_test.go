package svg

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/loov/layout"
)

func TestSanitizeHTMLRemovesObfuscatedScriptURLs(t *testing.T) {
	for _, url := range []string{
		"javascript:alert(1)", "java&#x09;script:alert(1)",
		"java&#x0a;script:alert(1)", "java&#x0d;script:alert(1)",
		"&#x01;JaVaScRiPt:alert(1)", "da&#x09;ta:text/html,unsafe",
	} {
		t.Run(url, func(t *testing.T) {
			got := sanitizeHTML(`<a href="` + url + `">click</a>`)
			if strings.Contains(got, "href=") || !strings.Contains(got, "click") {
				t.Fatalf("unsafe URL retained: %s", got)
			}
		})
	}
}

func TestSanitizeHTMLPreservesSafeLinks(t *testing.T) {
	for _, url := range []string{"https://example.com/", "/relative", "#anchor", "mailto:someone@example.com"} {
		t.Run(url, func(t *testing.T) {
			got := sanitizeHTML(`<a href="` + url + `">click</a>`)
			if !strings.Contains(got, `href="`+url+`"`) {
				t.Fatalf("safe link removed: %s", got)
			}
		})
	}
}

func TestWriteIsWellFormedXML(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("a").Label = `<<b>x<!-- a -- b --></b><i 1a="v" foo:bar="w" ok="y" ok="z">i</i><foo:bar>t</foo:bar>>`
	graph.Node("b").Label = "x\x01y"
	graph.Node("b").Tooltip = "t\x02￾"
	graph.Edge("a", "b").Label = "<<u>e</u>&amp;<br>z\x0b>"
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(&out)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			names := []xml.Name{start.Name}
			for _, attr := range start.Attr {
				names = append(names, attr.Name)
			}
			for _, name := range names {
				if name.Space != "" && !strings.Contains(name.Space, "://") {
					t.Errorf("undeclared prefix in %s:%s", name.Space, name.Local)
				}
			}
		}
	}
}

func TestColorsKeepAlpha(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("a").FillColor = layout.RGBA{R: 0xFF, A: 0x80}
	graph.Node("b").LineColor = layout.RGB{G: 0xFF}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fill='rgba(255,0,0,0.502)'", "stroke='#00FF00'"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s in\n%s", want, out.String())
		}
	}
}

func TestHTMLClusterLabel(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Clusters = []*layout.Cluster{{ID: "c", Label: "<<b>bold</b>>", Nodes: []*layout.Node{graph.Node("a")}}}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<b>bold</b></body></foreignObject>") || strings.Contains(out.String(), "&lt;b&gt;") {
		t.Errorf("cluster label not drawn as HTML:\n%s", out.String())
	}
}

func TestSanitizeHTMLRemovesRawTextElements(t *testing.T) {
	for _, tag := range []string{"noscript", "xmp", "noembed", "noframes", "plaintext"} {
		t.Run(tag, func(t *testing.T) {
			got := sanitizeHTML(`<b>ok</b><` + tag + `><img src="x" onerror="alert(1)"/></` + tag + `>`)
			if strings.Contains(got, "onerror") || strings.Contains(got, "<img") {
				t.Fatalf("raw text survived: %s", got)
			}
			if !strings.Contains(got, "<b>ok</b>") {
				t.Fatalf("safe markup removed: %s", got)
			}
		})
	}
}

// TestUnknownArrowDrawsNormal checks that arrow styles without a marker
// of their own still show the edge direction.
func TestUnknownArrowDrawsNormal(t *testing.T) {
	graph := layout.NewDigraph()
	edge := graph.Edge("a", "b")
	edge.ArrowHead, edge.ArrowTail = "diamond", "tee"
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"marker-end='url(#normal)'", "marker-start='url(#normal)'"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
}

// TestInvisible checks that invisible nodes, edges and clusters are left
// out of the drawing.
func TestInvisible(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("hidden").Invisible = true
	graph.Edge("hidden", "a")
	graph.Edge("a", "b").Invisible = true
	graph.Edge("a", "b").Label = "secret"
	graph.Clusters = []*layout.Cluster{{ID: "c", Nodes: []*layout.Node{graph.Node("b")}, Invisible: true}}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	svg := out.String()
	if got := strings.Count(svg, "class='node'"); got != 2 {
		t.Errorf("drew %d nodes, want 2", got)
	}
	if got := strings.Count(svg, "class='edge'"); got != 1 {
		t.Errorf("drew %d edges, want 1", got)
	}
	if strings.Contains(svg, "class='cluster'") || strings.Contains(svg, "secret") {
		t.Errorf("drew an invisible cluster or label:\n%s", svg)
	}
}
