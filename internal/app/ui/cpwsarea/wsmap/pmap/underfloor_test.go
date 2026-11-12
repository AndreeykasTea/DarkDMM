package pmap

import (
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func TestUnderFloorRule(t *testing.T) {
	const floor = "/turf/simulated/floor/tiled"
	coord := util.Point{X: 5, Y: 5, Z: 1}
	empty := underFloorView{}

	cases := []struct {
		name        string
		cover       string
		floorHidden bool
		view        underFloorView
		revealed    bool
		hidden      bool
	}{
		{"a plating covers nothing", "", false, empty, false, false},
		{"under a floor", floor, false, empty, false, true},
		// The usual way to look under a floor: the filter hides turfs.
		{"the floor is hidden by the filter", floor, true, empty, false, false},
		{"a jump from the search or the linter", floor, false, empty, true, false},
		{"the option or the route tool", floor, false, underFloorView{all: true}, false, false},
		{"inside of the selection", floor, false, underFloorView{area: &util.Bounds{X1: 4, Y1: 4, X2: 6, Y2: 6}, areaLevel: 1}, false, false},
		// Bounds of the selection have no Z level: another level of the map must not be shown.
		{"the selection on another level", floor, false, underFloorView{area: &util.Bounds{X1: 4, Y1: 4, X2: 6, Y2: 6}, areaLevel: 2}, false, true},
		{"on the border of the selection", floor, false, underFloorView{area: &util.Bounds{X1: 5, Y1: 5, X2: 5, Y2: 5}, areaLevel: 1}, false, false},
		{"out of the selection", floor, false, underFloorView{area: &util.Bounds{X1: 6, Y1: 6, X2: 8, Y2: 8}, areaLevel: 1}, false, true},
		{"the opened object", floor, false, underFloorView{instanceTile: &coord}, false, false},
		{"another opened object", floor, false, underFloorView{instanceTile: &util.Point{X: 1, Y: 1, Z: 1}}, false, true},
	}
	for _, c := range cases {
		if got := isUnderFloorHidden(c.cover, c.floorHidden, c.view, coord, c.revealed); got != c.hidden {
			t.Errorf("%s: hidden %t", c.name, got)
		}
	}
}

func TestMakeUnderFloorView(t *testing.T) {
	dmmap.PrefabStorage.Free()
	prefab := func(path string) *dmmprefab.Prefab {
		mv := &dmvars.MutableVariables{}
		return dmmap.PrefabStorage.Get(path, dmvars.FromParent(mv.ToImmutable()))
	}

	if makeUnderFloorView(false, false, nil, nil, 0, nil).all {
		t.Error("nothing is selected")
	}
	if !makeUnderFloorView(true, false, nil, nil, 0, nil).all {
		t.Error("the option")
	}
	if !makeUnderFloorView(false, true, nil, nil, 0, nil).all {
		t.Error("the route tool")
	}
	if !makeUnderFloorView(false, false, prefab("/obj/structure/cable/green"), nil, 0, nil).all {
		t.Error("a selected cable")
	}
	if makeUnderFloorView(false, false, prefab("/obj/structure/table"), nil, 0, nil).all {
		t.Error("a selected table")
	}
}
