# Rules for good text layouts

What makes a text drawing read well. Rules marked *(test)* are checked by
tests. The rest are judged by eye, with the diagnostics in
`testdata/diagnostics_text.txt` as a guide.

## Edges

- **Every edge joins its two nodes**, after carving and straightening have
  reworked the cells *(test: `TestEdgesJoined`)*.
- **Edges don't share a run.** Two edges along the same cells can't be told
  apart, nor whose arrowhead is whose. Text draws such runs heavy, so a shared
  run doesn't read as edges joining *(test: `TestEdgesApart`)*. Merged edges
  are one edge and may share a run.
- **A crossing looks like a crossing.** It is drawn `╂`, never `┼`, which
  reads as four edges meeting. Bends are rounded so that they stand apart from
  tees `├ ┴`, which mean a join.
- **Fewer corners.** A change that adds corners is worse, even when another
  measure improves.
- **No staircases.** A long edge stays in line with one of its ends and turns
  once, rather than jogging at every rank it passes.
- **Fewer crossings.** A move that saves corners but adds a crossing is worse.
- **Crossings away from corners and arrowheads.** A `╂` right beside a bend or
  an arrowhead is hard to follow; cross on a straight stretch of both edges.
- **A main path runs along one line** where the graph has one, such as the path
  from the start of an automaton.
- **No humps.** An edge doesn't step away from its target and back, as an edge
  that leaves upward and then turns down to a node level with its start.
- **Different edges keep a cell apart, where there is room.** Runs of two
  edges in neighboring columns, or a line right beside another edge's corner
  (`╯│`), read as touching. Leave a blank column between them, moving an end
  along its box if needed. This is a preference: a dense graph may pack edges
  side by side to stay compact.

  ```text
   touching           apart

      │▲                 │ ▲
   ╭──╯│              ╭──╯ │
   ▼   │              ▼    │
  ```

- **Short edges.** An edge end slides along its box toward the node it heads
  to, so the edge turns early and its runs stay short. A node joins its
  neighbors where its edges are shortest, rather than off to one side with one
  long edge.
- **Back edges keep to the outside.** Edges against the rank direction go
  around the drawing rather than threading between the forward edges.
- **Parallel edges run side by side.** Edges between the same two nodes, either
  way, run next to each other, each with its label beside its own line.

## Edge ends

- **Arrows end beside a node, never on its border.** An arrowhead on the top or
  bottom border of a box has no room to end before the node
  *(test: `TestArrowsBesideNodes`)*. An arrowhead never covers a node's label.
- **Each end gets its own row or column** on its side of a node: not shared
  with another end, not on a corner, and not running along the border.
- **An edge visibly attaches.** Where an edge without a marker leaves a side,
  the border shows it as `├` or `┤`.
- **Ends are about a cell apart**, with no blank column between neighbors that
  carving could not remove.
- **Edges fan out of a dot on separate sides** rather than sharing a run.
- **Ends sit balanced and symmetrical** on their side of a node: one end in
  the middle, several spread evenly about the middle (`╰┬─┬╯` rather than
  `╰┬┬──╯`). This is a preference that gives way to straight edges and to
  compactness: an end slides off center when that straightens its edge or
  saves room.

## Labels

- **A label belongs to one edge, unambiguously.** It sits right against its
  own edge, clearly nearer it than any other. A label in the row between two
  edges, as near one as the other, is wrong.

  ```text
   fine                  fine                  ambiguous

     SS(B)                 SS(B)                 SS(B)
   ──────────────▶       ──────────────▶       ──────────────▶
   ─╮                    ───────────╮          ─╮   SS(S)
    │   SS(S)               SS(S)   │           ╰───────────╮
    ╰─────────╮                     ▼                       ▼
              ▼
  ```

- **Labels sit against their line**, with no blank row, or more than one blank
  column, between them.
- **Labels go on the side the direction suggests**: right of the edge top to
  bottom, above it left to right, as Graphviz does. Below an edge running
  sideways, a label reads as belonging to what is below. This is a preference:
  take the other side when that keeps a label unambiguous or an edge straight.
- **Labels stay clear of node borders.** A label squeezed against a box border,
  or in the row of the borders between boxes, reads as part of the node. A
  label of an edge along a rank goes above the edge, in the row of the nodes'
  top borders:

  ```text
   against borders                    clear

    ╭─┴─╮            ╭───╮  ╭───╮      ╭─┴─╮ flat label ╭───╮x ╭───╮
    │ a ├───────────▶│ b ├─▶│ c │      │ a ├───────────▶│ b ├─▶│ c │
    ╰───╯ flat label ╰───╯x ╰───╯      ╰───╯            ╰───╯  ╰───╯
  ```

- **Labels take no rows of their own** when they fit beside a bend or a run
  that is there anyway, such as right after the corner where an edge turns
  toward its target.
- **Labels never cover each other.** When no spot is clear, crossing an edge
  reads better than covering a label.
- **Labels keep their text**: blanks inside a label stay, labels are padded
  with a space each side and centered, and they stay off cluster frames.

## Forks and merged edges

- **Merged edges can't read as edges that aren't there.** An edge merges at
  its start or its end, never both, and only with edges that look the same.
  Edges with labels, ports, loops and flat edges stay apart.
- **Forks and joins are balanced.** A node with several merged edges to one
  side sits over the middle of them, and the fork starts from the middle of the
  node. A change that pushes a balanced fork off center is worse, even when it
  saves corners elsewhere. Without merging, a node with two unlabeled edges in
  from the rank above still sits between its parents, so that both edges are
  about as long.
- **Straight edges win over centering where nothing merges.** Labeled edges
  never merge, so a node with only labeled edges to one side isn't a fork.
  Lining it up with one neighbor so that the edge runs straight beats centering
  it.

## Nodes, clusters and space

- **The nodes of a rank stay in line.** Carving never moves one node of a rank
  out of line with the others. Unless user specifically relaxed the rule.
- **Room to tell things apart.** Edges keep a cell off cluster frames, the fans
  of different nodes are a cell apart, and neighboring cluster frames never
  share a line.
- **An edge crosses a cluster frame at most once**, squarely, not at a corner,
  and doesn't dip in and out of a cluster.
- **Boxes fit their content**: as tall and wide as their labels and edge ends
  need, with no spare row or column.
- **Compact.** Don't widen the drawing to straighten an edge, and carve away
  the rows and columns that nothing needs. Nodes of a rank sit a gap apart,
  not further, and ranks are no further apart than the labels between them
  need.

## Overall

- **Symmetry.** Symmetric parts of a graph draw symmetrically, such as a node
  with two alike subtrees.
- **Input order breaks ties.** When two orders cost the same, nodes keep the
  order they are declared in, so the drawing matches how its author thinks of
  the graph.
- **Stable and deterministic.** The same graph always draws the same, on every
  machine. A small change to the graph should give a small change to the
  drawing, so that diffs stay readable.

## Priorities

When rules pull against each other, the earlier wins:

1. Every edge joins its nodes, and every label belongs unambiguously to its edge.
2. Edges don't share a run.
3. Fewer crossings.
4. Fewer corners.
5. Straight edges.
6. Balanced forks and joins.
7. Compactness.
8. Balanced, symmetrical edge ends.
9. Symmetry.

## Judging a change

Compare the drawings before and after in testdiff
(`go run ./internal/cmd/testdiff`), with the corners, crossings, rows and
unbalance of each drawing that changed. A change is good when no drawing gets
worse by these rules. A better total doesn't make up for one drawing that gets
worse.
