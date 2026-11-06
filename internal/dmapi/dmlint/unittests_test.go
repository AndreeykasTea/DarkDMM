package dmlint

import (
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func bayLintMap(t *testing.T, levels int) (*dmenv.Dme, *dmmap.Dmm) {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, vars ...string) {
		mv := &dmvars.MutableVariables{}
		mv.Put("name", `"thing"`)
		for i := 0; i+1 < len(vars); i += 2 {
			mv.Put(vars[i], vars[i+1])
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/area/space")
	add("/area/room", "requires_power", "1")
	add("/area/free", "requires_power", "0")
	add("/turf/floor")
	add("/turf/simulated/open")
	add("/obj/structure/cable", "d1", "0", "d2", "1", "icon_state", `"0-1"`)
	add("/obj/structure/disposalpipe/segment", "dir", "0", "icon_state", `"pipe-s"`)
	add("/obj/structure/disposalpipe/trunk", "dir", "0", "icon_state", `"pipe-t"`)
	add("/obj/machinery/door/airlock", "req_access", "null", "width", "1")
	add("/obj/machinery/power/apc")
	add("/obj/machinery/power/terminal")
	add("/obj/machinery/camera", "c_tag", "null")
	add("/obj/structure/ladder", "allowed_directions", "32")
	add("/obj/structure/ladder/up", "allowed_directions", "16")
	add("/obj/effect/landmark/map_data", "height", "1")
	add("/datum/access/a", "id", `"ACCESS_A"`)
	add("/datum/map/test",
		"station_levels", "list(1,2)", "player_levels", "list(1,2)",
		"apc_test_exempt_areas", "list(/area/space = 7)",
		"area_coherency_test_exempt_areas", "list(/area/space)",
		"area_coherency_test_exempted_root_areas", "list()",
		"area_coherency_test_subarea_count", "list()",
		"ladder_check_exempt_ladders", "list()")

	dmmap.PrefabStorage.Free()
	dmmap.BaseArea = dmmap.PrefabStorage.Get("/area/space", dmvars.FromParent(env.Objects["/area/space"].Vars))
	dmmap.BaseTurf = dmmap.PrefabStorage.Get("/turf/floor", dmvars.FromParent(env.Objects["/turf/floor"].Vars))
	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(9, 9, levels)
	return env, dmm
}

func putZ(env *dmenv.Dme, dmm *dmmap.Dmm, x, y, z int, path string, vars ...string) {
	v := dmvars.FromParent(env.Objects[path].Vars)
	for i := 0; i+1 < len(vars); i += 2 {
		v = dmvars.Set(v, vars[i], vars[i+1])
	}
	dmm.GetTile(util.Point{X: x, Y: y, Z: z}).InstancesAdd(dmmap.PrefabStorage.Get(path, v))
}

func unitIssues(env *dmenv.Dme, dmm *dmmap.Dmm, cat Category) []Issue {
	return find(Check(env, dmm, Options{UnitTests: true, MapFile: "maps/test/test.dmm", SavesAsTGM: true}), cat)
}

func TestUnitTestsAreOnlyWarnings(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	putZ(env, dmm, 5, 5, 1, "/obj/structure/cable", "icon_state", `"1-2"`)
	for _, i := range Check(env, dmm, Options{UnitTests: true, MapFile: "maps/test/test.dmm", SavesAsTGM: true}) {
		if i.Severity == SeverityError {
			t.Fatalf("an old problem of the map must not open the window: %v", i)
		}
	}
}

func TestRewriteFormatWarning(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	if got := find(Check(env, dmm, Options{SavesAsTGM: true}), CatSaveFormat); len(got) != 0 {
		t.Fatalf("no rewrite: %v", got)
	}
	got := find(Check(env, dmm, Options{SavesAsTGM: true, RewritesFormat: true}), CatSaveFormat)
	if len(got) != 1 || got[0].Severity != SeverityWarning {
		t.Fatalf("rewrite: %v", got)
	}
}

func TestUnitCables(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	// d1 and d2 come from the icon state, variables are absent or stale on real maps.
	putZ(env, dmm, 5, 5, 1, "/obj/structure/cable", "icon_state", `"1-2"`)
	putZ(env, dmm, 5, 6, 1, "/obj/structure/cable", "icon_state", `"0-2"`)
	if got := unitIssues(env, dmm, CatTestCable); len(got) != 1 || !strings.Contains(got[0].Message, "south") {
		t.Fatalf("only the south end is open: %v", got)
	}

	// Vertical cables have their own order of numbers and go to another level.
	env, dmm = bayLintMap(t, 1)
	putZ(env, dmm, 5, 5, 1, "/obj/structure/cable", "icon_state", `"16-0"`)
	if got := unitIssues(env, dmm, CatTestCable); len(got) != 0 {
		t.Fatalf("a vertical cable is correct: %v", got)
	}

	// Overlap and the fix.
	env, dmm = bayLintMap(t, 1)
	putZ(env, dmm, 5, 5, 1, "/obj/structure/cable", "icon_state", `"16-0"`)
	putZ(env, dmm, 5, 5, 1, "/obj/structure/cable", "icon_state", `"16-0"`)
	got := unitIssues(env, dmm, CatTestCable)
	if len(got) != 1 || got[0].Fix == nil {
		t.Fatalf("overlap: %v", got)
	}
	got[0].Fix()
	if again := unitIssues(env, dmm, CatTestCable); len(again) != 0 {
		t.Fatalf("after fix: %v", again)
	}

	// A terminal needs a knot.
	env, dmm = bayLintMap(t, 1)
	putZ(env, dmm, 5, 5, 1, "/obj/machinery/power/terminal")
	if got := unitIssues(env, dmm, CatTestCable); len(got) != 1 {
		t.Fatalf("terminal: %v", got)
	}
}

func TestUnitDisposals(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	// Trunk (5,4) - straight (5,5) without a dir, like on real maps - trunk (5,6).
	putZ(env, dmm, 5, 4, 1, "/obj/structure/disposalpipe/trunk")
	putZ(env, dmm, 5, 5, 1, "/obj/structure/disposalpipe/segment")
	putZ(env, dmm, 5, 6, 1, "/obj/structure/disposalpipe/trunk")
	if got := unitIssues(env, dmm, CatTestDisposal); len(got) != 0 {
		t.Fatalf("a connected pipe: %v", got)
	}

	// An open end.
	env, dmm = bayLintMap(t, 1)
	putZ(env, dmm, 5, 4, 1, "/obj/structure/disposalpipe/trunk")
	putZ(env, dmm, 5, 5, 1, "/obj/structure/disposalpipe/segment")
	if got := unitIssues(env, dmm, CatTestDisposal); len(got) != 1 {
		t.Fatalf("an open end: %v", got)
	}

	// Two straight pipes with different dirs are not matched by the test: the north one is fixed.
	env, dmm = bayLintMap(t, 1)
	putZ(env, dmm, 5, 4, 1, "/obj/structure/disposalpipe/trunk")
	putZ(env, dmm, 5, 5, 1, "/obj/structure/disposalpipe/segment", "dir", "1")
	putZ(env, dmm, 5, 6, 1, "/obj/structure/disposalpipe/trunk")
	got := unitIssues(env, dmm, CatTestDisposal)
	if len(got) != 1 || got[0].Fix == nil {
		t.Fatalf("a wrong dir: %v", got)
	}
	got[0].Fix()
	if again := unitIssues(env, dmm, CatTestDisposal); len(again) != 0 {
		t.Fatalf("after fix: %v", again)
	}
}

func TestUnitAccess(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	putZ(env, dmm, 1, 1, 1, "/obj/machinery/door/airlock", "req_access", `list("ACCESS_A")`)
	putZ(env, dmm, 2, 1, 1, "/obj/machinery/door/airlock", "req_access", `list(list("ACCESS_A","ACCESS_TYPO"))`)
	putZ(env, dmm, 3, 1, 1, "/obj/machinery/door/airlock", "req_access", `newlist()`)
	putZ(env, dmm, 4, 1, 1, "/obj/machinery/door/airlock", "req_access", `list(ACCESS_DEFINE)`)
	got := unitIssues(env, dmm, CatTestAccess)
	if len(got) != 1 || !strings.Contains(got[0].Message, "ACCESS_TYPO") {
		t.Fatalf("issues: %v", got)
	}
}

func TestUnitDoorsCamerasLadders(t *testing.T) {
	env, dmm := bayLintMap(t, 2)
	putZ(env, dmm, 1, 1, 2, "/obj/effect/landmark/map_data", "height", "2")
	dmm.GetTile(util.Point{X: 3, Y: 3, Z: 2}).InstancesRemoveByPath("/turf")
	putZ(env, dmm, 3, 3, 2, "/turf/simulated/open")
	putZ(env, dmm, 3, 3, 2, "/obj/machinery/door/airlock")
	if got := unitIssues(env, dmm, CatTestDoor); len(got) != 1 {
		t.Fatalf("door: %v", got)
	}

	putZ(env, dmm, 4, 4, 1, "/obj/machinery/camera", "c_tag", `"A"`)
	putZ(env, dmm, 5, 5, 1, "/obj/machinery/camera", "c_tag", `"A"`)
	putZ(env, dmm, 6, 6, 1, "/obj/machinery/camera")
	putZ(env, dmm, 7, 7, 1, "/obj/machinery/camera")
	if got := unitIssues(env, dmm, CatTestCamera); len(got) != 1 {
		t.Fatalf("camera: %v", got)
	}

	// A correct pair of ladders: the upper one goes down from an open tile.
	dmm.GetTile(util.Point{X: 8, Y: 8, Z: 2}).InstancesRemoveByPath("/turf")
	putZ(env, dmm, 8, 8, 2, "/turf/simulated/open")
	putZ(env, dmm, 8, 8, 2, "/obj/structure/ladder")
	putZ(env, dmm, 8, 8, 1, "/obj/structure/ladder/up")
	if got := unitIssues(env, dmm, CatTestLadder); len(got) != 0 {
		t.Fatalf("correct ladders: %v", got)
	}
	// A ladder to nowhere.
	putZ(env, dmm, 2, 8, 1, "/obj/structure/ladder/up")
	if got := unitIssues(env, dmm, CatTestLadder); len(got) != 1 {
		t.Fatalf("ladder to nowhere: %v", got)
	}
}

func TestUnitAreas(t *testing.T) {
	env, dmm := bayLintMap(t, 1)
	room := func(x, y int, area string) {
		tile := dmm.GetTile(util.Point{X: x, Y: y, Z: 1})
		tile.InstancesRemoveByPath("/area")
		putZ(env, dmm, x, y, 1, area)
	}
	// Without an APC, in two parts.
	room(1, 1, "/area/room")
	room(5, 5, "/area/room")
	room(7, 7, "/area/free")

	if got := unitIssues(env, dmm, CatTestAPC); len(got) != 1 || !strings.Contains(got[0].Message, "/area/room") {
		t.Fatalf("only /area/room needs an APC: %v", got)
	}
	got := unitIssues(env, dmm, CatTestArea)
	if len(got) != 1 || !strings.Contains(got[0].Message, "/area/room") {
		t.Fatalf("incoherent: %v", got)
	}

	putZ(env, dmm, 1, 1, 1, "/obj/machinery/power/apc")
	if got := unitIssues(env, dmm, CatTestAPC); len(got) != 0 {
		t.Fatalf("with an APC: %v", got)
	}

	// An APC in an exempted area.
	putZ(env, dmm, 2, 2, 1, "/obj/machinery/power/apc")
	if got := unitIssues(env, dmm, CatTestAPC); len(got) != 1 {
		t.Fatalf("an APC in space: %v", got)
	}
}
