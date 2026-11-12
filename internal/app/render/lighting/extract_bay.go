package lighting

import (
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// The Baystation lighting model (Foundation-19 and other Bay forks).
//
// Atoms have light_max_bright, light_inner_range, light_outer_range, light_falloff_curve and light_color.
// A lot of objects don't have these variables set in the code. They call set_light in the Initialize or
// in the on_update_icon, so the map doesn't know about their light. These objects are described in the bayEmitters.
const (
	powerUseActive  = 2 // POWER_USE_ACTIVE
	powerChanOff    = 0 // POWERCHAN_OFF
	zmAllowLighting = 4 // ZM_ALLOW_LIGHTING
	apcPath         = "/obj/machinery/power/apc"
	lightSwitchPath = "/obj/machinery/light_switch"
	mapDataPath     = "/obj/effect/landmark/map_data"
	blastDoorPath   = "/obj/machinery/door/blast"
	openTurfPath    = "/turf/simulated/open"
	securityGreen   = "/decl/security_level/code_green"
)

// bayEmitter is a type of objects, which set their light by the code.
type bayEmitter struct {
	path string
	// needsPower means that the object doesn't shine in an area without the power.
	needsPower bool
	read       func(vars *dmvars.Variables) (bool, Source)
}

var bayEmitters = []bayEmitter{
	{fixturePath, true, extractBayFixture},
	{"/obj/machinery/computer", true, func(vars *dmvars.Variables) (bool, Source) {
		return bayFromVars(vars, "light_max_bright_on", "light_inner_range_on", "light_outer_range_on", "light_color")
	}},
	{"/obj/machinery/alarm", true, bayConst(.25, .1, 1, "#00ff00")},         // Normal air: COLOR_LIME.
	{"/obj/machinery/status_display", true, bayConst(.5, .1, 1, "#ffffff")}, // update_display and set_picture.
	{apcPath, false, func(vars *dmvars.Variables) (bool, Source) { // Has its own cell.
		if v, ok := numVar(vars, "operating"); ok && v == 0 {
			return false, Source{}
		}
		return bayConst(.8, .1, 1, "#82ff4c")(vars)
	}},
	{"/obj/machinery/firealarm", true, func(*dmvars.Variables) (bool, Source) {
		// The round starts with the code green.
		if sl, ok := dmmap.TypeVars(securityGreen); ok {
			return bayFromVars(sl, "light_max_bright", "light_inner_range", "light_outer_range", "light_color_alarm")
		}
		return bayConst(.25, .1, 1, "#008000")(nil)
	}},
	{"/obj/machinery/turretid", true, func(vars *dmvars.Variables) (bool, Source) {
		color := "#003300"
		if v, ok := numVar(vars, "enabled"); ok && v != 0 {
			color = "#ff9900"
			if v, ok := numVar(vars, "lethal"); ok && v != 0 {
				color = "#990000"
			}
		}
		return bayConst(1, .5, 2, color)(vars)
	}},
	{lightSwitchPath, true, func(vars *dmvars.Variables) (bool, Source) {
		color := "#f86060"
		if v, ok := numVar(vars, "on"); ok && v != 0 {
			color = "#82ff4c"
		}
		return bayConst(.1, .1, 1, color)(vars)
	}},
	{"/obj/item/device/flashlight", false, func(vars *dmvars.Variables) (bool, Source) {
		if v, ok := numVar(vars, "on"); !ok || v == 0 {
			return false, Source{}
		}
		return bayFromVars(vars, "flashlight_max_bright", "flashlight_inner_range", "flashlight_outer_range", "light_color")
	}},
	{"/obj/structure/flora/pottedplant/unusual", false, bayConst(.4, .1, 2, "#007fff")},
	{"/obj/structure/flora/pottedplant/subterranean", false, bayConst(.4, .1, 2, "#ff6633")},
	{"/obj/machinery/door/airlock", true, func(vars *dmvars.Variables) (bool, Source) {
		// Only the bolts lights shine while the door is idle.
		if v, ok := numVar(vars, "locked"); !ok || v == 0 {
			return false, Source{}
		}
		if v, ok := numVar(vars, "lights"); ok && v == 0 {
			return false, Source{}
		}
		return bayConst(.25, .1, 1, "#ff3333")(vars)
	}},
	{"/obj/machinery/floor_light", true, func(vars *dmvars.Variables) (bool, Source) {
		if v, ok := numVar(vars, "use_power"); !ok || int(v) != powerUseActive {
			return false, Source{}
		}
		if v, ok := numVar(vars, "anchored"); !ok || v == 0 {
			return false, Source{}
		}
		return bayFromVars(vars, "default_light_max_bright", "default_light_inner_range", "default_light_outer_range", "default_light_colour")
	}},
}

// isBayVars returns true if the atom has variables of the Baystation lighting model.
func isBayVars(vars *dmvars.Variables) bool {
	_, ok := vars.Value("light_outer_range")
	return ok
}

// PowerAware returns true if the lighting of the loaded code base depends on the area power.
func PowerAware() bool {
	return dmmap.IsBaystation()
}

// extractBayAtom reads a light of the atom. The second result shows if the light depends on the area power.
func extractBayAtom(path string, vars *dmvars.Variables) (hasLight bool, needsPower bool, src Source) {
	for _, emitter := range bayEmitters {
		if dm.IsPath(path, emitter.path) {
			has, light := emitter.read(vars)
			return has, emitter.needsPower, light
		}
	}
	has, light := extractBay(vars)
	return has, false, light
}

func bayConst(maxBright, inner, outer float32, color string) func(*dmvars.Variables) (bool, Source) {
	return func(*dmvars.Variables) (bool, Source) {
		src := Source{Bay: true, Range: outer, Power: maxBright, Inner: inner, Curve: 2, Angle: 360}
		src.R, src.G, src.B, _ = util.ParseColor(color).RGBA()
		return true, src
	}
}

func bayFromVars(vars *dmvars.Variables, maxBrightVar, innerVar, outerVar, colorVar string) (bool, Source) {
	outer, _ := numVar(vars, outerVar)
	maxBright, ok := numVar(vars, maxBrightVar)
	if !ok {
		maxBright = 1
	}
	if outer <= 0 || maxBright == 0 {
		return false, Source{}
	}
	inner, _ := numVar(vars, innerVar)

	src := Source{Bay: true, Range: outer, Power: maxBright, Inner: inner, Curve: 2, Angle: 360}
	src.R, src.G, src.B = 1, 1, 1
	if col, ok := firstText(vars, []string{colorVar}); ok {
		src.R, src.G, src.B, _ = util.ParseColor(col).RGBA()
	}
	return true, src
}

// extractBay reads a light source from the variables of the atom (update_light in the Initialize of the atom).
func extractBay(vars *dmvars.Variables) (bool, Source) {
	has, src := bayFromVars(vars, "light_max_bright", "light_inner_range", "light_outer_range", "light_color")
	if !has {
		return false, Source{}
	}
	if curve, ok := numVar(vars, "light_falloff_curve"); ok {
		src.Curve = curve
	}
	if d, ok := numVar(vars, "light_direction"); ok && d != 0 {
		src.DirX, src.DirY = dirToOffset(int(d))
	}
	return true, src
}

// extractBayFixture reads the light of the fixture from its bulb type (light_type = /obj/item/light/...).
func extractBayFixture(vars *dmvars.Variables) (bool, Source) {
	bulbPath, ok := vars.Value("light_type")
	if !ok || bulbPath == dmvars.NullValue || !strings.HasPrefix(bulbPath, "/") {
		// No bulb type: maybe the light is set directly.
		return extractBay(vars)
	}
	bulb, ok := dmmap.TypeVars(bulbPath)
	if !ok {
		return false, Source{}
	}
	// Broken or empty fixtures.
	if status, ok := numVar(bulb, "status"); ok && status != 0 {
		return false, Source{}
	}

	has, src := bayFromVars(bulb, "b_max_bright", "b_inner_range", "b_outer_range", "b_colour")
	if !has {
		return false, Source{}
	}
	if curve, ok := numVar(bulb, "b_curve"); ok {
		src.Curve = curve
	}
	return true, src
}

// extractBaySpecial fills map related properties of the prefab: power and level connections.
func extractBaySpecial(info *prefabInfo, path string, vars *dmvars.Variables) {
	switch {
	case dm.IsPath(path, apcPath):
		info.special = specialAPC
		v, ok := numVar(vars, "lighting")
		info.specialOn = !ok || int(v) != powerChanOff
	case dm.IsPath(path, lightSwitchPath):
		info.special = specialSwitch
		v, ok := numVar(vars, "on")
		info.specialOn = ok && v != 0
		if other, ok := vars.Value("other_area"); ok && strings.HasPrefix(other, "/area") {
			info.specialArea = other
		}
	case dm.IsPath(path, mapDataPath):
		info.special = specialMapData
		height, ok := numVar(vars, "height")
		if !ok {
			height = 1
		}
		info.specialHeight = int(height)
	case dm.IsPath(path, blastDoorPath):
		// Blast doors, which begin opened, change their opacity in the Initialize.
		if v, ok := numVar(vars, "begins_closed"); ok && v == 0 {
			info.opaque = false
		}
	case dm.IsPath(path, "/turf"):
		if v, ok := numVar(vars, "z_flags"); ok {
			info.open = int(v)&zmAllowLighting != 0
		} else {
			info.open = dm.IsPath(path, openTurfPath)
		}
	}
}

// areaPower is a state of the area power, found by the map scan.
type areaPower struct {
	hasAPC     bool
	apcLightOn bool
	switched   bool // There is a light switch for this area.
	switchOn   bool
}

// powerScan is a result of the whole map scan. The area is a single object for all levels of the map.
type powerScan struct {
	areas map[string]*areaPower
	// hasBelow stores if the level (by the Z index) is connected with the previous one (HasBelow proc).
	hasBelow map[int]bool
}

func (s *powerScan) area(path string) *areaPower {
	a, ok := s.areas[path]
	if !ok {
		a = &areaPower{}
		s.areas[path] = a
	}
	return a
}

// scanMap finds APCs, light switches and connections between levels.
func (e *Extractor) scanMap(dmm *dmmap.Dmm) *powerScan {
	scan := &powerScan{areas: make(map[string]*areaPower), hasBelow: make(map[int]bool)}

	for _, tile := range dmm.Tiles {
		instances := tile.Instances()

		var areaPath string
		for i := len(instances) - 1; i >= 0; i-- {
			if dm.IsPath(instances[i].Prefab().Path(), "/area") {
				areaPath = instances[i].Prefab().Path()
				break
			}
		}

		for _, instance := range instances {
			info := e.info(instance.Prefab())
			switch info.special {
			case specialAPC:
				a := scan.area(areaPath)
				a.hasAPC = true
				a.apcLightOn = a.apcLightOn || info.specialOn
			case specialSwitch:
				target := areaPath
				if info.specialArea != "" {
					target = info.specialArea
				}
				a := scan.area(target)
				a.switched, a.switchOn = true, info.specialOn
			case specialMapData:
				// map_data/New marks levels from (z - height + 1) to (z - 1) as having a level above them.
				for z := tile.Coord.Z - info.specialHeight + 1; z < tile.Coord.Z; z++ {
					scan.hasBelow[z+1] = true
				}
			}
		}
	}

	return scan
}

// powered is the /area/proc/powered for lamps (the LIGHT channel and the light switch) and for other machines.
func (s *powerScan) powered(area *prefabInfo, areaPath string, isLamp bool) bool {
	a := s.areas[areaPath]
	if isLamp {
		switched := area.lightswitch
		if a != nil && a.switched {
			switched = a.switchOn
		}
		if !switched {
			return false
		}
	}
	if !area.requiresPower {
		return true
	}
	if area.alwaysUnpowered || a == nil || !a.hasAPC {
		return false
	}
	return !isLamp || a.apcLightOn
}
