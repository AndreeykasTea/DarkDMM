package tools

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"

	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/overlay"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// ScatterMode is a way of how the Scatter tool selects tiles.
type ScatterMode int

const (
	// ScatterModeArea - user selects a rectangle, the scatter is applied on release.
	ScatterModeArea ScatterMode = iota
	// ScatterModeBrush - user paints with a round brush.
	ScatterModeBrush
)

// ScatterEntry is a single object of the scatter palette.
type ScatterEntry struct {
	Prefab *dmmprefab.Prefab
	Weight int32
}

// ScatterSettings are settings of the Scatter tool. Shared between all maps.
type ScatterSettings struct {
	Mode ScatterMode

	// Chance in percents to place an object on the tile.
	Chance int32
	// Radius of the brush in tiles.
	BrushRadius int32
	// Spacing is a minimal distance in tiles between objects from the palette. 0 = no limit.
	Spacing int32

	// Palette of objects to scatter. If it's empty, the selected object is used.
	Palette []ScatterEntry

	RandomDir    bool
	RandomDir8   bool
	PixelJitter  int32
	OnlyFree     bool
	SkipOccupied bool
}

// Scatter is the global settings of the tool.
var Scatter = &ScatterSettings{
	Mode:        ScatterModeArea,
	Chance:      30,
	BrushRadius: 3,
	OnlyFree:    true,
}

// ToolScatter places objects randomly with a chance. Very helpful to create forests, debris, grass etc.
// It works like scatter brushes in game engines: objects are picked from the weighted palette.
//
// Default: place objects.
// Alternative: remove objects from the palette with the same chance.
type ToolScatter struct {
	tool

	start    util.Point
	area     util.Bounds
	dragging bool

	processed map[util.Point]bool
}

func (ToolScatter) Name() string {
	return TNScatter
}

func newScatter() *ToolScatter {
	return &ToolScatter{processed: make(map[util.Point]bool)}
}

func (t *ToolScatter) Stale() bool {
	return !t.dragging
}

func (t *ToolScatter) process() {
	fill, border := overlay.ColorToolFillTileFill, overlay.ColorToolFillTileBorder
	if t.AltBehaviour() {
		fill, border = overlay.ColorToolFillAltTileFill, overlay.ColorToolFillAltTileBorder
	}

	switch Scatter.Mode {
	case ScatterModeArea:
		if t.dragging {
			ed.OverlayPushArea(t.area, fill, border)
		}
	case ScatterModeBrush:
		if cs != nil && !cs.HoverOutOfBounds() {
			for _, coord := range brushTiles(cs.HoveredTile(), int(Scatter.BrushRadius)) {
				if ed.Dmm().HasTile(coord) {
					ed.OverlayPushTile(coord, fill, border)
				}
			}
		}
	}
}

func (t *ToolScatter) onStart(coord util.Point) {
	if len(scatterPalette()) == 0 {
		return
	}
	t.dragging = true
	t.start = coord
	t.onMove(coord)
}

func (t *ToolScatter) onMove(coord util.Point) {
	if !t.dragging {
		return
	}

	switch Scatter.Mode {
	case ScatterModeArea:
		t.area.X1 = float32(min(t.start.X, coord.X))
		t.area.Y1 = float32(min(t.start.Y, coord.Y))
		t.area.X2 = float32(max(t.start.X, coord.X))
		t.area.Y2 = float32(max(t.start.Y, coord.Y))
	case ScatterModeBrush:
		var tiles []util.Point
		for _, c := range brushTiles(coord, int(Scatter.BrushRadius)) {
			if !t.processed[c] && ed.Dmm().HasTile(c) {
				t.processed[c] = true
				tiles = append(tiles, c)
			}
		}
		if updated := t.apply(tiles); len(updated) > 0 {
			ed.UpdateCanvasByCoords(updated)
		}
	}
}

func (t *ToolScatter) onStop(util.Point) {
	if !t.dragging {
		return
	}

	if Scatter.Mode == ScatterModeArea {
		var tiles []util.Point
		for x := int(t.area.X1); x <= int(t.area.X2); x++ {
			for y := int(t.area.Y1); y <= int(t.area.Y2); y++ {
				tiles = append(tiles, util.Point{X: x, Y: y, Z: t.start.Z})
			}
		}
		if updated := t.apply(tiles); len(updated) > 0 {
			ed.UpdateCanvasByCoords(updated)
		}
	}

	if t.AltBehaviour() {
		ed.CommitChanges("Разброс: стирание")
	} else {
		ed.CommitChanges("Разброс")
	}

	t.dragging = false
	t.start = util.Point{}
	t.area = util.Bounds{}
	t.processed = make(map[util.Point]bool)
}

func (t *ToolScatter) OnDeselect() {
	t.dragging = false
	t.processed = make(map[util.Point]bool)
}

// apply places (or removes) palette objects on provided tiles. Returns coords of modified tiles.
func (t *ToolScatter) apply(tiles []util.Point) (updated []util.Point) {
	palette := scatterPalette()
	if len(palette) == 0 {
		return nil
	}

	chance := float64(Scatter.Chance) / 100

	// Random order makes the spacing look natural, without patterns from the iteration order.
	rand.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] })

	for _, coord := range tiles {
		if !ed.Dmm().HasTile(coord) || rand.Float64() >= chance {
			continue
		}

		tile := ed.Dmm().GetTile(coord)

		if t.AltBehaviour() {
			if removePaletteInstance(tile, palette) {
				updated = append(updated, coord)
			}
			continue
		}

		if Scatter.OnlyFree && isTileDense(tile) {
			continue
		}
		if Scatter.SkipOccupied && hasPaletteInstance(tile, palette) {
			continue
		}
		if Scatter.Spacing > 0 && hasPaletteNearby(coord, int(Scatter.Spacing), palette) {
			continue
		}

		prefab := randomizePrefab(pickWeighted(palette))
		t.basicPrefabAdd(tile, prefab)
		updated = append(updated, coord)
	}

	return updated
}

// scatterPalette returns the palette to use. The selected object is used when the palette is empty.
func scatterPalette() []ScatterEntry {
	var palette []ScatterEntry
	for _, e := range Scatter.Palette {
		if e.Prefab != nil && e.Weight > 0 {
			palette = append(palette, e)
		}
	}
	if len(palette) == 0 && ed != nil {
		if prefab, ok := ed.SelectedPrefab(); ok {
			palette = append(palette, ScatterEntry{Prefab: prefab, Weight: 1})
		}
	}
	return palette
}

func pickWeighted(palette []ScatterEntry) *dmmprefab.Prefab {
	var total int32
	for _, e := range palette {
		total += e.Weight
	}
	n := rand.Int32N(total)
	for _, e := range palette {
		if n < e.Weight {
			return e.Prefab
		}
		n -= e.Weight
	}
	return palette[len(palette)-1].Prefab
}

var (
	cardinalDirs = []int{dm.DirNorth, dm.DirSouth, dm.DirEast, dm.DirWest}
	allDirs      = []int{
		dm.DirNorth, dm.DirSouth, dm.DirEast, dm.DirWest,
		dm.DirNortheast, dm.DirNorthwest, dm.DirSoutheast, dm.DirSouthwest,
	}
)

// randomizePrefab returns a prefab with random dir and pixel offsets, if it's enabled.
func randomizePrefab(prefab *dmmprefab.Prefab) *dmmprefab.Prefab {
	// Only movable atoms could be rotated or shifted without breaking the map.
	if !dm.IsPath(prefab.Path(), "/obj") && !dm.IsPath(prefab.Path(), "/mob") {
		return prefab
	}
	if !Scatter.RandomDir && Scatter.PixelJitter <= 0 {
		return prefab
	}

	vars := prefab.Vars()
	if Scatter.RandomDir {
		dirs := cardinalDirs
		if Scatter.RandomDir8 {
			dirs = allDirs
		}
		vars = dmvars.Set(vars, "dir", strconv.Itoa(dirs[rand.IntN(len(dirs))]))
	}
	if j := int(Scatter.PixelJitter); j > 0 {
		vars = setOrDeleteZero(vars, "pixel_x", rand.IntN(j*2+1)-j)
		vars = setOrDeleteZero(vars, "pixel_y", rand.IntN(j*2+1)-j)
	}

	return dmmap.PrefabStorage.Get(prefab.Path(), vars)
}

// setOrDeleteZero sets the integer variable. Zero values are not stored, if the initial value is zero as well.
func setOrDeleteZero(vars *dmvars.Variables, name string, value int) *dmvars.Variables {
	if value == 0 {
		if parent := vars.Parent(); parent != nil && parent.IntV(name, 0) == 0 {
			return dmvars.Delete(vars, name)
		}
	}
	return dmvars.Set(vars, name, strconv.Itoa(value))
}

func isDense(prefab *dmmprefab.Prefab) bool {
	value, ok := prefab.Vars().Value("density")
	if !ok {
		return false
	}
	value = strings.TrimSpace(value)
	return value != "0" && value != "FALSE" && value != dmvars.NullValue
}

// isTileDense returns true if the tile has a dense turf (walls) or a dense object.
func isTileDense(tile *dmmap.Tile) bool {
	for _, instance := range tile.Instances() {
		path := instance.Prefab().Path()
		if (dm.IsPath(path, "/turf") || dm.IsPath(path, "/obj")) && isDense(instance.Prefab()) {
			return true
		}
	}
	return false
}

func isPalettePath(path string, palette []ScatterEntry) bool {
	for _, e := range palette {
		if e.Prefab.Path() == path {
			return true
		}
	}
	return false
}

func hasPaletteInstance(tile *dmmap.Tile, palette []ScatterEntry) bool {
	for _, instance := range tile.Instances() {
		if isPalettePath(instance.Prefab().Path(), palette) {
			return true
		}
	}
	return false
}

func removePaletteInstance(tile *dmmap.Tile, palette []ScatterEntry) bool {
	for _, instance := range tile.Instances() {
		if isPalettePath(instance.Prefab().Path(), palette) {
			tile.InstancesRemoveByInstance(instance)
			tile.InstancesRegenerate()
			return true
		}
	}
	return false
}

func hasPaletteNearby(coord util.Point, spacing int, palette []ScatterEntry) bool {
	for x := coord.X - spacing; x <= coord.X+spacing; x++ {
		for y := coord.Y - spacing; y <= coord.Y+spacing; y++ {
			c := util.Point{X: x, Y: y, Z: coord.Z}
			if ed.Dmm().HasTile(c) && hasPaletteInstance(ed.Dmm().GetTile(c), palette) {
				return true
			}
		}
	}
	return false
}

// brushTiles returns tiles inside the round brush.
func brushTiles(center util.Point, radius int) []util.Point {
	radius = max(0, radius)
	r := float64(radius) + .5
	var tiles []util.Point
	for x := -radius; x <= radius; x++ {
		for y := -radius; y <= radius; y++ {
			if math.Hypot(float64(x), float64(y)) <= r {
				tiles = append(tiles, util.Point{X: center.X + x, Y: center.Y + y, Z: center.Z})
			}
		}
	}
	return tiles
}
