package tools

import (
	"sort"
	"testing"

	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/util"
)

func pt(x, y int) util.Point {
	return util.Point{X: x, Y: y, Z: 1}
}

// drag simulates the mouse drag of the route tool through provided tiles.
func drag(t *ToolRoute, points ...util.Point) {
	t.onStart(points[0])
	for _, p := range points[1:] {
		t.onMove(p)
	}
	t.onStop(points[len(points)-1])
}

func cableStatesAt(f *fakeEditor, x, y int) []string {
	var res []string
	for _, p := range f.objectsAt(x, y, "/obj/structure/cable") {
		res = append(res, p.Vars().TextV("icon_state", ""))
	}
	sort.Strings(res)
	return res
}

func TestOrthogonalSteps(t *testing.T) {
	steps := orthogonalSteps(pt(1, 1), pt(4, 2))
	if len(steps) != 4 || steps[len(steps)-1] != pt(4, 2) {
		t.Fatalf("steps: %v", steps)
	}
	for i := 1; i < len(steps); i++ {
		dx, dy := abs(steps[i].X-steps[i-1].X), abs(steps[i].Y-steps[i-1].Y)
		if dx+dy != 1 {
			t.Fatalf("non orthogonal step: %v", steps)
		}
	}
}

func TestRouteCableL(t *testing.T) {
	f := newFakeEditor(10, 10, map[string]map[string]string{
		"/obj/structure/cable": {"d1": "0", "d2": "1", "icon_state": `"0-1"`},
	})
	f.selected = f.prefab("/obj/structure/cable", nil)
	tool := newRoute()

	// Fast mouse move from (2,2) to (5,4): the gap is filled with orthogonal steps.
	drag(tool, pt(2, 2), pt(5, 4))

	if len(f.commits) != 1 {
		t.Fatalf("commits: %v", f.commits)
	}
	// Start is a knot, end is a knot.
	if got := cableStatesAt(f, 2, 2); len(got) != 1 || got[0] != "0-4" {
		t.Fatalf("start: %v", got)
	}
	if got := cableStatesAt(f, 5, 4); len(got) != 1 || got[0] != "0-2" {
		t.Fatalf("end: %v", got)
	}
	// Every tile of the path has exactly one cable piece, and pieces are connected to neighbours.
	total := 0
	for x := 1; x <= 10; x++ {
		for y := 1; y <= 10; y++ {
			total += len(cableStatesAt(f, x, y))
		}
	}
	if total != 6 {
		t.Fatalf("expected 6 cable pieces, got %d", total)
	}
}

func TestRouteBacktrack(t *testing.T) {
	f := newFakeEditor(10, 10, map[string]map[string]string{
		"/obj/structure/cable": {"d1": "0", "d2": "1", "icon_state": `"0-1"`},
	})
	f.selected = f.prefab("/obj/structure/cable", nil)
	tool := newRoute()

	// Go right 3 tiles, then back by one: the last tile must be removed from the path.
	drag(tool, pt(2, 2), pt(3, 2), pt(4, 2), pt(5, 2), pt(4, 2))

	if got := cableStatesAt(f, 5, 2); len(got) != 0 {
		t.Fatalf("backtracked tile must be empty: %v", got)
	}
	if got := cableStatesAt(f, 4, 2); len(got) != 1 || got[0] != "0-8" {
		t.Fatalf("new end: %v", got)
	}
}

func TestRoutePipeTJunction(t *testing.T) {
	f := newFakeEditor(10, 10, map[string]map[string]string{
		"/obj/machinery/atmospherics/pipe/simple/hidden/supply":     nil,
		"/obj/machinery/atmospherics/pipe/manifold/hidden/supply":   nil,
		"/obj/machinery/atmospherics/pipe/manifold4w/hidden/supply": nil,
		"/obj/machinery/atmospherics/pipe/cap/hidden/supply":        nil,
	})
	f.selected = f.prefab("/obj/machinery/atmospherics/pipe/simple/hidden/supply", nil)
	tool := newRoute()

	// A vertical pipe and then a branch from its middle to the east.
	drag(tool, pt(5, 2), pt(5, 6))
	drag(tool, pt(5, 4), pt(8, 4))

	at := f.objectsAt(5, 4, "/obj/machinery/atmospherics/pipe")
	if len(at) != 1 || at[0].Path() != "/obj/machinery/atmospherics/pipe/manifold/hidden/supply" {
		t.Fatalf("junction: %v", at)
	}
	if got := dmnet.PipeDirs("manifold", at[0].Vars().IntV("dir", 2)); got != 1|2|4 {
		t.Fatalf("junction connections: %d", got)
	}
	// Open ends get caps.
	if end := f.objectsAt(8, 4, "/obj/machinery/atmospherics/pipe"); len(end) != 1 || end[0].Path() != "/obj/machinery/atmospherics/pipe/cap/hidden/supply" {
		t.Fatalf("end: %v", end)
	}
	if len(f.commits) != 2 {
		t.Fatalf("commits: %v", f.commits)
	}
}
