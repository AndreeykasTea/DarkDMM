// Package lighting implements an approximation of the SS13 static lighting system.
// Every light source lights tile corners in its view. There are two models:
//   - TG-based lighting (Paradise uses it as well): the falloff is computed with the formula from the `falloff_at_coord` proc;
//   - Baystation lighting (Foundation-19): LUM_FALLOFF and APPLY_CORNER macros, then LIGHTING_MULT_FACTOR * sqrt(lum) in corners.
package lighting

import (
	"math"
)

const (
	// lightingHeight is the LIGHTING_HEIGHT define from the game.
	lightingHeight = 1
	// maxRange limits a range of a single light source to keep the computation fast.
	maxRange = 32
	// bayMultFactor is the LIGHTING_MULT_FACTOR define from the Baystation code.
	bayMultFactor = .9
)

// Source is a single light source on the map.
type Source struct {
	// X and Y are tile coordinates of the source (1-based).
	X, Y int
	// OffsetX and OffsetY are the visual offset of the light in tiles (wall lights are shifted by half a tile).
	OffsetX, OffsetY float32

	Range, Power float32
	R, G, B      float32

	// Angle of the light cone in degrees. Values >= 360 or <= 0 mean an omnidirectional light.
	Angle float32
	// DirAngle is a direction of the light cone in degrees (0 = north, 90 = east).
	DirAngle float32

	// Bay is true for the Baystation lighting model (Foundation-19 and other Bay forks):
	// Range is light_outer_range, Power is light_max_bright, Inner is light_inner_range.
	Bay   bool
	Inner float32
	Curve float32
	// DirX and DirY are components of light_direction for the Bay cone light (0, 0 for omnidirectional).
	DirX, DirY float32
}

// Grid describes tiles of the level, which are needed to compute lighting.
type Grid struct {
	W, H int

	// Opaque tiles block the light and are not lit by themselves (only their corners from the neighbours).
	Opaque []bool
	// Fullbright tiles are always lit (areas without static lighting, like space).
	Fullbright []bool
	// Base lighting of the tile area (base_lighting_alpha + base_lighting_color) in the [0..1] range.
	BaseR, BaseG, BaseB []float32

	// Open tiles let the light pass to the level below (Baystation, ZM_ALLOW_LIGHTING).
	Open []bool
	// Upper are connected levels above, from the nearest one (Baystation).
	Upper []UpperLevel
	// Unpowered are light sources, which don't shine because their area has no power.
	Unpowered []Source
}

// UpperLevel is a level above, which could shine through its open tiles.
type UpperLevel struct {
	Grid    *Grid
	Sources []Source
}

func (g *Grid) idx(x, y int) int {
	return (y-1)*g.W + (x - 1)
}

func (g *Grid) opaque(x, y int) bool {
	if x < 1 || y < 1 || x > g.W || y > g.H {
		return true
	}
	return g.Opaque[g.idx(x, y)]
}

// Result stores lighting values for every corner of the level.
// Corners are indexed from 0 to W (inclusive) by X and from 0 to H (inclusive) by Y.
// The corner with (cx, cy) indexes is a bottom-left corner of the tile (cx+1, cy+1).
type Result struct {
	W, H int

	// Lum stores RGB values of lighting for every corner (3 floats per corner).
	Lum []float32

	Fullbright []bool
	// Base lighting per tile (RGB, 3 floats per tile).
	Base []float32

	SourcesCount int
	// UnpoweredCount is a number of lamps on the level, which don't shine because of the area power.
	UnpoweredCount int
}

func (r *Result) cornerIdx(cx, cy int) int {
	return (cy*(r.W+1) + cx) * 3
}

// Corner returns RGB lighting of the corner.
func (r *Result) Corner(cx, cy int) (float32, float32, float32) {
	i := r.cornerIdx(cx, cy)
	return r.Lum[i], r.Lum[i+1], r.Lum[i+2]
}

// IsFullbright returns true if the tile with provided 1-based coords is always lit.
func (r *Result) IsFullbright(x, y int) bool {
	return r.Fullbright[(y-1)*r.W+(x-1)]
}

// TileBase returns base lighting of the tile with provided 1-based coords.
func (r *Result) TileBase(x, y int) (float32, float32, float32) {
	i := ((y-1)*r.W + (x - 1)) * 3
	return r.Base[i], r.Base[i+1], r.Base[i+2]
}

// Compute computes the lighting for the provided grid and sources.
// It's a pure function and doesn't touch any global state, so it's safe to run it in the background.
func Compute(grid *Grid, sources []Source) *Result {
	res := &Result{
		W:            grid.W,
		H:            grid.H,
		Lum:          make([]float32, (grid.W+1)*(grid.H+1)*3),
		Fullbright:   grid.Fullbright,
		Base:         make([]float32, grid.W*grid.H*3),
		SourcesCount: len(sources),

		UnpoweredCount: len(grid.Unpowered),
	}

	for i := 0; i < grid.W*grid.H; i++ {
		res.Base[i*3] = grid.BaseR[i]
		res.Base[i*3+1] = grid.BaseG[i]
		res.Base[i*3+2] = grid.BaseB[i]
	}

	// Stamps help to not apply the same source to the same corner twice without clearing the memory.
	stamps := make([]int32, (grid.W+1)*(grid.H+1))

	bay := false
	stamp := int32(0)
	for _, src := range sources {
		stamp++
		bay = bay || src.Bay
		applySource(grid, res, stamps, stamp, src)
	}

	// The light of upper levels goes through their open tiles: apply_lum of the Baystation light source.
	for level := range grid.Upper {
		for _, src := range grid.Upper[level].Sources {
			stamp++
			bay = bay || src.Bay
			applyUpperSource(grid, level, res, stamps, stamp, src)
		}
	}

	// update_overlays of the Baystation lighting corner: the light is not linear there.
	if bay {
		for i, lum := range res.Lum {
			if lum > 0 {
				res.Lum[i] = bayMultFactor * float32(math.Sqrt(float64(lum)))
			}
		}
	}

	// The same as the `lighting_corner` does: if any of the channels is more than 1,
	// then the whole color is scaled down to keep the hue.
	for i := 0; i < len(res.Lum); i += 3 {
		r, g, b := res.Lum[i], res.Lum[i+1], res.Lum[i+2]
		mx := max(r, g, b)
		if mx > 1 {
			res.Lum[i], res.Lum[i+1], res.Lum[i+2] = r/mx, g/mx, b/mx
		}
	}

	return res
}

// applyUpperSource lights tiles of the grid by a source from the upper level.
// A tile gets the light if the source sees it on its level and all tiles above the grid level are open.
func applyUpperSource(grid *Grid, level int, res *Result, stamps []int32, stamp int32, src Source) {
	if src.Range <= 0 || src.Power == 0 {
		return
	}
	upper := grid.Upper[level].Grid
	if src.X < 1 || src.Y < 1 || src.X > grid.W || src.Y > grid.H {
		return
	}

	workingRange := int(math.Ceil(float64(min(src.Range, maxRange))))
	x1, y1 := max(1, src.X-workingRange), max(1, src.Y-workingRange)
	x2, y2 := min(grid.W, src.X+workingRange), min(grid.H, src.Y+workingRange)

	for ty := y1; ty <= y2; ty++ {
		for tx := x1; tx <= x2; tx++ {
			idx := grid.idx(tx, ty)
			if grid.Opaque[idx] {
				continue
			}
			open := true
			for l := 0; l <= level && open; l++ {
				open = grid.Upper[l].Grid.Open[idx]
			}
			if !open || !visible(upper, src.X, src.Y, tx, ty) {
				continue
			}
			for cy := ty - 1; cy <= ty; cy++ {
				for cx := tx - 1; cx <= tx; cx++ {
					sIdx := cy*(grid.W+1) + cx
					if stamps[sIdx] == stamp {
						continue
					}
					stamps[sIdx] = stamp
					lum := bayLum(src, float32(cx)+.5-float32(src.X), float32(cy)+.5-float32(src.Y))
					if lum == 0 {
						continue
					}
					i := sIdx * 3
					res.Lum[i] += lum * src.R
					res.Lum[i+1] += lum * src.G
					res.Lum[i+2] += lum * src.B
				}
			}
		}
	}
}

func applySource(grid *Grid, res *Result, stamps []int32, stamp int32, src Source) {
	if src.Range <= 0 || src.Power == 0 {
		return
	}
	if src.X < 1 || src.Y < 1 || src.X > grid.W || src.Y > grid.H {
		return
	}

	lightRange := min(src.Range, maxRange)
	workingRange := int(math.Ceil(float64(lightRange) + math.Max(math.Abs(float64(src.OffsetX)), math.Abs(float64(src.OffsetY)))))
	rangeDivisor := max(1, lightRange)

	directional := src.Angle > 0 && src.Angle < 360

	// The light position. Tile centers are in integer coords, corners are at .5 offsets.
	lx := float32(src.X) + src.OffsetX
	ly := float32(src.Y) + src.OffsetY

	x1, y1 := max(1, src.X-workingRange), max(1, src.Y-workingRange)
	x2, y2 := min(grid.W, src.X+workingRange), min(grid.H, src.Y+workingRange)

	for ty := y1; ty <= y2; ty++ {
		for tx := x1; tx <= x2; tx++ {
			// Opaque turfs are skipped, they get the light only from their neighbours corners.
			if grid.Opaque[grid.idx(tx, ty)] {
				continue
			}
			if !visible(grid, src.X, src.Y, tx, ty) {
				continue
			}

			// Every tile has 4 corners. The tile (tx, ty) has corners with indexes from (tx-1, ty-1) to (tx, ty).
			for cy := ty - 1; cy <= ty; cy++ {
				for cx := tx - 1; cx <= tx; cx++ {
					sIdx := cy*(grid.W+1) + cx
					if stamps[sIdx] == stamp {
						continue
					}
					stamps[sIdx] = stamp

					// Corner position in the tile coords.
					dx := float32(cx) + .5 - lx
					dy := float32(cy) + .5 - ly

					var lum float32
					if src.Bay {
						lum = bayLum(src, dx, dy)
					} else {
						falloff := 1 - clamp01(float32(math.Sqrt(float64(dx*dx+dy*dy+lightingHeight)))/rangeDivisor)
						if directional {
							falloff *= coneMultiplier(dx, dy, src.DirAngle, src.Angle)
						}
						lum = falloff * src.Power
					}
					if lum == 0 {
						continue
					}

					i := sIdx * 3
					res.Lum[i] += lum * src.R
					res.Lum[i+1] += lum * src.G
					res.Lum[i+2] += lum * src.B
				}
			}
		}
	}
}

// coneMultiplier is a port of the angle part of the `falloff_at_coord` proc.
func coneMultiplier(dx, dy, dirAngle, angle float32) float32 {
	coordAngle := float32(math.Atan2(float64(dx), float64(dy)) * 180 / math.Pi)
	if coordAngle < 0 {
		coordAngle += 360
	}
	angleDelta := float32(math.Abs(float64(dirAngle - coordAngle)))
	if angleDelta > 180 {
		angleDelta = 180 - (angleDelta - 180)
	}
	return max(1-max(angleDelta-(angle/2), 0)/30, 0)
}

// visible checks if the target tile is visible from the source tile.
// It's a simple approximation of the BYOND view(): the line between centers of tiles must not be blocked.
// The line is traced in both directions, and the tile is visible if any of them is clear.
// It makes the result symmetric and a bit more permissive, like the BYOND one.
func visible(grid *Grid, sx, sy, tx, ty int) bool {
	if sx == tx && sy == ty {
		return true
	}
	return lineClear(grid, sx, sy, tx, ty) || lineClear(grid, tx, ty, sx, sy)
}

// lineClear traces a Bresenham line and returns false if any tile between the start and the end is opaque.
func lineClear(grid *Grid, x0, y0, x1, y1 int) bool {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy

	x, y := x0, y0
	for {
		if x == x1 && y == y1 {
			return true
		}
		if (x != x0 || y != y0) && grid.opaque(x, y) {
			return false
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x += sx
		}
		if e2 <= dx {
			e += dx
			y += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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

// bayLum is a port of the Baystation LUM_FALLOFF and APPLY_CORNER macros:
// the light has full brightness up to the inner range and fades linearly to zero at the outer range.
// The falloff curve is applied only to the cone factor, exactly like the macro does (** binds tighter than *).
func bayLum(src Source, dx, dy float32) float32 {
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	falloff := clamp01(-(dist - src.Range) / max(src.Range-src.Inner, 1))
	if src.DirX != 0 || src.DirY != 0 {
		cone := clamp01((src.DirX*dx + src.DirY*dy) / max(dist, .5))
		falloff *= float32(math.Pow(float64(cone), float64(src.Curve)))
	}
	lum := falloff * src.Power * src.Power
	if src.Power < 0 {
		lum = -lum
	}
	return lum
}
