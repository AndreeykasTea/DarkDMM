package pmap

import (
	"fmt"
	"sort"
	"strings"

	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmroute"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

const (
	varReqAccess      = "req_access"
	varReqOneAccess   = "req_one_access"
	varCheckOneAccess = "check_one_access"
)

// AccessWindowAutoOpen controls whether the access window opens when an object with accesses is selected.
var AccessWindowAutoOpen = true

// accessClipboard stores copied accesses, so they could be pasted to other objects or maps.
var accessClipboard []string

type accessWindow struct {
	// bay is true for the Baystation access model, bayAnyOf is the "any of" mode (nested list).
	bay, bayAnyOf bool
	// complexRule describes a rule which can't be shown with checkboxes (mixed nested lists).
	complexRule string

	open     bool
	instance *dmminstance.Instance
	filter   string

	lastSelectedId uint64
	// Window is placed near the mouse only when it's opened.
	placeNearMouse bool
}

// HasAccessVars returns true if the object can have accesses.
func HasAccessVars(vars *dmvars.Variables) bool {
	_, ok := vars.Value(varReqAccess)
	if !ok {
		_, ok = vars.Value(varReqOneAccess)
	}
	return ok
}

// OpenAccessWindow opens the access window for the provided instance.
func (p *PaneMap) OpenAccessWindow(i *dmminstance.Instance) {
	p.accessWin.open = true
	p.accessWin.instance = i
	p.accessWin.placeNearMouse = true
}

func (p *PaneMap) processAccessWindow() {
	aw := &p.accessWin

	// Open the window automatically, when the user selects an object with accesses.
	if inst, ok := p.app.SelectedInstance(); ok && p.active {
		if inst.Id() != aw.lastSelectedId {
			aw.lastSelectedId = inst.Id()
			if AccessWindowAutoOpen && IsAccessRelevant(inst.Prefab().Path(), inst.Prefab().Vars()) {
				p.OpenAccessWindow(inst)
			}
		}
	} else if !ok {
		aw.lastSelectedId = 0
	}

	if !aw.open || aw.instance == nil {
		return
	}
	if !p.isInstanceOnMap(aw.instance) {
		aw.open = false
		aw.instance = nil
		return
	}

	if aw.placeNearMouse {
		aw.placeNearMouse = false
		pos := imgui.MousePos().Plus(imgui.Vec2{X: 24, Y: -40})
		imgui.SetNextWindowPosV(pos, imgui.ConditionAlways, imgui.Vec2{})
	}
	imgui.SetNextWindowSizeConstraints(
		imgui.Vec2{X: window.PointSize() * 320, Y: window.PointSize() * 200},
		imgui.Vec2{X: window.PointSize() * 520, Y: window.PointSize() * 700},
	)
	imgui.SetNextWindowSizeV(imgui.Vec2{X: window.PointSize() * 360, Y: window.PointSize() * 520}, imgui.ConditionFirstUseEver)

	open := true
	flags := imgui.WindowFlagsNoDocking | imgui.WindowFlagsNoCollapse
	if imgui.BeginV("Доступы###access_window_"+p.dmm.Name, &open, flags) {
		p.showAccessWindowContent()
	}
	imgui.End()

	if !open {
		aw.open = false
	}
}

// isInstanceOnMap checks that the instance still exists (it could be deleted or replaced by undo).
func (p *PaneMap) isInstanceOnMap(i *dmminstance.Instance) bool {
	coord := i.Coord()
	if !p.dmm.HasTile(coord) {
		return false
	}
	for _, instance := range p.dmm.GetTile(coord).Instances() {
		if instance.Id() == i.Id() {
			return true
		}
	}
	return false
}

func (p *PaneMap) showAccessWindowContent() {
	aw := &p.accessWin
	inst := aw.instance
	prefab := inst.Prefab()
	vars := prefab.Vars()
	db := dmaccess.Current()

	// Header: what the object is and where it's placed.
	imgui.TextColored(style.ColorGold, vars.TextV("name", dm.PathLast(prefab.Path())))
	imgui.TextDisabled(prefab.Path())
	coord := inst.Coord()
	areaName, areaPath := p.areaOf(coord)
	imgui.Text(fmt.Sprintf("Зона: %s", areaName))
	if imgui.IsItemHovered() {
		imgui.SetTooltip(areaPath)
	}
	imgui.TextDisabled(fmt.Sprintf("X:%d Y:%d Z:%d", coord.X, coord.Y, coord.Z))

	listVar := p.accessListVar(vars)
	raw := dmaccess.ParseList(vars.ValueV(listVar, ""))
	current := raw

	// Baystation: list(a, b) needs all accesses, list(list(a, b)) needs any of them.
	aw.bay = isBayAccess(vars)
	aw.complexRule = ""
	if aw.bay {
		current = dmaccess.Flatten(raw)
		aw.bayAnyOf = len(raw) == 1 && dmaccess.IsGroup(raw[0])
		groups := 0
		for _, v := range raw {
			if dmaccess.IsGroup(v) {
				groups++
			}
		}
		if groups > 0 && !aw.bayAnyOf {
			aw.complexRule = describeDoorAccess(db, &dmroute.Door{ReqAccess: raw})
		}
	}

	selected := make(map[string]bool, len(current))
	for _, v := range current {
		selected[v] = true
	}

	imgui.Separator()

	// Departments which the object belongs to.
	departments := accessDepartments(db, current)
	switch {
	case len(current) == 0:
		imgui.TextColored(style.ColorGreen3, "Без доступа: открыто всем")
	case len(departments) == 0:
		imgui.Text("Отдел: не определён")
	default:
		imgui.TextWrapped("Отдел: " + strings.Join(departments, ", "))
	}

	if len(current) > 0 {
		var names []string
		for _, v := range current {
			names = append(names, accessName(db, v))
		}
		imgui.TextWrapped("Требуется: " + strings.Join(names, ", "))
	}

	// Mode: any of the accesses or all of them.
	p.showAccessMode(vars, listVar, current)
	if aw.complexRule != "" {
		imgui.TextColored(style.ColorGold, "Сложное правило: "+aw.complexRule)
		imgui.TextDisabled("Изменение галочками заменит его на простое.")
	}

	imgui.Separator()

	imgui.Checkbox("Открывать при выборе объекта##access_auto_open", &AccessWindowAutoOpen)

	// Quick actions.
	if imgui.Button("Очистить") {
		p.setAccessList(listVar, nil)
	}
	imgui.SameLine()
	if imgui.Button("Копировать") {
		accessClipboard = append([]string(nil), current...)
	}
	imgui.SameLine()
	if imgui.Button("Вставить") && accessClipboard != nil {
		p.setAccessList(listVar, accessClipboard)
	}
	if imgui.IsItemHovered() {
		imgui.SetTooltip(fmt.Sprintf("Скопировано доступов: %d", len(accessClipboard)))
	}
	imgui.SameLine()
	if imgui.Button("Ко всем в зоне") {
		p.applyAccessToArea(coord, listVar, current)
	}
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Применить эти доступы ко всем объектам того же типа в этой зоне (на текущем Z-уровне)")
	}

	if db.IsEmpty() {
		imgui.Separator()
		imgui.TextWrapped("Определения доступов (#define ACCESS_*) не найдены в коде билда. Значения можно ввести вручную в редакторе переменных.")
		return
	}

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##access_filter", "Поиск доступа", &aw.filter)
	filter := strings.ToLower(aw.filter)

	imgui.BeginChildV("access_list", imgui.Vec2{}, false, 0)
	for _, region := range db.Regions {
		var visible []*dmaccess.Access
		count := 0
		for _, a := range region.Accesses {
			if selected[a.Value] {
				count++
			}
			if filter == "" || strings.Contains(strings.ToLower(a.Name()), filter) || strings.Contains(strings.ToLower(a.Define), filter) {
				visible = append(visible, a)
			}
		}
		if len(visible) == 0 {
			continue
		}

		label := region.Name
		if count > 0 {
			label = fmt.Sprintf("%s (%d)", region.Name, count)
		}
		headerFlags := imgui.TreeNodeFlagsNone
		if count > 0 || filter != "" {
			headerFlags = imgui.TreeNodeFlagsDefaultOpen
		}
		if !imgui.CollapsingHeaderV(label+"###region_"+region.Define+region.Name, headerFlags) {
			continue
		}

		for _, a := range visible {
			checked := selected[a.Value]
			if imgui.Checkbox(fmt.Sprintf("%s##access_%s", a.Name(), a.Value), &checked) {
				p.toggleAccess(listVar, current, a.Value, checked)
			}
			if imgui.IsItemHovered() {
				imgui.SetTooltip(fmt.Sprintf("%s = %s", a.Define, a.Value))
			}
		}
	}
	imgui.EndChild()
}

// accessListVar returns the name of the variable, which stores accesses of the object.
func (p *PaneMap) accessListVar(vars *dmvars.Variables) string {
	if _, ok := vars.Value(varCheckOneAccess); ok {
		return varReqAccess // Paradise: one list and a flag.
	}
	// TG style: two lists, req_one_access is used when it's not empty.
	if len(dmaccess.ParseList(vars.ValueV(varReqOneAccess, ""))) > 0 {
		return varReqOneAccess
	}
	if _, ok := vars.Value(varReqAccess); ok {
		return varReqAccess
	}
	return varReqOneAccess
}

func (p *PaneMap) showAccessMode(vars *dmvars.Variables, listVar string, current []string) {
	if aw := &p.accessWin; aw.bay {
		mode := 1
		if aw.bayAnyOf {
			mode = 0
		}
		if imgui.RadioButtonInt("Любой из доступов##access_mode_any", &mode, 0) && !aw.bayAnyOf {
			aw.bayAnyOf = true
			p.setAccessList(listVar, current)
		}
		imgui.SameLine()
		if imgui.RadioButtonInt("Все сразу##access_mode_all", &mode, 1) && aw.bayAnyOf {
			aw.bayAnyOf = false
			p.setAccessList(listVar, current)
		}
		return
	}

	if _, ok := vars.Value(varCheckOneAccess); ok {
		one := vars.ValueV(varCheckOneAccess, "1")
		anyOf := one != "0" && one != "FALSE"
		mode := 0
		if !anyOf {
			mode = 1
		}
		if imgui.RadioButtonInt("Любой из доступов##access_mode_any", &mode, 0) {
			p.setInstanceVar(varCheckOneAccess, "1")
		}
		imgui.SameLine()
		if imgui.RadioButtonInt("Все сразу##access_mode_all", &mode, 1) {
			p.setInstanceVar(varCheckOneAccess, "0")
		}
		return
	}

	_, hasOne := vars.Value(varReqOneAccess)
	_, hasAll := vars.Value(varReqAccess)
	if hasOne && hasAll {
		mode := 1
		if listVar == varReqOneAccess {
			mode = 0
		}
		// Moving accesses between req_access and req_one_access.
		if imgui.RadioButtonInt("Любой из доступов##access_mode_any", &mode, 0) && listVar != varReqOneAccess {
			p.moveAccessList(varReqAccess, varReqOneAccess, current)
		}
		imgui.SameLine()
		if imgui.RadioButtonInt("Все сразу##access_mode_all", &mode, 1) && listVar != varReqAccess {
			p.moveAccessList(varReqOneAccess, varReqAccess, current)
		}
	}
}

func (p *PaneMap) toggleAccess(listVar string, current []string, value string, enabled bool) {
	var res []string
	for _, v := range current {
		if v != value {
			res = append(res, v)
		}
	}
	if enabled {
		res = append(res, value)
	}
	p.setAccessList(listVar, res)
}

func (p *PaneMap) moveAccessList(from, to string, current []string) {
	inst := p.accessWin.instance
	vars := p.withVar(inst.Prefab().Path(), inst.Prefab().Vars(), from, "list()")
	vars = p.withVar(inst.Prefab().Path(), vars, to, dmaccess.FormatList(current))
	p.commitInstanceVars(vars)
}

func (p *PaneMap) setAccessList(listVar string, values []string) {
	p.setInstanceVar(listVar, p.formatAccessList(values))
}

// formatAccessList formats accesses for the map according to the access model of the build.
func (p *PaneMap) formatAccessList(values []string) string {
	if p.accessWin.bay && p.accessWin.bayAnyOf && len(values) > 1 {
		return "list(" + dmaccess.FormatList(values) + ")" // list(list(a, b)): any of them.
	}
	return dmaccess.FormatList(values)
}

// isBayAccess returns true for the Baystation access model: no check_one_access flag and no req_one_access list.
func isBayAccess(vars *dmvars.Variables) bool {
	_, hasFlag := vars.Value(varCheckOneAccess)
	_, hasOne := vars.Value(varReqOneAccess)
	return !hasFlag && !hasOne
}

func (p *PaneMap) setInstanceVar(name, value string) {
	inst := p.accessWin.instance
	p.commitInstanceVars(p.withVar(inst.Prefab().Path(), inst.Prefab().Vars(), name, value))
}

// withVar returns variables with the new value. If the value is the same as in the code, the variable is removed.
func (p *PaneMap) withVar(path string, vars *dmvars.Variables, name, value string) *dmvars.Variables {
	initial := ""
	if obj, ok := p.app.LoadedEnvironment().Objects[path]; ok {
		initial = obj.Vars.ValueV(name, "")
	}
	if normalizeAccessValue(initial) == normalizeAccessValue(value) {
		return dmvars.Delete(vars, name)
	}
	return dmvars.Set(vars, name, value)
}

// normalizeAccessValue makes "null", "list()" and an empty value equal.
func normalizeAccessValue(value string) string {
	switch strings.ReplaceAll(value, " ", "") {
	case "", dmvars.NullValue, "list()":
		return ""
	}
	if strings.HasPrefix(value, "list(") {
		return dmaccess.FormatList(dmaccess.ParseList(value))
	}
	return value
}

func (p *PaneMap) commitInstanceVars(vars *dmvars.Variables) {
	inst := p.accessWin.instance
	newPrefab := dmmap.PrefabStorage.Get(inst.Prefab().Path(), vars)
	if newPrefab.Id() == inst.Prefab().Id() {
		return
	}
	inst.SetPrefab(newPrefab)
	p.editor.CommitChanges("Доступы")
	p.app.SyncVarEditor()
	log.Printf("access of instance [%d] changed", inst.Id())
}

// applyAccessToArea sets the same accesses to all objects of the same type in the area of the provided tile.
func (p *PaneMap) applyAccessToArea(coord util.Point, listVar string, current []string) {
	inst := p.accessWin.instance
	_, areaPath := p.areaOf(coord)
	path := inst.Prefab().Path()
	value := p.formatAccessList(current)

	changed := 0
	for _, tile := range p.dmm.Tiles {
		if tile.Coord.Z != coord.Z {
			continue
		}
		if _, tileArea := p.areaOf(tile.Coord); tileArea != areaPath {
			continue
		}
		for _, i := range tile.Instances() {
			if i.Prefab().Path() != path {
				continue
			}
			vars := p.withVar(path, i.Prefab().Vars(), listVar, value)
			newPrefab := dmmap.PrefabStorage.Get(path, vars)
			if newPrefab.Id() != i.Prefab().Id() {
				i.SetPrefab(newPrefab)
				changed++
			}
		}
	}

	if changed > 0 {
		p.editor.CommitChanges(fmt.Sprintf("Доступы: зона (%d)", changed))
		p.app.SyncVarEditor()
	}
	log.Printf("access applied to the area [%s]: [%d] objects", areaPath, changed)
}

func (p *PaneMap) areaOf(coord util.Point) (name, path string) {
	if !p.dmm.HasTile(coord) {
		return "-", ""
	}
	for _, i := range p.dmm.GetTile(coord).Instances() {
		if dm.IsPath(i.Prefab().Path(), "/area") {
			return i.Prefab().Vars().TextV("name", dm.PathLast(i.Prefab().Path())), i.Prefab().Path()
		}
	}
	return "-", ""
}

func accessName(db *dmaccess.DB, value string) string {
	if a, ok := db.ByValue(value); ok {
		return a.Name()
	}
	return value
}

// accessDepartments returns names of departments of provided accesses.
func accessDepartments(db *dmaccess.DB, values []string) []string {
	unique := make(map[string]bool)
	for _, v := range values {
		for _, r := range db.RegionsOf(v) {
			unique[r.Name] = true
		}
	}
	res := make([]string, 0, len(unique))
	for name := range unique {
		res = append(res, name)
	}
	sort.Strings(res)
	return res
}

// hoveredAccessText returns a short description of accesses for the status panel.
func hoveredAccessText(vars *dmvars.Variables) string {
	db := dmaccess.Current()
	var values []string
	values = append(values, dmaccess.Flatten(dmaccess.ParseList(vars.ValueV(varReqAccess, "")))...)
	values = append(values, dmaccess.ParseList(vars.ValueV(varReqOneAccess, ""))...)
	if len(values) == 0 {
		return "Доступ: открыто"
	}
	names := make([]string, 0, len(values))
	for _, v := range values {
		names = append(names, accessName(db, v))
	}
	return "Доступ: " + strings.Join(names, ", ")
}

// Types which usually use accesses. Other objects have req_access only because it's declared on /obj.
var accessTypes = []string{
	"/obj/machinery/door",
	"/obj/machinery/door_control",
	"/obj/machinery/button",
	"/obj/machinery/computer",
	"/obj/machinery/vending",
	"/obj/machinery/turretid",
	"/obj/machinery/keycard_auth",
	"/obj/structure/closet/secure_closet",
	"/obj/structure/closet/crate/secure",
}

// IsAccessRelevant returns true if accesses matter for the object: it has accesses or it's a door, locker etc.
func IsAccessRelevant(path string, vars *dmvars.Variables) bool {
	if !HasAccessVars(vars) {
		return false
	}
	if len(dmaccess.Flatten(dmaccess.ParseList(vars.ValueV(varReqAccess, "")))) > 0 || len(dmaccess.ParseList(vars.ValueV(varReqOneAccess, ""))) > 0 {
		return true
	}
	for _, t := range accessTypes {
		if dm.IsPath(path, t) {
			return true
		}
	}
	return false
}
