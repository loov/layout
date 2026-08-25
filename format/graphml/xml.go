package graphml

import "encoding/xml"

// File is the root <graphml> element.
type File struct {
	XMLName           xml.Name `xml:"graphml"`
	XMLNS             string   `xml:"xmlns,attr"`
	XMLNSXSI          string   `xml:"xmlns:xsi,attr"`
	XMLNSY            string   `xml:"xmlns:y,attr"`
	XSISchemaLocation string   `xml:"xsi:schemalocation,attr"`

	Key    []Key    `xml:"key"`
	Graphs []*Graph `xml:"graph"`
}

// NewFile creates a File with the GraphML and yFiles namespaces set.
func NewFile() *File {
	file := &File{}
	file.XMLNS = "http://graphml.graphdrawing.org/xmlns"
	file.XMLNSXSI = "http://www.w3.org/2001/XMLSchema-instance"
	file.XMLNSY = "http://www.yworks.com/xml/graphml"
	file.XSISchemaLocation = "http://graphml.graphdrawing.org/xmlns http://graphml.graphdrawing.org/xmlns/1.0/graphml.xsd"
	return file
}

// Graph is a <graph> element.
type Graph struct {
	// XMLName xml.Name `xml:"graph"`
	ID          string      `xml:"id,attr"`
	EdgeDefault EdgeDefault `xml:"edgedefault,attr"`

	Node      []Node      `xml:"node"`
	Edge      []Edge      `xml:"edge"`
	Hyperedge []Hyperedge `xml:"hyperedge"`
	// TODO: parse info
}

// Key declares a data attribute that nodes or edges may carry.
type Key struct {
	ID  string `xml:"id,attr"`
	For string `xml:"for,attr"`

	AttrName string `xml:"attr.name,attr,omitempty"`
	AttrType string `xml:"attr.type,attr,omitempty"`

	YFilesType string `xml:"yfiles.type,attr,omitempty"`
}

// Node is a <node> element.
type Node struct {
	// XMLName xml.Name `xml:"node"`
	ID    string   `xml:"id,attr"`
	Port  []Port   `xml:"port"`
	Graph []*Graph `xml:"graph"`
	Attrs []Attr   `xml:"data"`

	// TODO: parse info
}

// Port is a named attachment point on a node.
type Port struct {
	// XMLName xml.Name `xml:"port"`
	Name string `xml:"name,attr"`
}

// Edge is an <edge> element.
type Edge struct {
	// XMLName xml.Name `xml:"edge"`
	ID string `xml:"id,attr,omitempty"`

	Source   string `xml:"source,attr"`
	Target   string `xml:"target,attr"`
	Directed *bool  `xml:"directed,attr,omitempty"`

	SourcePort string `xml:"sourceport,attr,omitempty"`
	TargetPort string `xml:"targetport,attr,omitempty"`

	Attrs []Attr `xml:"data"`
}

// EdgeDefault is the default direction of edges in a graph.
type EdgeDefault string

// Edge directions.
const (
	Undirected = EdgeDefault("undirected")
	Directed   = EdgeDefault("directed")
)

// Attr is a <data> element carrying the value for a Key.
type Attr struct {
	// XMLName xml.Name `xml:"data"`
	Key   string `xml:"key,attr"`
	Value []byte `xml:",innerxml"`
}

// Hyperedge is an edge connecting any number of endpoints.
type Hyperedge struct {
	// XMLName xml.Name `xml:"hyperedge"`

	ID       string     `xml:"id,attr,omitempty"`
	Endpoint []Endpoint `xml:"endpoint"`
}

// Endpoint is one end of a hyperedge.
type Endpoint struct {
	// XMLName xml.Name `xml:"endpoint"`
	Node string       `xml:"node,attr"`
	Port string       `xml:"port,attr,omitempty"`
	Type EndpointType `xml:"type,attr,omitempty"`
}

// EndpointType is the direction of a hyperedge endpoint.
type EndpointType string

// Endpoint directions.
const (
	EndpointIn    = EndpointType("in")
	EndpointOut   = EndpointType("out")
	EndpointUndir = EndpointType("undir")
)
