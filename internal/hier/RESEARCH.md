# Research

## graphviz - dot

dot draws graphs in four main phases. Knowing this helps you to understand what kind of layouts dot makes and how you can control them. The layout procedure used by dot relies on the graph being acyclic. Thus, the first step is to break any cycles which occur in the input graph by reversing the internal direction of certain cyclic edges. The next step assigns nodes to discrete ranks or levels. In a top-to-bottom drawing, ranks determine Y coordinates. Edges that span more than one rank are broken into chains of “virtual” nodes and unit-length edges. The third step orders nodes within ranks to avoid crossings. The fourth step sets X coordinates of nodes to keep edges short, and the final step routes edge splines. This is the same general approach as most hierarchical graph drawing programs, based on the work of Warfield [War77], Carpano [Car80] and Sugiyama [STT81]. We refer the reader to [GKNV93] for a thorough explanation of dot’s algorithms.

The dot algorithm produces a ranked layout of a graph respecting edge directions if possible. It is particularly appropriate for displaying hierarchies or directed acyclic graphs. The basic layout scheme is attributed to Sugiyama et al.[STT81] The specific algorithm used by dot follows the steps described by Gansner et al.[GKNV93]

The steps in the dot layout are:
  1) initialize
  2) rank
  3) mincross
  4) position
  5) sameports
  6) splines
  7) compoundEdges

* http://www.graphviz.org/pdf/dotguide.pdf
* http://www.graphviz.org/doc/libguide/libguide.pdf

## A Technique for Drawing Directed Graphs

We describe a four-pass algorithm for drawing directed graphs. The first pass finds an optimal rank assignment using a network simplex algorithm. The second pass sets the vertex order within ranks by an iterative heuristic incorporating a novel weight function and local transpositions to reduce crossings. The third pass finds optimal coordinates for nodes by constructing and ranking an auxiliary graph. The fourth pass makes splines to draw edges. The algorithm makes good drawings and runs fast.

* http://www.graphviz.org/Documentation/TSE93.pdf

## References

* http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.38.3837&rep=rep1&type=pdf
* http://docs.yworks.com/yfiles/doc/api/y/layout/hierarchic/IncrementalHierarchicLayouter.html
* http://hci.stanford.edu/courses/cs448b/w09/lectures/20090204-GraphsAndTrees.pdf
* http://marvl.infotech.monash.edu/~dwyer/
* http://mgarland.org/files/papers/layoutgpu.pdf
* http://research.microsoft.com/en-us/um/people/holroyd/papers/bundle.pdf
* http://stackoverflow.com/questions/19245350/graphviz-dot-algorithm
* http://www.csse.monash.edu.au/~tdwyer/Dwyer2009FastConstraints.pdf
* http://www.graphviz.org/Theory.php
* http://www.graphviz.org/Documentation/TSE93.pdf
* http://www.graphviz.org/pdf/dotguide.pdf
* https://en.wikipedia.org/wiki/Graph_drawing
* https://github.com/cpettitt/dagre/tree/master/lib/rank
* https://github.com/cpettitt/dagre/wiki#recommended-reading
* https://github.com/cpettitt/dig.js/tree/master/src/dig/dot
* https://github.com/d3/d3/issues/349
* https://github.com/ellson/graphviz/tree/master/lib/dotgen2
* https://www.microsoft.com/en-us/research/publication/drawing-graphs-with-glee/