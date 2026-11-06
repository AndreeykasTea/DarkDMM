package tools

import (
	"fmt"

	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/overlay"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

// RouteSettings are settings of the Route tool.
type RouteSettings struct {
	// UseCaps places caps on open ends of atmospheric pipes.
	UseCaps bool

	// Result of the last route, shown in the panel.
	LastResult   string
	LastWarnings []string
}

var Route = &RouteSettings{UseCaps: true}

// ToolRoute draws power cables, atmospheric and disposal pipes along the mouse path.
// Pieces are chosen automatically: straight, bends, junctions, knots/caps on ends,
// and merged with already existing pieces of the same network.
// Any other object is just placed along the path.
type ToolRoute struct {
	tool

	path     []util.Point
	visited  map[util.Point]bool
	dragging bool
}

func (ToolRoute) Name() string {
	return TNRoute
}

func newRoute() *ToolRoute {
	return &ToolRoute{visited: make(map[util.Point]bool)}
}

func (t *ToolRoute) Stale() bool {
	return !t.dragging
}

// Network returns the network which will be built with the selected object, or nil.
func (t *ToolRoute) Network() *dmnet.Network {
	prefab, ok := ed.SelectedPrefab()
	if !ok || ed.Environment() == nil {
		return nil
	}
	net := dmnet.Detect(ed.Environment(), prefab)
	net.UseCaps = Route.UseCaps
	return net
}

func (t *ToolRoute) process() {
	for idx, coord := range t.path {
		fill, border := overlay.ColorToolAddTileFill, overlay.ColorToolAddTileBorder
		if idx == 0 || idx == len(t.path)-1 {
			fill, border = overlay.ColorToolFillAltTileFill, overlay.ColorToolFillAltTileBorder
		}
		ed.OverlayPushTile(coord, fill, border)
	}
}

func (t *ToolRoute) onStart(coord util.Point) {
	if _, ok := ed.SelectedPrefab(); !ok {
		return
	}
	t.dragging = true
	t.path = []util.Point{coord}
	t.visited = map[util.Point]bool{coord: true}
}

func (t *ToolRoute) onMove(coord util.Point) {
	if !t.dragging || len(t.path) == 0 {
		return
	}
	// The mouse could jump over several tiles, so the gap is filled with orthogonal steps.
	for _, step := range orthogonalSteps(t.path[len(t.path)-1], coord) {
		t.addStep(step)
	}
}

// addStep adds the tile to the path. Moving back to the previous tile removes the last one,
// so the user can "undo" the path by moving the mouse back.
func (t *ToolRoute) addStep(coord util.Point) {
	if n := len(t.path); n >= 2 && t.path[n-2] == coord {
		delete(t.visited, t.path[n-1])
		t.path = t.path[:n-1]
		return
	}
	if !ed.Dmm().HasTile(coord) {
		return
	}
	t.path = append(t.path, coord)
	t.visited[coord] = true
}

func (t *ToolRoute) onStop(util.Point) {
	if !t.dragging {
		return
	}
	t.dragging = false

	path := t.path
	t.path = nil
	t.visited = make(map[util.Point]bool)

	net := t.Network()
	if net == nil || len(path) == 0 {
		return
	}

	changedCoords, warnings := applyRoute(net, path)
	Route.LastWarnings = warnings
	Route.LastResult = fmt.Sprintf("%s: тайлов %d, изменено %d", net.Kind, len(path), len(changedCoords))

	if len(changedCoords) > 0 {
		ed.UpdateCanvasByCoords(changedCoords)
		ed.CommitChanges("Прокладка: " + net.Kind.String())
	}
	log.Printf("route applied: %s, tiles: %d, changed: %d, warnings: %d", net.Description(), len(path), len(changedCoords), len(warnings))
}

func (t *ToolRoute) OnDeselect() {
	t.dragging = false
	t.path = nil
	t.visited = make(map[util.Point]bool)
}

// RouteConnections returns connections for every tile of the path.
func RouteConnections(path []util.Point) map[util.Point]int {
	res := make(map[util.Point]int, len(path))
	for i, c := range path {
		res[c] |= 0
		if i > 0 {
			res[c] |= dm.DirBetween(c.X, c.Y, path[i-1].X, path[i-1].Y)
		}
		if i < len(path)-1 {
			res[c] |= dm.DirBetween(c.X, c.Y, path[i+1].X, path[i+1].Y)
		}
	}
	return res
}

func applyRoute(net *dmnet.Network, path []util.Point) (changed []util.Point, warnings []string) {
	conns := RouteConnections(path)

	// A single click places a piece only for simple objects.
	if len(path) == 1 && net.Kind != dmnet.KindLine && net.Kind != dmnet.KindSmartCable {
		return nil, []string{"Протяните мышью хотя бы на соседний тайл"}
	}

	done := make(map[util.Point]bool, len(path))
	for _, coord := range path {
		if done[coord] {
			continue
		}
		done[coord] = true

		tile := ed.Dmm().GetTile(coord)
		isChanged, warning := net.Connect(tile, conns[coord])
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if isChanged {
			tile.InstancesRegenerate()
			changed = append(changed, coord)
		}
	}
	return changed, warnings
}

// orthogonalSteps returns tiles from the start (exclusive) to the end (inclusive) moving only orthogonally.
// The longer axis is walked first, it gives a natural looking path for fast mouse moves.
func orthogonalSteps(from, to util.Point) []util.Point {
	var steps []util.Point
	x, y := from.X, from.Y
	dx, dy := sign(to.X-x), sign(to.Y-y)
	for x != to.X || y != to.Y {
		remX, remY := abs(to.X-x), abs(to.Y-y)
		if remX >= remY && x != to.X {
			x += dx
		} else {
			y += dy
		}
		steps = append(steps, util.Point{X: x, Y: y, Z: from.Z})
	}
	return steps
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
