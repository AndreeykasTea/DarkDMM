package render

import (
	"math"

	"sdmm/internal/app/render/brush"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

// TileTint returns a color to tint the tile with. Used to show computed data (routes etc.) over the map.
type TileTint func(x, y int) (r, g, b, a float32, ok bool)

// SetTileTint sets the tint layer of the current level. Nil disables it.
func (r *Render) SetTileTint(tint TileTint) {
	r.tileTint = tint
}

// batchTileTint draws the tint only for visible tiles, so it's cheap even for big maps.
func (r *Render) batchTileTint(viewBounds util.Bounds) {
	if r.tileTint == nil || r.dmm == nil {
		return
	}
	iconSize := float32(dmmap.WorldIconSize)
	if iconSize <= 0 {
		return
	}
	x1 := max(1, int(math.Floor(float64(viewBounds.X1/iconSize)))+1)
	y1 := max(1, int(math.Floor(float64(viewBounds.Y1/iconSize)))+1)
	x2 := min(r.dmm.MaxX, int(math.Ceil(float64(viewBounds.X2/iconSize))))
	y2 := min(r.dmm.MaxY, int(math.Ceil(float64(viewBounds.Y2/iconSize))))

	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			cr, cg, cb, ca, ok := r.tileTint(x, y)
			if !ok {
				continue
			}
			tx, ty := float32(x-1)*iconSize, float32(y-1)*iconSize
			brush.RectFilledV(tx, ty, tx+iconSize, ty+iconSize, cr, cg, cb, ca)
		}
	}
}
