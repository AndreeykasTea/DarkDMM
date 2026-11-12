package dmmap

import (
	"testing"

	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
)

func underfloorPrefab(path string, kv ...string) *dmmprefab.Prefab {
	mv := &dmvars.MutableVariables{}
	for i := 0; i+1 < len(kv); i += 2 {
		mv.Put(kv[i], kv[i+1])
	}
	return PrefabStorage.Get(path, dmvars.FromParent(mv.ToImmutable()))
}

func TestHidesUnderFloor(t *testing.T) {
	PrefabStorage.Free()
	cases := []struct {
		prefab *dmmprefab.Prefab
		want   bool
	}{
		{underfloorPrefab("/obj/structure/cable/green", "level", "1"), true},
		{underfloorPrefab("/obj/structure/disposalpipe/segment", "level", "1"), true},
		{underfloorPrefab("/obj/machinery/atmospherics/pipe/simple/hidden/supply", "level", "1"), true},
		{underfloorPrefab("/obj/machinery/atmospherics/pipe/simple/visible", "level", "2"), false},
		{underfloorPrefab("/obj/machinery/power/terminal", "level", "1"), true},
		// The hide proc of this pipe changes only its icon state.
		{underfloorPrefab("/obj/machinery/atmospherics/pipe/vent", "level", "1"), false},
		// Vents have level = 1, but they stay on the floor in the game.
		{underfloorPrefab("/obj/machinery/atmospherics/unary/vent_pump", "level", "1"), false},
		{underfloorPrefab("/obj/structure/table"), false},
		{underfloorPrefab("/obj/structure/cable_coil_thing"), false},
		// Paradise objects with the undertile element.
		{underfloorPrefab("/obj/machinery/navbeacon", "level", "1"), true},
		{underfloorPrefab("/obj/machinery/navbeacon/invisible", "level", "1", "theoretical", "1"), false},
		{underfloorPrefab("/obj/structure/safe/floor", "level", "1"), true},
		{underfloorPrefab("/obj/structure/logistics_pipe/junction", "level", "1"), true},
		{underfloorPrefab("/obj/item/storage/backpack/satchel_flat", "level", "1"), true},
		{underfloorPrefab("/obj/machinery/magnetic_module", "level", "2"), false},
		{underfloorPrefab("/obj/structure/safe", "level", "1"), false},
	}
	for _, c := range cases {
		if got := HidesUnderFloor(c.prefab); got != c.want {
			t.Errorf("%s: %t", c.prefab.Path(), got)
		}
	}
}

func TestHidesUnderParadiseWall(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorAccessibility
	defer func() { underfloorModelCurrent = underfloorUnknown }()

	visible := underfloorPrefab("/obj/machinery/atmospherics/pipe/simple/visible", "level", "2")
	if HidesUnder(visible, "/turf/simulated/floor/plasteel") {
		t.Error("a visible pipe on a floor")
	}
	if !HidesUnder(visible, "/turf/simulated/wall/r_wall") {
		t.Error("a visible pipe in a wall is hidden in the game")
	}
	if HidesUnder(underfloorPrefab("/obj/structure/table"), "/turf/simulated/wall") {
		t.Error("a table")
	}
	if HidesUnder(underfloorPrefab("/obj/structure/cable"), "") {
		t.Error("nothing covers the tile")
	}

	underfloorModelCurrent = underfloorBay
	if HidesUnder(visible, "/turf/simulated/wall") {
		t.Error("Baystation pipes don't change their level in walls")
	}
}

func TestCoversUnderfloorBay(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorBay
	defer func() { underfloorModelCurrent = underfloorUnknown }()

	cases := []struct {
		prefab *dmmprefab.Prefab
		want   bool
	}{
		{underfloorPrefab("/turf/simulated/floor/tiled", "initial_flooring", "/decl/flooring/tiling"), true},
		{underfloorPrefab("/turf/simulated/floor/plating", "initial_flooring", "null"), false},
		{underfloorPrefab("/turf/simulated/floor/exoplanet/desert", "initial_flooring", "null"), false},
		{underfloorPrefab("/turf/simulated/floor/fixed/alium", "initial_flooring", "null"), true},
		{underfloorPrefab("/turf/simulated/floor/ascent", "initial_flooring", "null"), false},
		{underfloorPrefab("/turf/simulated/wall"), true},
		{underfloorPrefab("/turf/unsimulated/floor"), true},
		{underfloorPrefab("/turf/space"), false},
		{underfloorPrefab("/turf/simulated/open"), false},
		{underfloorPrefab("/turf/simulated/mineral"), false},
	}
	for _, c := range cases {
		if got := CoversUnderfloor(c.prefab); got != c.want {
			t.Errorf("%s: %t", c.prefab.Path(), got)
		}
	}
}

func TestCoversUnderfloorAccessibility(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorAccessibility
	defer func() { underfloorModelCurrent = underfloorUnknown }()

	cases := []struct {
		prefab *dmmprefab.Prefab
		want   bool
	}{
		{underfloorPrefab("/turf/simulated/floor/plasteel", "underfloor_accessibility", "0", "intact", "1"), true},
		{underfloorPrefab("/turf/simulated/floor/plating", "underfloor_accessibility", "2", "intact", "0"), false},
		{underfloorPrefab("/turf/simulated/floor/glass", "underfloor_accessibility", "1", "intact", "0"), false},
		// Water in Paradise doesn't change intact: this is why intact is not enough.
		{underfloorPrefab("/turf/simulated/floor/water", "underfloor_accessibility", "2", "intact", "1"), false},
	}
	for _, c := range cases {
		if got := CoversUnderfloor(c.prefab); got != c.want {
			t.Errorf("%s: %t", c.prefab.Path(), got)
		}
	}
}

func TestTileCover(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorBay
	defer func() { underfloorModelCurrent = underfloorUnknown }()

	tile := &Tile{}
	tile.InstancesAdd(underfloorPrefab("/obj/structure/cable"))
	if got := TileCover(tile); got != "" {
		t.Fatalf("no turf: %q", got)
	}
	tile.InstancesAdd(underfloorPrefab("/turf/simulated/floor/tiled", "initial_flooring", "/decl/flooring/tiling"))
	if got := TileCover(tile); got != "/turf/simulated/floor/tiled" {
		t.Fatalf("a floor: %q", got)
	}

	plating := &Tile{}
	plating.InstancesAdd(underfloorPrefab("/turf/simulated/floor/plating", "initial_flooring", "null"))
	if got := TileCover(plating); got != "" {
		t.Fatalf("a plating: %q", got)
	}

	underfloorModelCurrent = underfloorUnknown
	if got := TileCover(tile); got != "" {
		t.Fatalf("an unknown code base: %q", got)
	}
}

func TestCoversUnderfloorIntact(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorIntact
	defer func() { underfloorModelCurrent = underfloorUnknown }()

	if !CoversUnderfloor(underfloorPrefab("/turf/simulated/floor", "intact", "1")) {
		t.Error("a floor covers objects")
	}
	if CoversUnderfloor(underfloorPrefab("/turf/simulated/floor/plating", "intact", "0")) {
		t.Error("a plating doesn't cover objects")
	}
	if CoversUnderfloor(underfloorPrefab("/turf/space", "intact", "FALSE")) {
		t.Error("space doesn't cover objects")
	}
}

// A code base with an unknown turf model must show everything as before.
func TestCoversUnderfloorUnknown(t *testing.T) {
	PrefabStorage.Free()
	underfloorModelCurrent = underfloorUnknown
	if CoversUnderfloor(underfloorPrefab("/turf/floor")) {
		t.Error("an unknown model hides nothing")
	}
}
