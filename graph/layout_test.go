/*
 *  Copyright IBM Corporation 2026
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package graph

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	graphtypes "github.com/konveyor/move2kube/types/graph"
)

func sortedNodes(nodes []graphtypes.Node) []graphtypes.Node {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Id < nodes[j].Id })
	return nodes
}

func sortedEdges(edges []graphtypes.EdgeT) []graphtypes.EdgeT {
	sort.Slice(edges, func(i, j int) bool { return edges[i].Id < edges[j].Id })
	return edges
}

func TestGetNodesAndEdges(t *testing.T) {
	g := graphtypes.NewGraph()
	v0 := g.AddVertex("input", 0, nil)
	v1 := g.AddVertex("Dockerfile", 1, map[string]interface{}{"pathMappings": "src -> dest"})
	v2 := g.AddVertex("Kubernetes", 2, map[string]interface{}{"pathMappings": 42})
	g.AddEdge(v0, v1, "Service", map[string]interface{}{"newArtifact": []interface{}{"Dockerfile"}})
	g.AddEdge(v1, v2, "IR", nil)

	nodes, edges := GetNodesAndEdges(*g)

	wantNodes := []graphtypes.Node{
		{Id: "v-0", Type: graphtypes.GraphNodeTypeInput, Position: graphtypes.Position{Y: 0}, Data: graphtypes.Data{Label: "input"}},
		{Id: "v-1", Position: graphtypes.Position{Y: 1}, Data: graphtypes.Data{Label: "Dockerfile", PathMappings: "src -> dest"}},
		{Id: "v-2", Position: graphtypes.Position{Y: 2}, Data: graphtypes.Data{Label: "Kubernetes"}},
	}
	if diff := cmp.Diff(wantNodes, sortedNodes(nodes)); diff != "" {
		t.Errorf("nodes mismatch (-want +got):\n%s", diff)
	}

	arrow := graphtypes.MarkerEnd{Type: graphtypes.GraphEdgeArrowClosed}
	wantEdges := []graphtypes.EdgeT{
		{Id: "e-0", Source: "v-0", Target: "v-1", Label: "Service (Dockerfile)", MarkerEnd: arrow},
		{Id: "e-1", Source: "v-1", Target: "v-2", Label: "IR", MarkerEnd: arrow},
	}
	if diff := cmp.Diff(wantEdges, sortedEdges(edges)); diff != "" {
		t.Errorf("edges mismatch (-want +got):\n%s", diff)
	}
}

func TestBfsUpdatePositions(t *testing.T) {
	// v-0 (iter 0) fans out to v-1 and v-2 (iter 1); v-1 -> v-3 is in the same
	// iteration so v-3 becomes a sub iteration; v-4 is unreachable.
	node := func(id string, iter int) graphtypes.Node {
		return graphtypes.Node{Id: id, Position: graphtypes.Position{Y: iter}}
	}
	nodes := []graphtypes.Node{node("v-0", 0), node("v-1", 1), node("v-2", 1), node("v-3", 1), node("v-4", 2)}
	edges := []graphtypes.EdgeT{
		{Source: "v-0", Target: "v-1"},
		{Source: "v-0", Target: "v-2"},
		{Source: "v-1", Target: "v-3"},
	}

	BfsUpdatePositions(nodes, edges)

	got := map[string]graphtypes.Position{}
	for _, n := range nodes {
		got[n.Id] = n.Position
	}
	want := map[string]graphtypes.Position{
		"v-0": {X: 0, Y: 0},
		"v-1": {X: 0, Y: 200},
		"v-2": {X: 200, Y: 200},
		"v-3": {X: 0, Y: 400},
		// unreachable nodes are still placed so the UI can render them
		"v-4": {X: 0, Y: 0},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("positions mismatch (-want +got):\n%s", diff)
	}
}

func TestGraphSortIters(t *testing.T) {
	s := graphSortIters{Items: []graphIterAndSubIter{{2, 0}, {1, 1}, {1, 0}, {0, 3}}}
	sort.Stable(&s)
	want := []graphIterAndSubIter{{0, 3}, {1, 0}, {1, 1}, {2, 0}}
	if diff := cmp.Diff(want, s.Items); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}
