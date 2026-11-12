package pmap

import (
	"sdmm/internal/app/render/bucket/level/chunk/unit"
)

func (p *PaneMap) ProcessUnit(u unit.Unit) bool {
	if p.app.PathsFilter().IsHiddenPath(u.Instance().Prefab().Path()) {
		return false
	}
	if p.IsCoverHidingAt(u.CoveredBy(), u.Instance().Coord()) {
		return false
	}
	p.locateHoveredInstance(u)
	return true
}

func (p *PaneMap) locateHoveredInstance(u unit.Unit) {
	mouseX, mouseY := p.canvasState.RelMouseX(), p.canvasState.RelMouseY()

	if u.ViewBounds().Contains(float32(mouseX), float32(mouseY)) {
		xOffset := int(float32(mouseX)-u.ViewBounds().X1) + u.Sprite().X1
		yOffset := u.Sprite().IconHeight() - 1 - int(float32(mouseY)-u.ViewBounds().Y1) + u.Sprite().Y1
		if u.Sprite().IsOpaqueAt(xOffset, yOffset) {
			p.tmpLastHoveredInstance = u.Instance()
		}
	}
}
