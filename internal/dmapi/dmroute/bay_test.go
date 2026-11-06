package dmroute

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func bayVars(kv ...string) *dmvars.Variables {
	mv := &dmvars.MutableVariables{}
	for i := 0; i+1 < len(kv); i += 2 {
		mv.Put(kv[i], kv[i+1])
	}
	return mv.ToImmutable()
}

func bayPrefab(path string, kv ...string) *dmmprefab.Prefab {
	return dmmap.PrefabStorage.Get(path, dmvars.FromParent(bayVars(kv...)))
}

func TestAnalyzeBayTypes(t *testing.T) {
	dmmap.PrefabStorage.Free()

	cases := []struct {
		name   string
		prefab *dmmprefab.Prefab
		blocks bool
		door   bool
	}{
		// Foundation-19 has 1798 blast doors on site53: they must not be treated as usual doors.
		{"closed blast door", bayPrefab("/obj/machinery/door/blast/regular", "density", "1", "begins_closed", "1", "req_access", "null"), true, false},
		{"open blast door", bayPrefab("/obj/machinery/door/blast/regular/open", "density", "1", "begins_closed", "0", "req_access", "null"), false, false},
		{"airlock", bayPrefab("/obj/machinery/door/airlock", "density", "1", "req_access", `list("ACCESS_A")`), false, true},
		// Not a door despite the same beginning of the path.
		{"door control", bayPrefab("/obj/machinery/door_control", "density", "0"), false, false},
		{"full window", bayPrefab("/obj/structure/window/basic/full", "density", "1", "dir", "5"), true, false},
		{"directional window", bayPrefab("/obj/structure/window/basic", "density", "1", "dir", "1"), false, false},
		{"window spawner", bayPrefab("/obj/effect/wingrille_spawn/reinforced/full", "density", "1"), true, false},
	}
	for _, c := range cases {
		info := analyze(c.prefab)
		if info.blocks != c.blocks || (info.door != nil) != c.door {
			t.Errorf("%s: blocks %t, door %t", c.name, info.blocks, info.door != nil)
		}
	}
}

// A multi tile airlock occupies two tiles: nobody could walk around it through the second one.
func TestMultiTileAirlockBlocksBothTiles(t *testing.T) {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, kv ...string) {
		env.Objects[path] = &dmenv.Object{Path: path, Vars: bayVars(kv...)}
	}
	add("/area/station")
	add("/turf/simulated/floor", "density", "0")
	add("/turf/simulated/wall", "density", "1")
	dmmap.PrefabStorage.Free()
	prefab := func(path string, kv ...string) *dmmprefab.Prefab {
		v := dmvars.FromParent(env.Objects[path].Vars)
		for i := 0; i+1 < len(kv); i += 2 {
			v = dmvars.Set(v, kv[i], kv[i+1])
		}
		return dmmap.PrefabStorage.Get(path, v)
	}
	dmmap.BaseArea = prefab("/area/station")
	dmmap.BaseTurf = prefab("/turf/simulated/floor")
	add("/obj/machinery/door/airlock/multi_tile", "density", "1", "req_access", `list("ACCESS_A")`, "width", "2")

	// A wall with a double door in it:
	//   ###D D###  (the door is on x=4, its filler is on x=5)
	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(8, 3, 1)
	for x := 1; x <= 8; x++ {
		if x == 4 || x == 5 {
			continue
		}
		tile := dmm.GetTile(util.Point{X: x, Y: 2, Z: 1})
		tile.InstancesSet(nil)
		tile.InstancesAdd(prefab("/turf/simulated/wall"))
		tile.InstancesRegenerate()
	}
	dmm.GetTile(util.Point{X: 4, Y: 2, Z: 1}).InstancesAdd(prefab("/obj/machinery/door/airlock/multi_tile"))

	grid := Build(dmm, 1)
	start := util.Point{X: 5, Y: 1, Z: 1}
	if res := grid.Reach(start, map[string]bool{}, false); grid.DistAt(res, 5, 3) >= 0 {
		t.Fatal("the second half of the door must be closed without an access")
	}
	if res := grid.Reach(start, map[string]bool{`"ACCESS_A"`: true}, false); grid.DistAt(res, 5, 3) < 0 {
		t.Fatal("the door must be opened with the access")
	}

	// The same door in a vertical wall takes the tile to the north.
	dmm = &dmmap.Dmm{}
	dmm.SetMapSize(3, 8, 1)
	for y := 1; y <= 8; y++ {
		if y == 4 || y == 5 {
			continue
		}
		tile := dmm.GetTile(util.Point{X: 2, Y: y, Z: 1})
		tile.InstancesSet(nil)
		tile.InstancesAdd(prefab("/turf/simulated/wall"))
		tile.InstancesRegenerate()
	}
	dmm.GetTile(util.Point{X: 2, Y: 4, Z: 1}).InstancesAdd(prefab("/obj/machinery/door/airlock/multi_tile"))
	grid = Build(dmm, 1)
	if res := grid.Reach(util.Point{X: 1, Y: 5, Z: 1}, map[string]bool{}, false); grid.DistAt(res, 3, 5) >= 0 {
		t.Fatal("the vertical door must close the north tile")
	}
}
