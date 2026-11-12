package pmap

import (
	"sdmm/internal/app/render"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/util"
)

// Objects under a floor tile are not shown, like in the game. But it's impossible to map them blindly,
// and nothing invisible should be changed by accident. So they are visible when the mapper works with them.

// underFloorView is what the mapper works with at the moment. It's computed once per frame,
// because it's needed for every object under a floor on the screen.
type underFloorView struct {
	// all is true if objects are visible everywhere: an option, the route tool or such an object is selected.
	all bool
	// area is a selected area in tile coordinates: everything selected must be visible, since it'll be changed.
	area *util.Bounds
	// instanceTile is a tile of the object opened in the editor panels.
	instanceTile *util.Point
}

func makeUnderFloorView(showOption, routeTool bool, selectedPrefab *dmmprefab.Prefab, area *util.Bounds, instanceTile *util.Point) underFloorView {
	return underFloorView{
		all:          showOption || routeTool || (selectedPrefab != nil && dmmap.HidesUnderFloor(selectedPrefab)),
		area:         area,
		instanceTile: instanceTile,
	}
}

func (v underFloorView) shows(coord util.Point) bool {
	if v.all {
		return true
	}
	if v.instanceTile != nil && *v.instanceTile == coord {
		return true
	}
	return v.area != nil && v.area.Contains(float32(coord.X), float32(coord.Y))
}

// isUnderFloorHidden is the rule of visibility: floorHidden means that the floor is hidden by the paths filter.
func isUnderFloorHidden(cover string, floorHidden bool, view underFloorView, coord util.Point, revealed bool) bool {
	return cover != "" && !floorHidden && !revealed && !view.shows(coord)
}

func (p *PaneMap) updateUnderFloorView() {
	var selectedPrefab *dmmprefab.Prefab
	if prefab, ok := p.app.SelectedPrefab(); ok {
		selectedPrefab = prefab
	}

	var area *util.Bounds
	if grab, ok := tools.Selected().(*tools.ToolGrab); ok && grab.HasSelectedArea() {
		bounds := grab.Bounds()
		area = &bounds
	}

	var instanceTile *util.Point
	if instance, ok := p.app.SelectedInstance(); ok {
		coord := instance.Coord()
		instanceTile = &coord
	}

	p.underFloor = makeUnderFloorView(render.ShowUnderFloor, tools.IsSelected(tools.TNRoute), selectedPrefab, area, instanceTile)
}

// IsUnderFloorHidden returns true if an object on the tile covered by the provided turf is hidden at the moment.
func (p *PaneMap) IsUnderFloorHidden(cover string, coord util.Point) bool {
	if cover == "" {
		return false
	}
	return isUnderFloorHidden(cover, p.app.PathsFilter().IsHiddenPath(cover), p.underFloor, coord, p.editor.IsUnderFloorRevealed(coord))
}
