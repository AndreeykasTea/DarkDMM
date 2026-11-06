package lighting

import (
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// Names of variables which are checked to find light properties of the atom.
// The first found variable wins. It covers TG, Paradise, Bay and older codebases.
var (
	rangeVars     = []string{"light_range", "light_range_on", "luminosity"}
	turfRangeVars = []string{"light_range", "light_range_on"}
	powerVars     = []string{"light_power", "light_power_on"}
	colorVars     = []string{"light_color", "light_color_on"}

	// Light fixtures set their light in the Initialize, so the map has only "brightness" variables for them.
	fixtureRangeVars = []string{"brightness_range", "brightness", "light_range"}
	fixturePowerVars = []string{"brightness_power", "bulb_power", "light_power"}
	fixtureColorVars = []string{"brightness_color", "bulb_colour", "bulb_color", "light_color"}
)

const fixturePath = "/obj/machinery/light"

// Kinds of objects, which are important for the whole map, not only for their tile.
const (
	specialNone = iota
	specialAPC
	specialSwitch
	specialMapData
)

// prefabInfo stores lighting related properties of the prefab.
// Prefabs are immutable, so it's safe to cache it by the prefab pointer.
type prefabInfo struct {
	isArea bool

	opaque bool
	isDoor bool
	// open is true for turfs, which let the light pass to the level below (ZM_ALLOW_LIGHTING).
	open bool

	// Areas only.
	fullbright          bool
	baseR, baseG, baseB float32
	requiresPower       bool
	alwaysUnpowered     bool
	lightswitch         bool

	hasLight bool
	light    Source
	// needsPower means that the light works only in an area with the power.
	needsPower bool
	// isLamp is for fixtures: they also depend on the light switch and on the lighting channel of the APC.
	isLamp bool

	special       int
	specialOn     bool
	specialArea   string
	specialHeight int
}

// Extractor collects lighting data from the map. Must be used only in the main thread.
type Extractor struct {
	cache map[*dmmprefab.Prefab]*prefabInfo
}

func NewExtractor() *Extractor {
	return &Extractor{cache: make(map[*dmmprefab.Prefab]*prefabInfo)}
}

// Build creates a lighting grid and a list of light sources for the provided Z-level of the map.
// For the Baystation lighting model the grid also has levels above, which shine through the open space.
func (e *Extractor) Build(dmm *dmmap.Dmm, z int) (*Grid, []Source) {
	if z < 1 || z > dmm.MaxZ {
		return newGrid(dmm.MaxX, dmm.MaxY), nil
	}

	var scan *powerScan
	if isBayEnvironment() {
		scan = e.scanMap(dmm)
	}

	grid, sources := e.buildLevel(dmm, z, scan)

	if scan != nil {
		for up := z + 1; up <= dmm.MaxZ && scan.hasBelow[up]; up++ {
			upGrid, upSources := e.buildLevel(dmm, up, scan)
			grid.Upper = append(grid.Upper, UpperLevel{Grid: upGrid, Sources: upSources})
		}
	}

	return grid, sources
}

func newGrid(w, h int) *Grid {
	return &Grid{
		W:          w,
		H:          h,
		Opaque:     make([]bool, w*h),
		Open:       make([]bool, w*h),
		Fullbright: make([]bool, w*h),
		BaseR:      make([]float32, w*h),
		BaseG:      make([]float32, w*h),
		BaseB:      make([]float32, w*h),
	}
}

func (e *Extractor) buildLevel(dmm *dmmap.Dmm, z int, scan *powerScan) (*Grid, []Source) {
	w, h := dmm.MaxX, dmm.MaxY
	grid := newGrid(w, h)

	var sources []Source

	for y := 1; y <= h; y++ {
		for x := 1; x <= w; x++ {
			idx := (y-1)*w + (x - 1)
			instances := dmm.GetTile(util.Point{X: x, Y: y, Z: z}).Instances()

			var area *prefabInfo
			var areaPath string
			if scan != nil {
				for i := len(instances) - 1; i >= 0; i-- {
					if info := e.info(instances[i].Prefab()); info.isArea {
						area, areaPath = info, instances[i].Prefab().Path()
						break
					}
				}
			}

			for _, instance := range instances {
				info := e.info(instance.Prefab())
				if info.fullbright {
					grid.Fullbright[idx] = true // Turfs (Bay space) and areas could be always lit.
				}
				if info.isArea {
					grid.BaseR[idx], grid.BaseG[idx], grid.BaseB[idx] = info.baseR, info.baseG, info.baseB
					continue
				}
				if info.opaque {
					grid.Opaque[idx] = true
					// Fillers of multi tile airlocks have the same opacity as the door.
					if scan != nil && info.isDoor {
						for _, extra := range dmmap.DoorExtraTiles(dmm, util.Point{X: x, Y: y, Z: z}, instance.Prefab()) {
							if extra.X >= 1 && extra.Y >= 1 && extra.X <= w && extra.Y <= h {
								grid.Opaque[grid.idx(extra.X, extra.Y)] = true
							}
						}
					}
				}
				if info.open {
					grid.Open[idx] = true
				}
				if info.hasLight {
					src := info.light
					src.X, src.Y = x, y
					if info.needsPower && area != nil && !Current.IgnorePower && !scan.powered(area, areaPath, info.isLamp) {
						grid.Unpowered = append(grid.Unpowered, src)
						continue
					}
					sources = append(sources, src)
				}
			}
		}
	}

	return grid, sources
}

func (e *Extractor) info(p *dmmprefab.Prefab) *prefabInfo {
	if info, ok := e.cache[p]; ok {
		return info
	}
	info := extractInfo(p)
	e.cache[p] = info
	return info
}

func extractInfo(p *dmmprefab.Prefab) *prefabInfo {
	info := &prefabInfo{}
	vars := p.Vars()
	path := p.Path()

	if dm.IsPath(path, "/area") {
		info.isArea = true
		info.requiresPower, info.lightswitch = true, true
		if v, ok := numVar(vars, "requires_power"); ok {
			info.requiresPower = v != 0
		}
		if v, ok := numVar(vars, "always_unpowered"); ok {
			info.alwaysUnpowered = v != 0
		}
		if v, ok := numVar(vars, "lightswitch"); ok {
			info.lightswitch = v != 0
		}
		// Areas without static lighting are always fully lit (space etc.).
		if v, ok := numVar(vars, "static_lighting"); ok && v == 0 {
			info.fullbright = true
		}
		if v, ok := numVar(vars, "dynamic_lighting"); ok && v == 0 {
			info.fullbright = true
		}
		if alpha, ok := numVar(vars, "base_lighting_alpha"); ok && alpha > 0 {
			r, g, b := float32(1), float32(1), float32(1)
			if col, ok := vars.Text("base_lighting_color"); ok && col != "" {
				r, g, b, _ = util.ParseColor(col).RGBA()
			}
			k := clamp01(alpha / 255)
			info.baseR, info.baseG, info.baseB = r*k, g*k, b*k
		}
		return info
	}

	if !dm.IsPath(path, "/turf") && !dm.IsPath(path, "/obj") && !dm.IsPath(path, "/mob") {
		return info
	}

	if !dm.IsPath(path, "/mob") {
		if v, ok := numVar(vars, "opacity"); ok && v != 0 {
			info.opaque = true
		}
	}

	// Bay turfs (space) disable the lighting by themselves.
	if dm.IsPath(path, "/turf") {
		if v, ok := numVar(vars, "dynamic_lighting"); ok && v == 0 {
			info.fullbright = true
		}
	}

	// The Baystation lighting model has light_outer_range instead of light_range.
	// Fixtures with a bulb type belong to this model in any case.
	_, hasBulbType := vars.Value("light_type")
	if isBayVars(vars) || (hasBulbType && dm.IsPath(path, fixturePath)) {
		extractBaySpecial(info, path, vars)
		info.isDoor = dm.IsPath(path, "/obj/machinery/door")
		info.hasLight, info.needsPower, info.light = extractBayAtom(path, vars)
		info.isLamp = dm.IsPath(path, fixturePath)
		return info
	}

	if dm.IsPath(path, fixturePath) {
		info.hasLight, info.light = extractFixture(vars)
	} else {
		info.hasLight, info.light = extractGeneric(vars, dm.IsPath(path, "/turf"))
	}

	return info
}

func extractGeneric(vars *dmvars.Variables, isTurf bool) (bool, Source) {
	names := rangeVars
	if isTurf {
		// luminosity = 1 is the default value of turfs in many code bases.
		// It means "is not hidden by the BYOND darkness", not a lamp on every tile.
		names = turfRangeVars
	}
	lightRange, ok := firstNum(vars, names)
	if !ok || lightRange <= 0 {
		return false, Source{}
	}
	if on, ok := numVar(vars, "light_on"); ok && on == 0 {
		return false, Source{}
	}

	src := Source{
		Range: lightRange,
		Power: 1,
		Angle: 360,
	}
	src.R, src.G, src.B = 1, 1, 1

	if power, ok := firstNum(vars, powerVars); ok {
		src.Power = power
	}
	if col, ok := firstText(vars, colorVars); ok {
		src.R, src.G, src.B, _ = util.ParseColor(col).RGBA()
	}
	if angle, ok := numVar(vars, "light_angle"); ok {
		src.Angle = angle
	}
	if src.Angle > 0 && src.Angle < 360 {
		lightDir := dm.DirNorth
		if v, ok := numVar(vars, "light_dir"); ok {
			lightDir = int(v)
		}
		src.DirAngle = dirToAngle(lightDir)
	}

	return true, src
}

func extractFixture(vars *dmvars.Variables) (bool, Source) {
	// LIGHT_OK is 0. Empty, burned and broken fixtures don't shine.
	if status, ok := numVar(vars, "status"); ok && status != 0 {
		return false, Source{}
	}

	lightRange, ok := firstNum(vars, fixtureRangeVars)
	if !ok || lightRange <= 0 {
		return false, Source{}
	}

	src := Source{
		Range: lightRange,
		Power: 1,
		Angle: 360,
	}
	src.R, src.G, src.B = 1, 1, 1

	if power, ok := firstNum(vars, fixturePowerVars); ok {
		src.Power = power
	}
	if col, ok := firstText(vars, fixtureColorVars); ok {
		src.R, src.G, src.B, _ = util.ParseColor(col).RGBA()
	}
	if angle, ok := numVar(vars, "light_angle"); ok {
		src.Angle = angle
	}

	// Fixtures are mounted on walls: the light goes in the opposite of their dir and shifted by half a tile.
	dir := dm.DirDefault
	if v, ok := numVar(vars, "dir"); ok {
		dir = int(v)
	}
	lightDir := reverseDir(dir)
	src.DirAngle = dirToAngle(lightDir)
	ox, oy := dirToOffset(lightDir)
	src.OffsetX, src.OffsetY = ox*.5, oy*.5

	return true, src
}

func firstNum(vars *dmvars.Variables, names []string) (float32, bool) {
	for _, name := range names {
		if v, ok := numVar(vars, name); ok {
			return v, true
		}
	}
	return 0, false
}

func firstText(vars *dmvars.Variables, names []string) (string, bool) {
	for _, name := range names {
		if v, ok := vars.Value(name); ok && v != dmvars.NullValue && strings.HasPrefix(v, "\"") {
			if txt, ok := vars.Text(name); ok && txt != "" {
				return txt, true
			}
		}
	}
	return "", false
}

// numVar parses numeric variable. Handles booleans and simple expressions with "TRUE"/"FALSE".
func numVar(vars *dmvars.Variables, name string) (float32, bool) {
	value, ok := vars.Value(name)
	if !ok || value == dmvars.NullValue {
		return 0, false
	}
	value = strings.TrimSpace(value)
	switch value {
	case "TRUE":
		return 1, true
	case "FALSE":
		return 0, true
	}
	n, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, false
	}
	return float32(n), true
}

func reverseDir(dir int) int {
	var res int
	if dir&dm.DirNorth != 0 {
		res |= dm.DirSouth
	}
	if dir&dm.DirSouth != 0 {
		res |= dm.DirNorth
	}
	if dir&dm.DirEast != 0 {
		res |= dm.DirWest
	}
	if dir&dm.DirWest != 0 {
		res |= dm.DirEast
	}
	return res
}

func dirToOffset(dir int) (float32, float32) {
	var x, y float32
	if dir&dm.DirNorth != 0 {
		y++
	}
	if dir&dm.DirSouth != 0 {
		y--
	}
	if dir&dm.DirEast != 0 {
		x++
	}
	if dir&dm.DirWest != 0 {
		x--
	}
	return x, y
}

func dirToAngle(dir int) float32 {
	switch dir {
	case dm.DirNorth:
		return 0
	case dm.DirNortheast:
		return 45
	case dm.DirEast:
		return 90
	case dm.DirSoutheast:
		return 135
	case dm.DirSouth:
		return 180
	case dm.DirSouthwest:
		return 225
	case dm.DirWest:
		return 270
	case dm.DirNorthwest:
		return 315
	}
	return 0
}
