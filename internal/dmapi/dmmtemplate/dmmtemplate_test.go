package dmmtemplate

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	for _, p := range []string{"/area/room", "/turf/floor", "/obj/table"} {
		mv := &dmvars.MutableVariables{}
		mv.Put("name", `"x"`)
		env.Objects[p] = &dmenv.Object{Path: p, Vars: mv.ToImmutable()}
	}
	dmmap.PrefabStorage.Free()

	var tiles []dmmap.Tile
	for x := 10; x <= 12; x++ {
		for y := 20; y <= 21; y++ {
			tile := dmmap.Tile{Coord: util.Point{X: x, Y: y, Z: 1}}
			tile.InstancesAdd(dmmap.PrefabStorage.Get("/area/room", dmvars.FromParent(env.Objects["/area/room"].Vars)))
			tile.InstancesAdd(dmmap.PrefabStorage.Get("/turf/floor", dmvars.FromParent(env.Objects["/turf/floor"].Vars)))
			if x == 11 {
				tile.InstancesAdd(dmmap.PrefabStorage.Get("/obj/table", dmvars.Set(dmvars.FromParent(env.Objects["/obj/table"].Vars), "name", `"стол"`)))
			}
			tiles = append(tiles, tile)
		}
	}

	tpl := Make("Комната: тест", tiles, Options{WithTurfs: true})
	if tpl.Width != 3 || tpl.Height != 2 || tpl.HasAreas() || !tpl.HasTurfs() {
		t.Fatalf("template: %+v", tpl)
	}

	dir := t.TempDir()
	path, err := Save(dir, tpl)
	if err != nil {
		t.Fatal(err)
	}
	list := List(dir)
	if len(list) != 1 || list[0].Name != "Комната: тест" || list[0].File != path {
		t.Fatalf("list: %+v", list)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// One unknown type is skipped and reported.
	loaded.Tiles[0].Prefabs = append(loaded.Tiles[0].Prefabs, Prefab{Path: "/obj/unknown"})
	result, missing := loaded.ToTiles(env)
	if len(missing) != 1 || missing[0] != "/obj/unknown" {
		t.Fatalf("missing: %v", missing)
	}
	if result[0].Coord != (util.Point{X: 1, Y: 1, Z: 1}) {
		t.Fatalf("the first tile must be bottom-left: %v", result[0].Coord)
	}
	tables := 0
	for _, tile := range result {
		for _, i := range tile.Instances() {
			if i.Prefab().Path() == "/obj/table" {
				tables++
				if v, _ := i.Prefab().Vars().Value("name"); v != `"стол"` || tile.Coord.X != 2 {
					t.Fatalf("table: %s at %v", v, tile.Coord)
				}
			}
		}
	}
	if tables != 2 {
		t.Fatalf("tables: %d", tables)
	}
}

func TestFileName(t *testing.T) {
	if got := FileName(`a/b:c*?`); got != "a_b_c__.json" {
		t.Fatal(got)
	}
	if got := FileName("  "); got != "template.json" {
		t.Fatal(got)
	}
}
