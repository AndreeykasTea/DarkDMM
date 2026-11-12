package dmmap

import (
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
)

// The game hides cables, hidden pipes etc. when they are under a floor tile. It's not done by layers:
// they are above the turf by their layer, but objects become invisible in the Initialize of the turf or the object.
//   - Baystation (Foundation-19): hide(hides_under_flooring() && !is_plating());
//   - TG and Paradise: the undertile element and the "underfloor_accessibility" variable of the turf;
//   - older TG forks: the "intact" variable of the turf.

type underfloorModel int

const (
	underfloorUnknown underfloorModel = iota
	underfloorAccessibility
	underfloorIntact
	underfloorBay
)

// Turfs of Baystation, which don't cover objects in the editor.
var bayOpenTurfs = []string{
	// They return true from is_plating.
	"/turf/simulated/open",
	"/turf/simulated/mineral",
	"/turf/simulated/ocean",
	"/turf/simulated/floor/ascent",
	// The game hides objects here too (is_plating of the turf is false), but there is no floor
	// above a cable in space: it's visible for a mapper, otherwise such a cable can't be noticed at all.
	"/turf/space",
}

const (
	bayFloorPath      = "/turf/simulated/floor"
	bayFloorFixedPath = "/turf/simulated/floor/fixed"
	bayVentPipePath   = "/obj/machinery/atmospherics/pipe/vent"

	// UNDERFLOOR_VISIBLE: objects under such a turf are visible, but can't be reached.
	underfloorVisibleValue = 1
)

func detectUnderfloorModel() underfloorModel {
	if turf, ok := TypeVars("/turf"); ok {
		if _, ok := turf.Value("underfloor_accessibility"); ok {
			return underfloorAccessibility
		}
		if _, ok := turf.Value("intact"); ok {
			return underfloorIntact
		}
	}
	if IsBaystation() {
		return underfloorBay
	}
	return underfloorUnknown
}

// IsBaystation returns true if the loaded code base is based on Baystation (Foundation-19 and other forks).
func IsBaystation() bool {
	return isBaystation
}

func detectBaystation() bool {
	atom, ok := TypeVars("/atom")
	if !ok {
		return false
	}
	_, ok = atom.Value("light_outer_range")
	return ok
}

// HidesUnderFloor returns true for objects, which the game hides under a floor tile.
func HidesUnderFloor(p *dmmprefab.Prefab) bool {
	path := p.Path()
	switch {
	case dm.IsPath(path, "/obj/structure/cable"),
		dm.IsPath(path, "/obj/structure/disposalpipe"):
		return true
	case dm.IsPath(path, bayVentPipePath):
		return false // Only its icon state is changed by the hide proc, the vent stays visible.
	case dm.IsPath(path, "/obj/machinery/atmospherics/pipe"),
		dm.IsPath(path, "/obj/machinery/power/terminal"):
		return p.Vars().IntV("level", 1) != 2 // Visible pipes have level = 2.
	}
	return false
}

// CoversUnderfloor returns true if the turf hides objects under it.
func CoversUnderfloor(turf *dmmprefab.Prefab) bool {
	vars := turf.Vars()
	path := turf.Path()

	switch underfloorModelCurrent {
	case underfloorAccessibility:
		return vars.IntV("underfloor_accessibility", 0) < underfloorVisibleValue
	case underfloorIntact:
		value, ok := vars.Value("intact")
		return ok && value != "0" && value != "FALSE" && value != dmvars.NullValue
	case underfloorBay:
		for _, open := range bayOpenTurfs {
			if dm.IsPath(path, open) {
				return false
			}
		}
		// is_plating of the floor: there is no flooring on the turf.
		if dm.IsPath(path, bayFloorPath) && !dm.IsPath(path, bayFloorFixedPath) {
			value, ok := vars.Value("initial_flooring")
			return ok && value != dmvars.NullValue && value != ""
		}
		return true // is_plating of other turfs is false.
	}
	return false
}

// TileCover returns a path of the turf, which hides objects under it on the tile, or an empty string.
func TileCover(tile *Tile) string {
	if underfloorModelCurrent == underfloorUnknown {
		return ""
	}
	// The last turf is the one the game uses.
	instances := tile.Instances()
	for i := len(instances) - 1; i >= 0; i-- {
		if p := instances[i].Prefab(); dm.IsPath(p.Path(), "/turf") {
			if CoversUnderfloor(p) {
				return p.Path()
			}
			return ""
		}
	}
	return ""
}
