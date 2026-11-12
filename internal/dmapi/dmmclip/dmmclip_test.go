package dmmclip

import (
	"testing"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type fakeUnderFloor struct {
	hiddenTile util.Point
}

func (f fakeUnderFloor) IsTileUnderFloorHidden(tile *dmmap.Tile) bool {
	return tile.Coord == f.hiddenTile
}

func (f fakeUnderFloor) IsHiddenUnderFloor(tile *dmmap.Tile, p *dmmprefab.Prefab) bool {
	return tile.Coord == f.hiddenTile && dm.IsPath(p.Path(), "/obj/structure/cable")
}

func TestCopySkipsHiddenAndMarksTiles(t *testing.T) {
	dmmap.PrefabStorage.Free()
	prefab := func(path string) *dmmprefab.Prefab {
		mv := &dmvars.MutableVariables{}
		return dmmap.PrefabStorage.Get(path, dmvars.FromParent(mv.ToImmutable()))
	}
	shown, hidden := util.Point{X: 1, Y: 1, Z: 1}, util.Point{X: 2, Y: 1, Z: 1}
	dmmap.BaseArea = prefab("/area/space")
	dmmap.BaseTurf = prefab("/turf/simulated/floor/plasteel")
	defer func() { dmmap.BaseArea, dmmap.BaseTurf = nil, nil }()
	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(2, 1, 1)
	for _, coord := range []util.Point{shown, hidden} {
		dmm.GetTile(coord).InstancesAdd(prefab("/obj/structure/cable"))
	}

	clip := New()
	filter := dm.NewPathsFilter(nil)
	clip.Copy(filter, dmm, []util.Point{shown, hidden}, fakeUnderFloor{hiddenTile: hidden})
	data := clip.Buffer()

	for _, tile := range data.Buffer {
		hasCable := false
		for _, i := range tile.Instances() {
			if dm.IsPath(i.Prefab().Path(), "/obj/structure/cable") {
				hasCable = true
			}
		}
		if hasCable != (tile.Coord == shown) {
			t.Errorf("%v: cable copied %t", tile.Coord, hasCable)
		}
	}
	// The paste keeps cables of the target only where the mapper didn't see them on the copy.
	if data.UnderFloorHidden[shown] || !data.UnderFloorHidden[hidden] {
		t.Errorf("under floor hidden: %v", data.UnderFloorHidden)
	}
}
