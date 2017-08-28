package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/svg"
)

var (
	informat  = flag.String("s", "", "input format")
	outformat = flag.String("t", "svg", "output format")

	verbose = flag.Bool("v", false, "verbose output")
)

func infof(format string, args ...interface{}) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, args...)
		if !strings.HasSuffix("\n", format) {
			fmt.Fprint(os.Stderr, "\n")
		}
	}
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	if !strings.HasSuffix("\n", format) {
		fmt.Fprint(os.Stderr, "\n")
	}
}

func main() {
	flag.Parse()

	input := flag.Arg(0)
	output := flag.Arg(1)

	if input == "" {
		errorf("input is missing")
		flag.Usage()
		return
	}

	if *informat == "" {
		// try to detect input format
		switch strings.ToLower(filepath.Ext(input)) {
		case ".dot":
			*informat = "dot"
		case ".gv":
			*informat = "dot"
		}
	}

	if output != "" {
		*outformat = ""
	}
	if *outformat == "" {
		// try to detect output format
		switch strings.ToLower(filepath.Ext(output)) {
		case ".svg":
			*outformat = "svg"
		default:
			*outformat = "svg"
		}
	}

	if *informat == "" || *outformat == "" {
		errorf("unable to detect input or output format")
		flag.Usage()
		os.Exit(1)
		return
	}

	var graphs []*layout.Graph
	var err error

	infof("parsing %q", input)

	switch *informat {
	case "dot":
		graphs, err = dot.ParseFile(input)
	default:
		errorf("unknown input format %q", *informat)
		flag.Usage()
		os.Exit(1)
		return
	}

	if err != nil || len(graphs) == 0 {
		if len(graphs) == 0 && err == nil {
			err = errors.New("file doesn't contain graphs")
		}
		errorf("failed to parse %q: %v", input, err)
		os.Exit(1)
		return
	}

	if len(graphs) != 1 {
		infof("parsed %v graphs", len(graphs))
	} else {
		infof("parsed 1 graph")
	}

	graph := graphs[0]
	if len(graphs) > 1 {
		errorf("file %q contains multiple graphs, processing only first\n", input)
	}

	// layout
	layout.Hierarchical(graph)

	// output
	var out io.Writer
	if output == "" {
		out = os.Stdout
	} else {
		file, err := os.Create(output)
		if err != nil {
			errorf("unable to create file %q: %v", output, err)
			os.Exit(1)
			return
		}
		defer file.Close()
		out = file
	}

	switch *outformat {
	case "svg":
		err = svg.Write(out, graph)
	default:
		errorf("unknown output format %q", *outformat)
		os.Exit(1)
		return
	}

	if err != nil {
		errorf("writing %q failed: %v", output, err)
		os.Exit(1)
		return
	}
}
