package tools

import "sdmm/internal/util"

// RouteMapState is the state of the Routes tool. The computation and drawing are done by the map pane.
type RouteMapState struct {
	Start, End *util.Point
	Accesses   map[string]bool
	AllowSpace bool

	// Version is incremented on every change, so the pane knows when to recompute the route.
	Version int
}

var RouteMap = &RouteMapState{Accesses: make(map[string]bool)}

// Changed marks the state as changed.
func (s *RouteMapState) Changed() {
	s.Version++
}

// ToolRouteMap shows where a crew member with selected accesses can walk.
//
// Default: set the start point.
// Alternative: set the target point.
type ToolRouteMap struct {
	tool
}

func (ToolRouteMap) Name() string {
	return TNRouteMap
}

func newRouteMap() *ToolRouteMap {
	return &ToolRouteMap{}
}

func (t *ToolRouteMap) onStart(coord util.Point) {
	c := coord
	if t.AltBehaviour() {
		RouteMap.End = &c
	} else {
		RouteMap.Start = &c
	}
	RouteMap.Changed()
}
