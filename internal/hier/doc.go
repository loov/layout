// Package hier implements the stages of hierarchical (Sugiyama) graph layout
// on a compact internal graph representation.
//
// The stages are run in order on a Graph:
//
//  1. Decycle reverses edges until the graph is acyclic.
//  2. Rank assigns each node a rank (row), tightens and balances them.
//  3. AddVirtuals splits edges spanning several ranks with virtual nodes.
//  4. OrderRanks orders nodes within ranks to reduce edge crossings.
//  5. Position assigns coordinates using the Brandes-Köpf algorithm.
//
// Each stage has a Default* entry point; the package layout drives them.
package hier
