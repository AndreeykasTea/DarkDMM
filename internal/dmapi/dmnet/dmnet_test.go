package dmnet

import (
	"sort"
	"testing"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

const (
	n = dm.DirNorth
	s = dm.DirSouth
	e = dm.DirEast
	w = dm.DirWest
)

func testEnv() *dmenv.Dme {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, vars map[string]string) {
		mv := &dmvars.MutableVariables{}
		mv.Put("dir", "2")
		for k, v := range vars {
			mv.Put(k, v)
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/obj/structure/cable", map[string]string{"d1": "0", "d2": "1", "icon_state": `"0-1"`})
	add("/obj/structure/cable/yellow", map[string]string{"d1": "0", "d2": "1", "icon_state": `"0-1"`})
	add("/obj/machinery/atmospherics/pipe/simple/hidden/supply", nil)
	add("/obj/machinery/atmospherics/pipe/manifold/hidden/supply", nil)
	add("/obj/machinery/atmospherics/pipe/manifold4w/hidden/supply", nil)
	add("/obj/machinery/atmospherics/pipe/cap/hidden/supply", nil)
	add("/obj/machinery/atmospherics/unary/vent_pump/on", nil)
	add("/obj/structure/disposalpipe/segment", nil)
	add("/obj/structure/disposalpipe/junction", nil)
	add("/obj/structure/disposalpipe/trunk", nil)
	add("/obj/machinery/disposal", nil)
	dmmap.PrefabStorage.Free()
	return env
}

func initial(env *dmenv.Dme, path string) *dmmprefab.Prefab {
	return dmmap.PrefabStorage.Get(path, dmvars.FromParent(env.Objects[path].Vars))
}

func withVar(env *dmenv.Dme, path, name, value string) *dmmprefab.Prefab {
	return dmmap.PrefabStorage.Get(path, dmvars.Set(dmvars.FromParent(env.Objects[path].Vars), name, value))
}

func newTile() *dmmap.Tile {
	return &dmmap.Tile{Coord: util.Point{X: 5, Y: 5, Z: 1}}
}

func cableStates(t *dmmap.Tile, path string) []string {
	var res []string
	for _, i := range t.Instances() {
		if i.Prefab().Path() == path {
			res = append(res, i.Prefab().Vars().TextV("icon_state", ""))
		}
	}
	sort.Strings(res)
	return res
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPlanCable(t *testing.T) {
	cases := []struct {
		name     string
		existing []CablePiece
		dirs     int
		want     []string
	}{
		{"dead end", nil, e, []string{"0-4"}},
		{"straight", nil, e | w, []string{"4-8"}},
		{"bend", nil, n | e, []string{"1-4"}},
		{"t-junction on empty", nil, n | s | e, []string{"1-2", "1-4"}},
		{"branch from straight", []CablePiece{{1, 2}}, e, []string{"1-4"}},
		{"to knot", []CablePiece{{0, 2}}, n, []string{"0-1"}},
		{"already connected", []CablePiece{{1, 2}}, n | s, nil},
	}
	for _, c := range cases {
		var got []string
		for _, p := range PlanCable(c.existing, c.dirs) {
			got = append(got, p.State())
		}
		sort.Strings(got)
		if !equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCableRoute(t *testing.T) {
	env := testEnv()
	net := Detect(env, initial(env, "/obj/structure/cable/yellow"))
	if net.Kind != KindCable {
		t.Fatalf("kind: %v", net.Kind)
	}

	tile := newTile()
	// Other cable layer on the tile must be ignored.
	tile.InstancesAdd(withVar(env, "/obj/structure/cable", "icon_state", `"1-2"`))

	net.Connect(tile, e|w)
	if got := cableStates(tile, "/obj/structure/cable/yellow"); !equal(got, []string{"4-8"}) {
		t.Fatalf("got %v", got)
	}
	// Second pass over the same tile does nothing.
	if changed, _ := net.Connect(tile, e|w); changed {
		t.Fatal("must not duplicate cables")
	}
	// Branch to the north.
	net.Connect(tile, n)
	if got := cableStates(tile, "/obj/structure/cable/yellow"); !equal(got, []string{"1-4", "4-8"}) {
		t.Fatalf("got %v", got)
	}
}

func pipes(t *dmmap.Tile) (paths []string, dirs []int) {
	for _, i := range t.Instances() {
		if dm.IsPath(i.Prefab().Path(), "/obj/machinery/atmospherics/pipe") || dm.IsPath(i.Prefab().Path(), "/obj/structure/disposalpipe") {
			paths = append(paths, i.Prefab().Path())
			dirs = append(dirs, i.Prefab().Vars().IntV("dir", 2))
		}
	}
	return
}

func TestPlanPipe(t *testing.T) {
	cases := []struct {
		dirs int
		want PipePiece
	}{
		{n | s, PipePiece{"simple", s}},
		{e | w, PipePiece{"simple", e}},
		{n | e, PipePiece{"simple", n | e}},
		{n | s | e, PipePiece{"manifold", w}},
		{n | s | e | w, PipePiece{"manifold4w", s}},
		{e, PipePiece{"cap", e}},
	}
	for _, c := range cases {
		if got := PlanPipe(c.dirs, true); got != c.want {
			t.Errorf("PlanPipe(%d) = %v, want %v", c.dirs, got, c.want)
		}
		// The chosen piece must have exactly the requested connections.
		if got := PipeDirs(c.want.Kind, c.want.Dir); got != c.dirs {
			t.Errorf("PipeDirs(%v) = %d, want %d", c.want, got, c.dirs)
		}
	}
}

func TestPipeMerge(t *testing.T) {
	env := testEnv()
	net := Detect(env, initial(env, "/obj/machinery/atmospherics/pipe/simple/hidden/supply"))
	if net.Kind != KindPipe {
		t.Fatal(net.Kind)
	}

	tile := newTile()
	net.Connect(tile, n|s) // Vertical pipe.
	net.Connect(tile, e)   // Branch to the east -> manifold without the west side.
	paths, dirs := pipes(tile)
	if len(paths) != 1 || paths[0] != "/obj/machinery/atmospherics/pipe/manifold/hidden/supply" || dirs[0] != w {
		t.Fatalf("got %v %v", paths, dirs)
	}
	net.Connect(tile, w) // Crossing -> 4 way.
	paths, _ = pipes(tile)
	if len(paths) != 1 || paths[0] != "/obj/machinery/atmospherics/pipe/manifold4w/hidden/supply" {
		t.Fatalf("got %v", paths)
	}

	// Cap is replaced when the pipe continues.
	end := newTile()
	net.Connect(end, n)
	paths, _ = pipes(end)
	if len(paths) != 1 || paths[0] != "/obj/machinery/atmospherics/pipe/cap/hidden/supply" {
		t.Fatalf("got %v", paths)
	}
	net.Connect(end, s)
	paths, dirs = pipes(end)
	if len(paths) != 1 || paths[0] != "/obj/machinery/atmospherics/pipe/simple/hidden/supply" || PipeDirs("simple", dirs[0]) != n|s {
		t.Fatalf("got %v %v", paths, dirs)
	}
}

func TestPipeToVent(t *testing.T) {
	env := testEnv()
	net := Detect(env, initial(env, "/obj/machinery/atmospherics/pipe/simple/hidden/supply"))
	tile := newTile()
	tile.InstancesAdd(initial(env, "/obj/machinery/atmospherics/unary/vent_pump/on"))
	net.Connect(tile, e)
	paths, _ := pipes(tile)
	if len(paths) != 0 {
		t.Fatalf("pipe must not be placed under the vent: %v", paths)
	}
	if d := tile.Instances()[0].Prefab().Vars().IntV("dir", 2); d != e {
		t.Fatalf("vent must face the pipe, dir: %d", d)
	}
}

func TestDisposals(t *testing.T) {
	env := testEnv()
	net := Detect(env, initial(env, "/obj/structure/disposalpipe/segment"))
	if net.Kind != KindDisposal {
		t.Fatal(net.Kind)
	}

	bend := newTile()
	net.Connect(bend, n|e)
	paths, dirs := pipes(bend)
	if len(paths) != 1 || dirs[0] != n|e {
		t.Fatalf("bend: %v %v", paths, dirs)
	}

	junction := newTile()
	net.Connect(junction, n|s)
	net.Connect(junction, e)
	paths, dirs = pipes(junction)
	if len(paths) != 1 || paths[0] != "/obj/structure/disposalpipe/junction" {
		t.Fatalf("junction: %v", paths)
	}
	if got, _ := DisposalDirs(paths[0], dirs[0], ""); got != n|s|e {
		t.Fatalf("junction connections: %d", got)
	}

	unit := newTile()
	unit.InstancesAdd(initial(env, "/obj/machinery/disposal"))
	net.Connect(unit, w)
	paths, dirs = pipes(unit)
	if len(paths) != 1 || paths[0] != "/obj/structure/disposalpipe/trunk" || dirs[0] != w {
		t.Fatalf("trunk: %v %v", paths, dirs)
	}

	cross := newTile()
	net.Connect(cross, n|s|e)
	if changed, warning := net.Connect(cross, w); changed || warning == "" {
		t.Fatal("4-way disposal must be refused with a warning")
	}
}

func TestBayDisposals(t *testing.T) {
	env := testEnv()
	mv := &dmvars.MutableVariables{}
	mv.Put("dir", "2")
	mv.Put("icon_state", `"pipe-c"`)
	env.Objects["/obj/structure/disposalpipe/segment/bent"] = &dmenv.Object{Path: "/obj/structure/disposalpipe/segment/bent", Vars: mv.ToImmutable()}
	net := Detect(env, initial(env, "/obj/structure/disposalpipe/segment"))
	if !net.bayDisposals {
		t.Fatal("bay disposals must be detected")
	}

	for _, dirs := range []int{n | e, e | s, s | w, w | n} {
		tile := newTile()
		net.Connect(tile, dirs)
		i := tile.Instances()[0].Prefab()
		state := i.Vars().TextV("icon_state", "")
		dir := i.Vars().IntV("dir", 2)
		if state != "pipe-c" {
			t.Fatalf("bend %d: icon state %s", dirs, state)
		}
		if got, _ := DisposalDirs(i.Path(), dir, state); got != dirs {
			t.Fatalf("bend %d: connections %d (dir %d)", dirs, got, dir)
		}
	}

	// The bend becomes a junction, when a branch is added.
	tile := newTile()
	net.Connect(tile, n|e)
	net.Connect(tile, s)
	p := tile.Instances()[0].Prefab()
	if got, _ := DisposalDirs(p.Path(), p.Vars().IntV("dir", 2), p.Vars().TextV("icon_state", "")); got != n|e|s || p.Path() != disposalJunction {
		t.Fatalf("junction: %s %d", p.Path(), got)
	}
}
