package dmnet

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
)

// bayEnv has disposal pipes like in the Baystation code: "dir = 0" for types.
func bayEnv() *dmenv.Dme {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path, icon string) {
		mv := &dmvars.MutableVariables{}
		mv.Put("dir", "0")
		mv.Put("icon_state", `"`+icon+`"`)
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/obj/structure/disposalpipe/segment", "pipe-s")
	add("/obj/structure/disposalpipe/segment/bent", "pipe-c")
	add("/obj/structure/disposalpipe/junction", "pipe-j1")
	add("/obj/structure/disposalpipe/trunk", "pipe-t")
	dmmap.PrefabStorage.Free()
	return env
}

func TestDisposalDirsBay(t *testing.T) {
	const up, down = "/obj/structure/disposalpipe/up", "/obj/structure/disposalpipe/down"
	const segment = "/obj/structure/disposalpipe/segment"
	const sort = "/obj/structure/disposalpipe/sortjunction"

	cases := []struct {
		name  string
		path  string
		dir   int
		state string
		want  int
	}{
		// 584 straight pipes and 10 trunks of site53 have no dir on the map at all.
		{"straight without a dir", segment, 0, "pipe-s", n | s},
		{"trunk without a dir", "/obj/structure/disposalpipe/trunk", 0, "pipe-t", s},
		{"corner", segment, n, "pipe-c", n | e},
		{"junction without a dir", "/obj/structure/disposalpipe/junction", 0, "pipe-j1", s | w | n},
		{"sort junction", sort, n, "pipe-j1s", n | e | s},
		{"flipped by the state", sort, n, "pipe-j2s", n | w | s},
		{"flipped by the type", sort + "/wildcard/flipped", n, "", n | w | s},
		{"up", up, e, "pipe-u", e},
		{"down", down, w, "pipe-d", w},
		{"tagger", "/obj/structure/disposalpipe/tagger", e, "pipe-tagger", e | w},
	}
	for _, c := range cases {
		if got, _ := DisposalDirs(c.path, c.dir, c.state); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// Before the fix a vertical pipe without a dir was treated as not connected at all and was replaced with a new piece.
func TestConnectToPipeWithoutDir(t *testing.T) {
	env := bayEnv()
	base := dmmap.PrefabStorage.Get("/obj/structure/disposalpipe/segment", dmvars.FromParent(env.Objects["/obj/structure/disposalpipe/segment"].Vars))
	net := Detect(env, base)

	tile := newTile()
	tile.InstancesAdd(base)

	changed, warning := net.Connect(tile, e)
	if !changed || warning != "" {
		t.Fatalf("changed %t, warning %q", changed, warning)
	}
	var piece *dmmprefab.Prefab
	for _, i := range tile.Instances() {
		piece = i.Prefab()
	}
	if piece.Path() != "/obj/structure/disposalpipe/junction" {
		t.Fatalf("a junction is expected: %s", piece.Path())
	}
	if got, _ := DisposalDirs(piece.Path(), dirOf(piece), piece.Vars().TextV("icon_state", "")); got != n|s|e {
		t.Fatalf("connections: %d", got)
	}
}

func TestBayStraightPipesLookSouthOrEast(t *testing.T) {
	env := bayEnv()
	base := dmmap.PrefabStorage.Get("/obj/structure/disposalpipe/segment", dmvars.FromParent(env.Objects["/obj/structure/disposalpipe/segment"].Vars))
	net := Detect(env, base)

	for _, c := range []struct{ dirs, want int }{{n, s}, {s, s}, {e, e}, {w, e}, {n | s, s}, {e | w, e}} {
		tile := newTile()
		if changed, _ := net.Connect(tile, c.dirs); !changed {
			t.Fatal("must be placed")
		}
		var piece *dmmprefab.Prefab
		for _, i := range tile.Instances() {
			piece = i.Prefab()
		}
		if dirOf(piece) != c.want {
			t.Errorf("connections %d: dir %d, want %d", c.dirs, dirOf(piece), c.want)
		}
		// The south is the default for the type, no reason to write it to the map.
		if c.want == s {
			for _, name := range piece.Vars().Iterate() {
				if name == "dir" {
					t.Errorf("connections %d: useless dir variable", c.dirs)
				}
			}
		}
	}
}
