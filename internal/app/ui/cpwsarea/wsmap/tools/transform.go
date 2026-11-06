package tools

import (
	"fmt"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

// TransformOp is a transformation of the selected area.
type TransformOp int

const (
	OpRotateCW TransformOp = iota
	OpRotateCCW
	OpFlipX
	OpFlipY
)

func (op TransformOp) String() string {
	switch op {
	case OpRotateCW:
		return "Поворот по часовой"
	case OpRotateCCW:
		return "Поворот против часовой"
	case OpFlipX:
		return "Отражение по горизонтали"
	case OpFlipY:
		return "Отражение по вертикали"
	}
	return ""
}

func (op TransformOp) dir(dir int) int {
	switch op {
	case OpRotateCW:
		return dm.DirTurnCW(dir)
	case OpRotateCCW:
		return dm.DirTurnCCW(dir)
	case OpFlipX:
		return dm.DirFlipX(dir)
	case OpFlipY:
		return dm.DirFlipY(dir)
	}
	return dir
}

func (op TransformOp) pixel(x, y int) (int, int) {
	switch op {
	case OpRotateCW:
		return y, -x
	case OpRotateCCW:
		return -y, x
	case OpFlipX:
		return -x, y
	case OpFlipY:
		return x, -y
	}
	return x, y
}

func (op TransformOp) isFlip() bool {
	return op == OpFlipX || op == OpFlipY
}

// localCoord transforms a local (0-based) coordinate inside the area w*h.
func (op TransformOp) localCoord(x, y, w, h int) (int, int) {
	switch op {
	case OpRotateCW:
		return y, w - 1 - x
	case OpRotateCCW:
		return h - 1 - y, x
	case OpFlipX:
		return w - 1 - x, y
	case OpFlipY:
		return x, h - 1 - y
	}
	return x, y
}

// Transform rotates or mirrors the selected area with all objects inside.
// Directions, pixel offsets, cable connections and directional subtypes of objects are transformed as well.
func (t *ToolGrab) Transform(op TransformOp) error {
	if !t.HasSelectedArea() {
		return fmt.Errorf("нет выделения")
	}

	dmm := ed.Dmm()
	env := ed.Environment()
	area := t.fillArea
	z := t.fillStart.Z
	x1, y1 := int(area.X1), int(area.Y1)
	w, h := int(area.X2-area.X1)+1, int(area.Y2-area.Y1)+1

	newW, newH := w, h
	if !op.isFlip() {
		newW, newH = h, w
	}
	if x1+newW-1 > dmm.MaxX || y1+newH-1 > dmm.MaxY {
		return fmt.Errorf("повёрнутая область не помещается в карту")
	}

	tiles := collectTiles(dmm, area, z)
	cache := make(map[uint64]*dmmprefab.Prefab)

	var updated []util.Point
	for _, tile := range tiles {
		ed.TileDelete(tile.Coord)
		updated = append(updated, tile.Coord)
	}

	for _, tile := range tiles {
		lx, ly := op.localCoord(tile.Coord.X-x1, tile.Coord.Y-y1, w, h)
		dst := util.Point{X: x1 + lx, Y: y1 + ly, Z: z}

		prefabs := make(dmmdata.Prefabs, 0, len(tile.Instances()))
		for _, i := range tile.Instances() {
			p, ok := cache[i.Prefab().Id()]
			if !ok {
				p = TransformPrefab(env, i.Prefab(), op)
				cache[i.Prefab().Id()] = p
			}
			prefabs = append(prefabs, p)
		}

		ed.TileReplace(dst, prefabs)
		updated = append(updated, dst)
	}

	t.fillArea = util.Bounds{X1: float32(x1), Y1: float32(y1), X2: float32(x1 + newW - 1), Y2: float32(y1 + newH - 1)}
	t.fillAreaInit = t.fillArea
	t.initTiles = collectTiles(dmm, t.fillArea, z)
	t.prevTiles = make(map[util.Point]dmmdata.Prefabs)
	t.mode = tSelectModeMoveArea

	ed.UpdateCanvasByCoords(updated)
	ed.CommitChanges(op.String())

	log.Printf("selection transformed: %s, area %dx%d", op, w, h)
	return nil
}

// TransformPrefab returns the prefab rotated or mirrored by the operation.
func TransformPrefab(env *dmenv.Dme, p *dmmprefab.Prefab, op TransformOp) *dmmprefab.Prefab {
	path := p.Path()
	if dm.IsPath(path, "/area") {
		return p
	}

	vars := p.Vars()
	newPath := transformPath(env, path, op)
	// A junction with its own icon state is mirrored by this state (see below), not by the type.
	if newPath != path && dm.IsPath(path, "/obj/structure/disposalpipe") && isOwn(vars, "icon_state") {
		newPath = path
	}
	if newPath != path {
		// Directional subtypes declare their own dir and offsets, so only map variables are moved to the new type.
		vars = rebaseVars(env, vars, newPath)
	}

	initial := func(name string, def string) string {
		if obj, ok := env.Objects[newPath]; ok {
			return obj.Vars.ValueV(name, def)
		}
		return def
	}
	setInt := func(name string, value int, def int) {
		initialValue := initial(name, strconv.Itoa(def))
		// dir = 0 of the type is the same as the south.
		if name == "dir" && initialValue == "0" {
			initialValue = strconv.Itoa(dm.DirSouth)
		}
		if strconv.Itoa(value) == initialValue {
			if isOwn(vars, name) {
				vars = dmvars.Delete(vars, name)
			}
			return
		}
		vars = dmvars.Set(vars, name, strconv.Itoa(value))
	}

	// Directional subtypes already have the proper dir. Mirrored junctions don't: only their side is changed.
	if (newPath == path || isDisposal(newPath)) && dirMatters(p) {
		dir := dm.FixDir(vars.IntV("dir", dm.DirSouth))
		newDir := op.dir(dir)
		// A Baystation disposal corner connects dir and clockwise(dir). After mirroring the clockwise
		// neighbour becomes counterclockwise, so the dir is taken from the other side of the corner.
		if op.isFlip() && dm.IsPath(newPath, "/obj/structure/disposalpipe/segment") && vars.TextV("icon_state", "") == "pipe-c" {
			newDir = op.dir(dm.DirTurnCW(dir))
		}
		// Unit tests of Baystation require straight pipes to have only the south or the east dir.
		if _, bay := env.Objects["/obj/structure/disposalpipe/segment/bent"]; bay &&
			dm.IsPath(newPath, "/obj/structure/disposalpipe/segment") && vars.TextV("icon_state", "") == "pipe-s" {
			switch newDir {
			case dm.DirNorth:
				newDir = dm.DirSouth
			case dm.DirWest:
				newDir = dm.DirEast
			}
		}
		if newDir != dir {
			setInt("dir", newDir, dm.DirSouth)
		}
	}

	// Baystation junctions are mirrored with the icon state, Paradise ones with the type (see transformPath).
	if op.isFlip() && newPath == path && dm.IsPath(newPath, "/obj/structure/disposalpipe") {
		state := vars.TextV("icon_state", "")
		if mirrored, ok := mirroredJunctionStates[state]; ok {
			if initial("icon_state", "") == strconv.Quote(mirrored) {
				vars = dmvars.Delete(vars, "icon_state")
			} else {
				vars = dmvars.Set(vars, "icon_state", strconv.Quote(mirrored))
			}
		}
	}

	if isOwn(vars, "pixel_x") || isOwn(vars, "pixel_y") {
		px, py := op.pixel(vars.IntV("pixel_x", 0), vars.IntV("pixel_y", 0))
		setInt("pixel_x", px, 0)
		setInt("pixel_y", py, 0)
	}

	if dm.IsPath(newPath, "/obj/structure/cable") {
		if piece, ok := dmnet.ParseCableState(vars.TextV("icon_state", "")); ok {
			piece.D1, piece.D2 = op.dir(piece.D1), op.dir(piece.D2)
			state := strconv.Quote(piece.State())
			if state == initial("icon_state", "") {
				if isOwn(vars, "icon_state") {
					vars = dmvars.Delete(vars, "icon_state")
				}
			} else {
				vars = dmvars.Set(vars, "icon_state", state)
			}
			// Baystation maps declare d1 and d2 as well, they must follow the icon state.
			if isOwn(vars, "d1") || isOwn(vars, "d2") {
				d1, d2 := min(piece.D1, piece.D2), max(piece.D1, piece.D2)
				vars = dmvars.Set(vars, "d1", strconv.Itoa(d1))
				vars = dmvars.Set(vars, "d2", strconv.Itoa(d2))
			}
		}
	}

	return dmmap.PrefabStorage.Get(newPath, vars)
}

// Junctions mirrored by the icon state.
var mirroredJunctionStates = map[string]string{
	"pipe-j1":  "pipe-j2",
	"pipe-j2":  "pipe-j1",
	"pipe-j1s": "pipe-j2s",
	"pipe-j2s": "pipe-j1s",
}

// transformPath handles types with a direction in the path and mirrored disposal junctions.
func transformPath(env *dmenv.Dme, path string, op TransformOp) string {
	exists := func(p string) bool {
		_, ok := env.Objects[p]
		return ok
	}

	// "/obj/machinery/firealarm/directional/north" -> ".../directional/east"
	if idx := strings.LastIndex(path, "/directional/"); idx >= 0 {
		name := path[idx+len("/directional/"):]
		if dir := dm.DirByName(name); dir != 0 {
			newPath := path[:idx] + "/directional/" + dm.DirName(op.dir(dir))
			if exists(newPath) {
				return newPath
			}
		}
	}

	// Mirroring of a junction changes its side, so the "reversed" type must be used.
	if op.isFlip() {
		for _, base := range []string{
			"/obj/structure/disposalpipe/junction",
			"/obj/structure/disposalpipe/sortjunction",
			"/obj/structure/disposalpipe/sortjunction/wildcard",
			"/obj/structure/disposalpipe/sortjunction/untagged",
		} {
			// Paradise calls the mirrored junction "reversed", Baystation - "mirrored" and "flipped".
			for _, mirrored := range []string{base + "/reversed", base + "/mirrored", base + "/flipped"} {
				switch {
				case path == base && exists(mirrored):
					return mirrored
				case path == mirrored && exists(base):
					return base
				}
			}
		}
	}

	return path
}

// rebaseVars moves variables declared on the map to the new type.
func rebaseVars(env *dmenv.Dme, vars *dmvars.Variables, newPath string) *dmvars.Variables {
	obj, ok := env.Objects[newPath]
	if !ok {
		return vars
	}
	res := dmvars.FromParent(obj.Vars)
	for _, name := range vars.Iterate() {
		// Placement of the directional type is defined by the type itself.
		if (name == "dir" && !isDisposal(newPath)) || name == "pixel_x" || name == "pixel_y" {
			continue
		}
		// Mirrored junction types define their own icon state.
		if name == "icon_state" && dm.IsPath(newPath, "/obj/structure/disposalpipe") {
			continue
		}
		if value, ok := vars.Value(name); ok {
			res = dmvars.Set(res, name, value)
		}
	}
	return res
}

func isDisposal(path string) bool {
	return dm.IsPath(path, "/obj/structure/disposalpipe")
}

func isOwn(vars *dmvars.Variables, name string) bool {
	for _, n := range vars.Iterate() {
		if n == name {
			return true
		}
	}
	return false
}

// iconDirsOf returns the number of dirs of the prefab icon state, or 0 if it's unknown.
// It's a variable to be replaced in tests, where icons are not available.
var iconDirsOf = func(p *dmmprefab.Prefab) int {
	icon, _ := p.Vars().Text("icon")
	state := p.Vars().TextV("icon_state", "")
	if s, err := dmicon.Cache.GetState(icon, state); err == nil {
		return s.Dirs
	}
	return 0
}

// dirMatters returns true if the dir of the object should be transformed.
// Objects with a single direction icon and a default dir are not touched, so floors, tables etc.
// won't get useless "dir" variables on the map.
func dirMatters(p *dmmprefab.Prefab) bool {
	if isOwn(p.Vars(), "dir") {
		return true
	}
	switch iconDirsOf(p) {
	case 1:
		return false
	case 0:
		return !dm.IsPath(p.Path(), "/turf")
	}
	return true
}
