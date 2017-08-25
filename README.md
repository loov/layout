# layout [![GoDoc](https://godoc.org/github.com/loov/layout?status.svg)](https://godoc.org/github.com/loov/layout) [![Go Report Card](https://goreportcard.com/badge/github.com/loov/layout)](https://goreportcard.com/report/github.com/loov/layout)

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

Basic usage:

```
package main

import (
    "os"

    "github.com/loov/layout"
    "github.com/loov/layout/format/svg"
)

func main() {
    graph := layout.NewDigraph()
    graph.Node("A")
    graph.Node("B")
    graph.Node("C")
    graph.Node("D")
    graph.Edge("A", "B")
    graph.Edge("A", "C")
    graph.Edge("B", "D")
    graph.Edge("C", "D")
    graph.Edge("D", "A")

    layout.Hierarchical(graph)

    svg.Write(os.Stdout, graph)
}
```

## Quality

Currently the `layout.Hierarchy` algorithm output is significantly worse than graphviz. It is recommended to use `graphviz dot`, if possible.