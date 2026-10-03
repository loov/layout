package svg_test

import (
	"log"
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func ExampleWrite() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b").Label = "next"
	if err := layout.Hierarchical(graph); err != nil {
		log.Fatal(err)
	}
	if err := svg.Write(os.Stdout, graph); err != nil {
		log.Fatal(err)
	}
	// Output:
	// <svg xmlns='http://www.w3.org/2000/svg' width='80.88' height='156'>
	// 	<style type="text/css"><![CDATA[
	// 		.edge { fill: none; }
	// 	]]></style>
	// 	<defs>
	// 		<marker id="normal" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	// 	      <path d="M0,0 L0,8 L10,4 z" fill="context-stroke" />
	// 	    </marker>
	// 		<marker id="vee" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	// 	      <path d="M0,0 L10,4 L0,8 L3,4 z" fill="context-stroke" />
	// 	    </marker>
	// 		<marker id="dot" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	// 	      <circle cx="4" cy="4" r="3.5" fill="context-stroke" />
	// 	    </marker>
	// 		<marker id="odot" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	// 	      <circle cx="4" cy="4" r="3" fill="white" stroke="context-stroke" />
	// 	    </marker>
	// 	</defs><g><path class='edge' marker-end='url(#normal)' stroke='#000000' stroke-width='1' d='M32,48 L32,63 Q32,78 32,93 L32,108 '></path><text text-anchor='middle' alignment-baseline='middle' x='51.44' y='90' font-size='14' fill='#000000'>next</text>
	// <ellipse cx='32' cy='32' rx='16' ry='16' class='node' fill='#FFFFFF' stroke='#000000' stroke-width='1'></ellipse><text text-anchor='middle' alignment-baseline='middle' x='32' y='32' font-size='14' fill='#000000'>a</text>
	// <ellipse cx='32' cy='124' rx='16' ry='16' class='node' fill='#FFFFFF' stroke='#000000' stroke-width='1'></ellipse><text text-anchor='middle' alignment-baseline='middle' x='32' y='124' font-size='14' fill='#000000'>b</text>
	// </g></svg>
}
