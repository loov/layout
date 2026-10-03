package graphml

import (
	"strings"
	"testing"
)

func TestNestedGraphsUseTheirOwnEdgeDirectionDefault(t *testing.T) {
	for _, tc := range []struct {
		name, root, nested string
		want               bool
	}{
		{"undirected child", "directed", "undirected", false},
		{"directed child", "undirected", "directed", true},
		{"inherited directed", "directed", "", true},
		{"inherited undirected", "undirected", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `<graphml><graph edgedefault="` + tc.root + `"><node id="parent"><graph` + edgeDefault(tc.nested) + `><node id="a"/><node id="b"/><edge source="a" target="b"/><edge source="b" target="a" directed="true"/><edge source="a" target="a" directed="false"/></graph></node></graph></graphml>`
			graphs, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			edges := graphs[0].Edges
			if len(edges) != 3 {
				t.Fatalf("got %d edges", len(edges))
			}
			if edges[0].Directed != tc.want {
				t.Errorf("nested default = %v, want %v", edges[0].Directed, tc.want)
			}
			if !edges[1].Directed || edges[2].Directed {
				t.Error("explicit edge directions were not preserved")
			}
		})
	}
}

func edgeDefault(value string) string {
	if value == "" {
		return ""
	}
	return ` edgedefault="` + value + `"`
}
