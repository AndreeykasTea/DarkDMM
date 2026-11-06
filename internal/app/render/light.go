package render

import (
	"math"

	"sdmm/internal/app/render/brush"
	"sdmm/internal/app/render/lighting"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

var (
	lightSourceMarkerColor = util.MakeColor(1, .85, .2, .9)
	lightSourceOffColor    = util.MakeColor(1, .2, .2, 1)
)

// batchLighting draws the lighting preview over the current level.
// Every tile is drawn as a quad with colors from its corners, multiplied with the canvas. Exactly like in the game.
func (r *Render) batchLighting(viewBounds util.Bounds) {
	settings := lighting.Current
	if settings == nil || !settings.Enabled || r.dmm == nil {
		return
	}

	res := r.lighting.Result(r.dmm, r.Camera.Level)
	if res == nil {
		return
	}

	iconSize := float32(dmmap.WorldIconSize)
	if iconSize <= 0 {
		return
	}

	x1 := max(1, int(math.Floor(float64(viewBounds.X1/iconSize)))+1)
	y1 := max(1, int(math.Floor(float64(viewBounds.Y1/iconSize)))+1)
	x2 := min(res.W, int(math.Ceil(float64(viewBounds.X2/iconSize))))
	y2 := min(res.H, int(math.Ceil(float64(viewBounds.Y2/iconSize))))

	ambient := clamp01(float32(settings.Ambient) / 100)
	intensity := max(0, float32(settings.Intensity)/100)

	corner := func(cx, cy int, br, bg, bb float32) [4]float32 {
		r, g, b := res.Corner(cx, cy)
		// Negative lights can't make the darkness darker than it is.
		r, g, b = max(r, 0)*intensity+br, max(g, 0)*intensity+bg, max(b, 0)*intensity+bb
		// Normalize the over-bright color, the same way the game does.
		if mx := max(r, g, b); mx > 1 {
			r, g, b = r/mx, g/mx, b/mx
		}
		return [4]float32{
			ambient + (1-ambient)*r,
			ambient + (1-ambient)*g,
			ambient + (1-ambient)*b,
			1,
		}
	}

	brush.SetBlendMode(brush.BlendMultiply)
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			if res.IsFullbright(x, y) {
				continue
			}
			br, bg, bb := res.TileBase(x, y)

			tx1 := float32(x-1) * iconSize
			ty1 := float32(y-1) * iconSize
			brush.RectGradientV(tx1, ty1, tx1+iconSize, ty1+iconSize,
				corner(x-1, y-1, br, bg, bb), // bottom-left
				corner(x, y-1, br, bg, bb),   // bottom-right
				corner(x-1, y, br, bg, bb),   // top-left
				corner(x, y, br, bg, bb),     // top-right
			)
		}
	}
	brush.SetBlendMode(brush.BlendNormal)

	if settings.ShowSources {
		r.batchLightSources(viewBounds, iconSize)
	}
}

// batchLightSources draws markers in places of the light sources.
func (r *Render) batchLightSources(viewBounds util.Bounds, iconSize float32) {
	for _, src := range r.lighting.Sources() {
		cx := (float32(src.X) - .5 + src.OffsetX) * iconSize
		cy := (float32(src.Y) - .5 + src.OffsetY) * iconSize
		if !viewBounds.Contains(cx, cy) {
			continue
		}
		const half = 3
		brush.RectFilled(cx-half, cy-half, cx+half, cy+half, util.MakeColor(src.R, src.G, src.B, 1))
		brush.Rect(cx-half, cy-half, cx+half, cy+half, lightSourceMarkerColor)
	}
	// Lamps without the power are shown with a red frame.
	for _, src := range r.lighting.Unpowered() {
		cx := (float32(src.X) - .5) * iconSize
		cy := (float32(src.Y) - .5) * iconSize
		if !viewBounds.Contains(cx, cy) {
			continue
		}
		const half = 3
		brush.Rect(cx-half, cy-half, cx+half, cy+half, lightSourceOffColor)
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
