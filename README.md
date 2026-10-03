# layout [![GoDoc](https://godoc.org/github.com/loov/layout?status.svg)](https://godoc.org/github.com/loov/layout)

layout draws graphs in pure Go.

You build a graph in code or read one from a DOT or GraphML file. `layout.Hierarchical` places the nodes in ranks and routes the edges, in the same style as Graphviz `dot`. `layout.Force` is there for graphs without a clear direction. The result can be written as SVG, Unicode text for a terminal, JSON coordinates, DOT or GraphML.

It understands a good part of the DOT language: clusters, record and HTML-like labels, ports, `rankdir`, `minlen`, and colors by X11 name.

## Installation

The graph layouting can be used as a command-line tool and as a library.

To install the command-line tool:
```
go get -u github.com/loov/layout/cmd/glay
```

To install the package:
```
go get -u github.com/loov/layout
```

## Usage

```go
package main

import (
    "log"
    "os"

    "github.com/loov/layout"
    "github.com/loov/layout/format/svg"
)

func main() {
    graph := layout.NewDigraph()
    graph.Edge("checkout", "build")
    graph.Edge("checkout", "lint")
    graph.Edge("build", "unit")
    graph.Edge("build", "integration")
    graph.Edge("lint", "review").LineStyle = layout.Dashed
    graph.Edge("unit", "review")
    graph.Edge("integration", "review").Label = "slow"

    approve := graph.Edge("review", "deploy")
    approve.Label = "approve"
    approve.LineColor = layout.RGB{G: 0x80}
    graph.Node("review").Shape = layout.Box
    graph.Node("deploy").FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}

    graph.Clusters = []*layout.Cluster{{
        ID: "test", Label: "test",
        Nodes:     []*layout.Node{graph.Node("unit"), graph.Node("integration")},
        LineColor: layout.RGB{B: 0xFF},
    }}

    if err := layout.Hierarchical(graph); err != nil {
        log.Fatal(err)
    }
    svg.Write(os.Stdout, graph)
}
```

![Output](./testdata/readme.svg)

The same graph drawn for a terminal, with `graph.ForText = true` before laying out and `text.Write` instead of `svg.Write`:

```
                        ╭─────────╮
                        │checkout │
                        ╰──┬──┬───╯
                           │  │
                    ╭──────╯  ╰────────────╮
                    │                      │
                    ▼                      ▼
                 ╭──────╮               ╭─────╮
                 │build │               │lint │
                 ╰─┬─┬──╯               ╰──┬──╯
                   │ │                     ┊
           ╭───────╯ ╰───────╮             ┊
           │                 │             ┊
┌ test ┈┈┈┈╂┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈╂┈┈┈┈┈┈┈┈┈┈┈┐ ┊
┊          │                 │           ┊ ┊
┊          ▼                 ▼           ┊ ┊
┊       ╭─────╮        ╭────────────╮    ┊ ┊
┊       │unit │        │integration │    ┊ ┊
┊       ╰──┬──╯        ╰─────┬──────╯    ┊ ┊
┊          │                 │           ┊ ┊
└┈┈┈┈┈┈┈┈┈┈╂┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈╂┈┈┈┈┈┈┈┈┈┈┈┘ ┊
           │                 │             ┊
           │                 │ slow        ┊
           │                 │             ┊
           ╰───────────────╮ │ ┌┈┈┈┈┈┈┈┈┈┈┈┘
                           │ │ ┊
                           ▼ ▼ ▼
                         ┌───────┐
                         │review │
                         └───┬───┘
                             │
                             │ approve
                             │
                             ▼
                         ╭───────╮
                         │deploy │
                         ╰───────╯
```

Other layouts and outputs:

* `layout.Force(graph)` — force-directed layout for undirected or cyclic graphs.
* `format/json` — plain coordinates for drawing the graph elsewhere.

The same is available from the command line: `glay -l force -q fast -t txt|ans|json|svg|dot|graphml input.dot`; colored text takes `-colors 16|truecolor` and `-bg color`.
