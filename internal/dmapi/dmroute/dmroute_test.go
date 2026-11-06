package dmroute

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// testMap builds a map from rows (top row first): '.' floor, '#' wall, 'D' door with access 12,
// 'A' door with access 12 or 13 (any), 'O' open door, 'B' bolted door, 'S' space, 'P' closed shutter.
func testMap(t *testing.T, rows ...string) *dmmap.Dmm {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, vars ...string) {
		mv := &dmvars.MutableVariables{}
		for i := 0; i+1 < len(vars); i += 2 {
			mv.Put(vars[i], vars[i+1])
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/area/station")
	add("/turf/simulated/floor", "density", "0")
	add("/turf/simulated/wall", "density", "1")
	add("/turf/space", "density", "0")
	add("/obj/machinery/door/airlock", "density", "1", "req_access", "null", "check_one_access", "TRUE", "locked", "FALSE")
	add("/obj/machinery/door/poddoor", "density", "1")

	dmmap.PrefabStorage.Free()
	prefab := func(path string, vars ...string) *dmmprefab.Prefab {
		v := dmvars.FromParent(env.Objects[path].Vars)
		for i := 0; i+1 < len(vars); i += 2 {
			v = dmvars.Set(v, vars[i], vars[i+1])
		}
		return dmmap.PrefabStorage.Get(path, v)
	}
	dmmap.BaseArea = prefab("/area/station")
	dmmap.BaseTurf = prefab("/turf/simulated/floor")

	h, w := len(rows), len(rows[0])
	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(w, h, 1)
	for row, line := range rows {
		y := h - row
		for col, ch := range line {
			tile := dmm.GetTile(util.Point{X: col + 1, Y: y, Z: 1})
			switch ch {
			case '#':
				tile.InstancesSet(nil)
				tile.InstancesAdd(prefab("/turf/simulated/wall"))
				tile.InstancesRegenerate()
			case 'S':
				tile.InstancesSet(nil)
				tile.InstancesAdd(prefab("/turf/space"))
				tile.InstancesRegenerate()
			case 'D':
				tile.InstancesAdd(prefab("/obj/machinery/door/airlock", "req_access", "list(12)"))
			case 'A':
				tile.InstancesAdd(prefab("/obj/machinery/door/airlock", "req_access", "list(12,13)"))
			case 'O':
				tile.InstancesAdd(prefab("/obj/machinery/door/airlock"))
			case 'B':
				tile.InstancesAdd(prefab("/obj/machinery/door/airlock", "locked", "1"))
			case 'P':
				tile.InstancesAdd(prefab("/obj/machinery/door/poddoor"))
			}
		}
	}
	return dmm
}

func TestDoorAccess(t *testing.T) {
	all := &Door{ReqAccess: []string{"1", "2"}, CheckOneAccess: false}
	any := &Door{ReqAccess: []string{"1", "2"}, CheckOneAccess: true}
	open := &Door{}
	if all.Opens(map[string]bool{"1": true}) || !all.Opens(map[string]bool{"1": true, "2": true}) {
		t.Fatal("all accesses")
	}
	if !any.Opens(map[string]bool{"2": true}) || any.Opens(map[string]bool{"3": true}) {
		t.Fatal("any access")
	}
	if !open.Opens(nil) {
		t.Fatal("open door")
	}
}

func TestReachAndPath(t *testing.T) {
	dmm := testMap(t,
		"#########",
		"#...D...#",
		"#...#...#",
		"#.O.#.B.#",
		"#########",
	)
	g := Build(dmm, 1)
	start := util.Point{X: 2, Y: 4, Z: 1} // Left room.
	target := util.Point{X: 8, Y: 4, Z: 1}

	// Without the access the right room is closed.
	res := g.Reach(start, map[string]bool{}, false)
	if g.Path(res, target) != nil {
		t.Fatal("the right room must be unreachable without access")
	}
	if len(res.ClosedDoors) != 1 || res.ClosedDoors[0] != (util.Point{X: 5, Y: 4, Z: 1}) {
		t.Fatalf("closed doors: %v", res.ClosedDoors)
	}

	// With the access 12 the door opens.
	res = g.Reach(start, map[string]bool{"12": true}, false)
	path := g.Path(res, target)
	if path == nil || path[0] != start || path[len(path)-1] != target {
		t.Fatalf("path: %v", path)
	}
	for i := 1; i < len(path); i++ {
		if abs(path[i].X-path[i-1].X)+abs(path[i].Y-path[i-1].Y) != 1 {
			t.Fatalf("path is not continuous: %v", path)
		}
	}
	// Bolted door is a wall.
	if g.DistAt(res, 7, 2) >= 0 {
		t.Fatal("bolted door must block")
	}
}

func TestSpaceAndShutters(t *testing.T) {
	dmm := testMap(t,
		"#####",
		"#.P.#",
		"#.S.#",
		"#####",
	)
	g := Build(dmm, 1)
	start := util.Point{X: 2, Y: 2, Z: 1}
	if g.DistAt(g.Reach(start, nil, false), 4, 2) >= 0 {
		t.Fatal("space and shutters must block")
	}
	if g.DistAt(g.Reach(start, nil, true), 4, 2) < 0 {
		t.Fatal("space is allowed")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestBayNestedAccess(t *testing.T) {
	// req_access = list("A", list("B", "C")): A is required and one of B or C.
	d := &Door{ReqAccess: []string{`"A"`, `list("B","C")`}}
	if d.Opens(map[string]bool{`"A"`: true}) {
		t.Fatal("group member is required")
	}
	if !d.Opens(map[string]bool{`"A"`: true, `"C"`: true}) {
		t.Fatal("any member of the group is enough")
	}
	if d.Opens(map[string]bool{`"B"`: true, `"C"`: true}) {
		t.Fatal("plain access is required")
	}
}
