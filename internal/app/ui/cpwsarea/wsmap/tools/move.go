package tools

import (
	"sdmm/internal/app/prefs"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/imguiext"
	"sdmm/internal/util"
	"strconv"

	"github.com/SpaiR/imgui-go"
)

// ToolMove can be used move a single object.
type ToolMove struct {
	tool
	instance        *dmminstance.Instance
	lastTile        *dmmap.Tile
	lastMouseCoords imgui.Vec2
	lastOffsets     [2]int
}

func (ToolMove) Name() string {
	return TNMove
}

func newMove() *ToolMove {
	return &ToolMove{}
}

func (t *ToolMove) Stale() bool {
	return t.instance == nil
}

func (ToolMove) AltBehaviour() bool {
	return false
}

func (t *ToolMove) onStart(util.Point) {
	if hoveredInstance := ed.HoveredInstance(); hoveredInstance != nil {
		ed.InstanceSelect(hoveredInstance)
		t.instance = hoveredInstance
		t.lastMouseCoords = imgui.MousePos()
		vars := t.instance.Prefab().Vars()
		switch ed.Prefs().Editor.NudgeMode {
		case prefs.SaveNudgeModePixel:
			t.lastOffsets = [2]int{vars.IntV("pixel_x", 0), vars.IntV("pixel_y", 0)}
		case prefs.SaveNudgeModeStep:
			t.lastOffsets = [2]int{vars.IntV("step_x", 0), vars.IntV("step_y", 0)}
		case prefs.SaveNudgeModePixelAlt:
			t.lastOffsets = [2]int{vars.IntV("pixel_w", 0), vars.IntV("pixel_z", 0)}
		}
	}
}

func (t *ToolMove) process() {
	if t.instance == nil || !imguiext.IsShiftDown() {
		return
	}
	xAxis := "pixel_x"
	yAxis := "pixel_y"
	if ed.Prefs().Editor.NudgeMode == prefs.SaveNudgeModeStep {
		xAxis = "step_x"
		yAxis = "step_y"
	} else if ed.Prefs().Editor.NudgeMode == prefs.SaveNudgeModePixelAlt {
		xAxis = "pixel_w"
		yAxis = "pixel_z"
	}
	origPrefab := t.instance.Prefab()
	mouseCoords := imgui.MousePos()
	offsetX := (mouseCoords.X - t.lastMouseCoords.X) / ed.ZoomLevel()
	offsetY := (t.lastMouseCoords.Y - mouseCoords.Y) / ed.ZoomLevel()

	newX, newY := t.lastOffsets[0]+int(offsetX), t.lastOffsets[1]+int(offsetY)
	// Nothing to do while the mouse stays in place. Otherwise a new prefab would be created every frame.
	if origPrefab.Vars().IntV(xAxis, 0) == newX && origPrefab.Vars().IntV(yAxis, 0) == newY {
		return
	}

	newVars := dmvars.Set(origPrefab.Vars(), xAxis, strconv.Itoa(newX))
	newVars = dmvars.Set(newVars, yAxis, strconv.Itoa(newY))
	// The prefab must be in the storage, like all other prefabs on the map.
	t.instance.SetPrefab(dmmap.PrefabStorage.Get(origPrefab.Path(), newVars))

	ed.UpdateCanvasByCoords([]util.Point{t.instance.Coord()})
}

func (t *ToolMove) onMove(coord util.Point) {
	if t.instance == nil || imguiext.IsShiftDown() {
		return
	}

	prefab := t.instance.Prefab()
	oldCoord := t.instance.Coord()
	if t.lastTile != nil {
		t.lastTile.InstancesRegenerate() //should stop some issues
	}
	t.lastTile = ed.Dmm().GetTile(coord)
	ed.InstanceDelete(t.instance)
	t.lastTile.InstancesAdd(prefab)
	// The moved instance is the last one with the prefab: the tile could already have the same object.
	instances := t.lastTile.Instances()
	for idx := len(instances) - 1; idx >= 0; idx-- {
		if instances[idx].Prefab().Id() == prefab.Id() {
			t.instance = instances[idx]
			break
		}
	}
	t.lastTile.InstancesRegenerate()
	// The old tile must be redrawn as well, otherwise the moved object stays there visually.
	ed.UpdateCanvasByCoords([]util.Point{oldCoord, coord})
}

func (t *ToolMove) onStop(util.Point) {
	if t.instance == nil {
		return
	}
	//remove other turfs if we moved a turf
	if t.lastTile != nil {
		if dm.IsPath(t.instance.Prefab().Path(), "/turf") {
			for _, found := range t.lastTile.Instances() {
				if dm.IsPath(found.Prefab().Path(), "/turf") && found != t.instance {
					ed.InstanceDelete(found)
				}
			}
		}
	}
	t.instance = nil
	t.lastTile = nil
	ed.CommitChanges("Перемещение префаба")
}
