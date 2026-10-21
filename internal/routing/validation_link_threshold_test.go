package routing

import (
	"math"
	"testing"
)

func TestUT020_RelayTreeLinkThresholds(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	tree := testTree(1, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil)
	constraints := testConstraints()
	constraints.MinLinkQuality = 0.8
	constraints.MaxLinkLoss = 0.1

	tests := []struct {
		name string
		edit func(*NeighborGraph)
		want string
	}{
		{name: "source quality below minimum", edit: func(g *NeighborGraph) {
			g.Links[0].Quality = math.Nextafter(0.8, 0)
		}, want: "quality"},
		{name: "relay quality below minimum", edit: func(g *NeighborGraph) {
			g.Links[1].Quality = math.Nextafter(0.8, 0)
		}, want: "quality"},
		{name: "source loss above maximum", edit: func(g *NeighborGraph) {
			g.Links[0].Loss = math.Nextafter(0.1, 1)
		}, want: "loss"},
		{name: "relay loss above maximum", edit: func(g *NeighborGraph) {
			g.Links[1].Loss = math.Nextafter(0.1, 1)
		}, want: "loss"},
		{name: "inclusive threshold boundaries", edit: func(g *NeighborGraph) {}},
		{name: "inside thresholds", edit: func(g *NeighborGraph) {
			for i := range g.Links {
				g.Links[i].Quality = math.Nextafter(0.8, 1)
				g.Links[i].Loss = math.Nextafter(0.1, 0)
			}
		}},
		{name: "unselected poor link", edit: func(g *NeighborGraph) {
			link := testLinks([2]uint16{90, 91})[0]
			link.Quality = 0
			link.Loss = 1
			g.Links = append(g.Links, link)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			graph := testGraph(testLinks([2]uint16{1, 10}, [2]uint16{10, 2}))
			for i := range graph.Links {
				graph.Links[i].Quality = constraints.MinLinkQuality
				graph.Links[i].Loss = constraints.MaxLinkLoss
			}
			tc.edit(&graph)
			err := ValidateRelayTree(snapshot, graph, constraints, tree)
			if tc.want != "" {
				requireRoutingError(t, err, tc.want)
			} else if err != nil {
				t.Fatalf("valid selected links rejected: %v", err)
			}
		})
	}
}
