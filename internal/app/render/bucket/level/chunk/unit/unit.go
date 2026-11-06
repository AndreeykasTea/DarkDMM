package unit

import (
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/util"
)

// Unit stores render information about specific object prefab on the map.
type Unit struct {
	sprite   *dmicon.Sprite
	instance *dmminstance.Instance

	// state and dir are used to render animated icons.
	state *dmicon.State
	dir   int

	layer      float32
	viewBounds util.Bounds

	r, g, b, a float32
}

func (u Unit) Sprite() *dmicon.Sprite {
	return u.sprite
}

// SpriteAt returns the sprite of the animation frame at the provided time (in ticks).
func (u Unit) SpriteAt(ticks float64) *dmicon.Sprite {
	if u.state == nil || !u.state.IsAnimated() {
		return u.sprite
	}
	return u.state.SpriteByFrame(u.dir, u.state.FrameAt(ticks))
}

// IsAnimated returns true if the unit icon has several frames.
func (u Unit) IsAnimated() bool {
	return u.state != nil && u.state.IsAnimated()
}

func (u Unit) Instance() *dmminstance.Instance {
	return u.instance
}

func (u Unit) Layer() float32 {
	return u.layer
}

func (u Unit) ViewBounds() util.Bounds {
	return u.viewBounds
}

func (u Unit) R() float32 {
	return u.r
}

func (u Unit) G() float32 {
	return u.g
}

func (u Unit) B() float32 {
	return u.b
}

func (u Unit) A() float32 {
	return u.a
}

func Make(x, y int, i *dmminstance.Instance, iconSize int) Unit {
	// All vars below are built-in and expected to exist.
	icon, _ := i.Prefab().Vars().Text("icon")
	iconState, _ := i.Prefab().Vars().Text("icon_state")
	dir, _ := i.Prefab().Vars().Int("dir")
	pixelX, _ := i.Prefab().Vars().Int("pixel_x")
	pixelY, _ := i.Prefab().Vars().Int("pixel_y")
	stepX, _ := i.Prefab().Vars().Int("step_x")
	stepY, _ := i.Prefab().Vars().Int("step_y")
	pixelW, _ := i.Prefab().Vars().Int("pixel_w")
	pixelZ, _ := i.Prefab().Vars().Int("pixel_z")

	var state *dmicon.State
	sp := dmicon.SpritePlaceholder()
	if st, err := dmicon.Cache.GetState(icon, iconState); err == nil {
		state = st
		sp = st.SpriteV(dir)
	}
	x1 := float32((x-1)*iconSize + pixelX + stepX + pixelW)
	y1 := float32((y-1)*iconSize + pixelY + stepY + pixelZ)
	x2 := x1 + float32(sp.IconWidth())
	y2 := y1 + float32(sp.IconHeight())
	r, g, b, a := parseColor(i.Prefab())

	return Unit{
		sp, i, state, dir, countLayer(i.Prefab()),
		util.Bounds{X1: x1, Y1: y1, X2: x2, Y2: y2},
		r, g, b, a,
	}
}

func parseColor(p *dmmprefab.Prefab) (r, g, b, a float32) {
	// Default rgba is white.
	r, g, b, a = 1, 1, 1, 1
	if color, _ := p.Vars().Text("color"); color != "" {
		r, g, b, _ = util.ParseColor(color).RGBA()
	}
	// Alpha is applied even without the color variable (BYOND behaviour).
	if alpha := p.Vars().FloatV("alpha", 255); alpha < 255 {
		a = max(0, alpha) / 255
	}
	return r, g, b, a
}

// countLayer returns the value of combined prefab vars: plane + Layer.
func countLayer(p *dmmprefab.Prefab) float32 {
	plane, _ := p.Vars().Float("plane")
	layer, _ := p.Vars().Float("layer")

	// Layers can have essentially effect values added onto them
	// We should clip them off to reduce the max possible layer to like 4999 (likely far lower)
	const backgroundLayer = 20_000
	const topdownLayer = 10_000
	const effectsLayer = 5000
	if layer > backgroundLayer {
		layer -= backgroundLayer
	}
	if layer > topdownLayer {
		layer -= topdownLayer
	}
	if layer > effectsLayer {
		layer -= effectsLayer
	}

	layer = plane*10_000 + layer*1000

	// When mobs are on the same Layer with object they are always rendered above them (BYOND specific stuff).
	if dm.IsPath(p.Path(), "/obj") {
		layer += 100
	} else if dm.IsPath(p.Path(), "/mob") {
		layer += 10
	}

	return layer
}
