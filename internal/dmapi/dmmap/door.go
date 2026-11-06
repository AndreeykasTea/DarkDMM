package dmmap

import (
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/util"
)

const multiTileAirlockPath = "/obj/machinery/door/airlock/multi_tile"

// Objects which connect to doors (blend_objects of the Baystation door).
var doorBlendObjects = []string{"/obj/structure/wall_frame", "/obj/structure/window", "/obj/structure/grille", "/obj/effect/wingrille_spawn"}

// DoorExtraTiles returns additional tiles occupied by a door with the width more than 1 (Baystation multi tile airlocks).
// The game creates fillers on these tiles with the density and the opacity of the door.
func DoorExtraTiles(dmm *Dmm, coord util.Point, door *dmmprefab.Prefab) []util.Point {
	width := door.Vars().IntV("width", 1)
	if width <= 1 {
		return nil
	}

	dx, dy := 1, 0
	if multiTileGoesNorth(dmm, coord, door) {
		dx, dy = 0, 1
	}

	res := make([]util.Point, 0, width-1)
	for i := 1; i < width; i++ {
		res = append(res, util.Point{X: coord.X + dx*i, Y: coord.Y + dy*i, Z: coord.Z})
	}
	return res
}

// multiTileGoesNorth returns the direction of the door. Airlocks choose it by themselves in the game:
// update_connections looks for walls around and on_update_icon sets the dir, so a dir on the map means nothing for them.
func multiTileGoesNorth(dmm *Dmm, coord util.Point, door *dmmprefab.Prefab) bool {
	if !dm.IsPath(door.Path(), multiTileAirlockPath) {
		// get_adjusted_dir of the door.
		dir := dm.FixDir(door.Vars().IntV("dir", dm.DirSouth))
		return dir != dm.DirNorth && dir != dm.DirSouth
	}

	// The second tile is on the north or the east side, so these sides are checked one tile further.
	connected := func(dx, dy int) bool {
		p := util.Point{X: coord.X + dx, Y: coord.Y + dy, Z: coord.Z}
		return dmm.HasTile(p) && isDoorConnection(dmm.GetTile(p))
	}
	vertical := connected(0, 2) || connected(0, -1)
	horizontal := connected(2, 0) || connected(-1, 0)
	return vertical && !horizontal
}

func isDoorConnection(tile *Tile) bool {
	for _, instance := range tile.Instances() {
		path := instance.Prefab().Path()
		if dm.IsPath(path, "/turf/simulated/wall") || dm.IsPath(path, "/turf/simulated/shuttle/wall") {
			return true
		}
		for _, blend := range doorBlendObjects {
			if dm.IsPath(path, blend) {
				return true
			}
		}
	}
	return false
}
