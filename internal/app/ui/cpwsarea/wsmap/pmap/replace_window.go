package pmap

import (
	"fmt"
	"sort"
	"strings"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

type replaceScope int

const (
	replaceScopeMap replaceScope = iota
	replaceScopeLevel
	replaceScopeSelection
)

// Variables which describe placement of the object. They are kept by default, when objects are replaced.
var placementVars = []string{"dir", "pixel_x", "pixel_y", "pixel_w", "pixel_z", "step_x", "step_y"}

// replaceWindow allows to replace objects of one type with objects of any other type.
type replaceWindow struct {
	open           bool
	placeNearMouse bool

	fromPath        string
	fromPrefab      *dmmprefab.Prefab // Not nil, when only the exact prefab should be replaced.
	onlyExact       bool
	includeSubtypes bool

	toPath   string
	toPrefab *dmmprefab.Prefab // Not nil, when the target was taken from the selected prefab with its vars.

	keepPlacement bool
	keepAllVars   bool
	scope         replaceScope

	// Suggestions for the target path.
	suggestFor string
	suggests   []string

	lastResult string
}

// OpenReplaceWindow opens the window to replace objects like the provided instance.
func (p *PaneMap) OpenReplaceWindow(i *dmminstance.Instance) {
	rw := &p.replaceWin
	rw.open = true
	rw.placeNearMouse = true
	rw.lastResult = ""
	rw.keepPlacement = true
	if i != nil {
		rw.fromPath = i.Prefab().Path()
		rw.fromPrefab = i.Prefab()
		rw.onlyExact = false
	}
	if prefab, ok := p.app.SelectedPrefab(); ok && (i == nil || prefab.Path() != i.Prefab().Path()) {
		rw.toPath = prefab.Path()
		rw.toPrefab = prefab
	}
}

func (p *PaneMap) processReplaceWindow() {
	rw := &p.replaceWin
	if !rw.open {
		return
	}

	if rw.placeNearMouse {
		rw.placeNearMouse = false
		imgui.SetNextWindowPosV(imgui.MousePos().Plus(imgui.Vec2{X: 24, Y: -40}), imgui.ConditionAlways, imgui.Vec2{})
	}
	imgui.SetNextWindowSizeV(imgui.Vec2{X: window.PointSize() * 460}, imgui.ConditionFirstUseEver)

	open := true
	if imgui.BeginV("Замена объектов###replace_window_"+p.dmm.Name, &open, imgui.WindowFlagsNoDocking|imgui.WindowFlagsNoCollapse|imgui.WindowFlagsAlwaysAutoResize) {
		p.showReplaceWindowContent()
	}
	imgui.End()

	if !open {
		rw.open = false
	}
}

func (p *PaneMap) showReplaceWindowContent() {
	rw := &p.replaceWin
	env := p.app.LoadedEnvironment()
	width := window.PointSize() * 440

	imgui.Dummy(imgui.Vec2{X: width})

	// Source.
	imgui.TextColored(style.ColorGold, "Что заменить")
	imgui.SetNextItemWidth(-window.PointSize() * 110)
	if imgui.InputTextWithHint("##replace_from", "/obj/...", &rw.fromPath) {
		rw.fromPrefab = nil
		rw.onlyExact = false
	}
	imgui.SameLine()
	if imgui.Button("Выбранный##replace_from_selected") {
		if inst, ok := p.app.SelectedInstance(); ok {
			rw.fromPath, rw.fromPrefab = inst.Prefab().Path(), inst.Prefab()
		} else if prefab, ok := p.app.SelectedPrefab(); ok {
			rw.fromPath, rw.fromPrefab = prefab.Path(), prefab
		}
	}
	showPathStatus(env.Objects[rw.fromPath] != nil, rw.fromPath)
	imgui.Checkbox("Включая подтипы##replace_subtypes", &rw.includeSubtypes)
	if rw.fromPrefab != nil && rw.fromPrefab.Vars().Len() > 0 {
		imgui.SameLine()
		imgui.Checkbox("Только с такими же переменными##replace_exact", &rw.onlyExact)
		if imgui.IsItemHovered() {
			imgui.SetTooltip(prefabVarsTooltip(rw.fromPrefab))
		}
	}

	imgui.Separator()

	// Target.
	imgui.TextColored(style.ColorGold, "На что заменить")
	imgui.SetNextItemWidth(-window.PointSize() * 110)
	if imgui.InputTextWithHint("##replace_to", "/obj/...", &rw.toPath) {
		rw.toPrefab = nil
	}
	imgui.SameLine()
	if imgui.Button("Выбранный##replace_to_selected") {
		if prefab, ok := p.app.SelectedPrefab(); ok {
			rw.toPath, rw.toPrefab = prefab.Path(), prefab
		}
	}
	targetValid := env.Objects[rw.toPath] != nil
	showPathStatus(targetValid, rw.toPath)
	if rw.toPrefab != nil && rw.toPrefab.Vars().Len() > 0 {
		imgui.TextDisabled("С переменными выбранного префаба")
		if imgui.IsItemHovered() {
			imgui.SetTooltip(prefabVarsTooltip(rw.toPrefab))
		}
	}

	// Autocomplete for the target path.
	if !targetValid && len(rw.toPath) >= 2 {
		if rw.suggestFor != rw.toPath {
			rw.suggestFor = rw.toPath
			rw.suggests = suggestPaths(env.Objects, rw.toPath, 10)
		}
		for _, s := range rw.suggests {
			if imgui.SelectableV(s+"##suggest_"+s, false, 0, imgui.Vec2{}) {
				rw.toPath = s
				rw.toPrefab = nil
			}
		}
	}

	imgui.Separator()

	imgui.Checkbox("Сохранять направление и сдвиги##replace_keep_placement", &rw.keepPlacement)
	if imgui.IsItemHovered() {
		imgui.SetTooltip(strings.Join(placementVars, ", "))
	}
	imgui.Checkbox("Переносить все изменённые переменные##replace_keep_all", &rw.keepAllVars)
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Переменные объекта, которые есть у нового типа (name, req_access и т.д.)")
	}

	scope := int(rw.scope)
	imgui.Text("Где:")
	imgui.SameLine()
	imgui.RadioButtonInt("Вся карта##replace_scope_map", &scope, int(replaceScopeMap))
	imgui.SameLine()
	imgui.RadioButtonInt("Текущий Z##replace_scope_level", &scope, int(replaceScopeLevel))
	imgui.SameLine()
	imgui.RadioButtonInt("Выделение##replace_scope_selection", &scope, int(replaceScopeSelection))
	rw.scope = replaceScope(scope)

	imgui.Separator()

	sourceValid := env.Objects[rw.fromPath] != nil
	if !sourceValid || !targetValid {
		imgui.BeginDisabled()
	}
	if imgui.Button("Посчитать") {
		rw.lastResult = fmt.Sprintf("Найдено объектов: %d", len(p.collectReplaceTargets()))
	}
	imgui.SameLine()
	if imgui.Button("Заменить") {
		count := p.doReplaceObjects()
		rw.lastResult = fmt.Sprintf("Заменено объектов: %d", count)
	}
	if !sourceValid || !targetValid {
		imgui.EndDisabled()
	}
	imgui.SameLine()
	if imgui.Button("Поменять местами##replace_swap") {
		rw.fromPath, rw.toPath = rw.toPath, rw.fromPath
		rw.fromPrefab, rw.toPrefab = rw.toPrefab, rw.fromPrefab
	}

	if rw.lastResult != "" {
		imgui.Text(rw.lastResult)
	}
}

func showPathStatus(valid bool, path string) {
	if path == "" {
		return
	}
	if !valid {
		imgui.TextColored(style.ColorRed, "Тип не найден в окружении")
	}
}

func prefabVarsTooltip(p *dmmprefab.Prefab) string {
	var lines []string
	for _, name := range p.Vars().Iterate() {
		lines = append(lines, name+" = "+p.Vars().ValueV(name, ""))
	}
	return strings.Join(lines, "\n")
}

func suggestPaths[T any](objects map[string]T, query string, limit int) []string {
	query = strings.ToLower(query)
	var res []string
	for path := range objects {
		if strings.Contains(strings.ToLower(path), query) {
			res = append(res, path)
		}
	}
	// Shorter paths first: they are closer to what the user typed.
	sort.Slice(res, func(i, j int) bool {
		if len(res[i]) != len(res[j]) {
			return len(res[i]) < len(res[j])
		}
		return res[i] < res[j]
	})
	if len(res) > limit {
		res = res[:limit]
	}
	return res
}

// collectReplaceTargets returns instances which match the source of the replacement.
func (p *PaneMap) collectReplaceTargets() []*dmminstance.Instance {
	rw := &p.replaceWin

	var bounds *util.Bounds
	if rw.scope == replaceScopeSelection {
		grab, ok := tools.Selected().(*tools.ToolGrab)
		if !ok || !grab.HasSelectedArea() {
			rw.lastResult = "Нет выделения: выделите область инструментом «Выделение»"
			return nil
		}
		b := grab.Bounds()
		bounds = &b
	}

	var res []*dmminstance.Instance
	for _, tile := range p.dmm.Tiles {
		c := tile.Coord
		if rw.scope != replaceScopeMap && c.Z != p.activeLevel {
			continue
		}
		if bounds != nil && !(float32(c.X) >= bounds.X1 && float32(c.X) <= bounds.X2 && float32(c.Y) >= bounds.Y1 && float32(c.Y) <= bounds.Y2) {
			continue
		}
		for _, i := range tile.Instances() {
			path := i.Prefab().Path()
			if rw.includeSubtypes {
				if !dm.IsPath(path, rw.fromPath) {
					continue
				}
			} else if path != rw.fromPath {
				continue
			}
			if rw.onlyExact && rw.fromPrefab != nil && i.Prefab().Id() != rw.fromPrefab.Id() {
				continue
			}
			res = append(res, i)
		}
	}
	return res
}

func (p *PaneMap) doReplaceObjects() int {
	rw := &p.replaceWin
	env := p.app.LoadedEnvironment()
	target := env.Objects[rw.toPath]
	if target == nil {
		return 0
	}

	baseVars := dmvars.FromParent(target.Vars)
	if rw.toPrefab != nil && rw.toPrefab.Path() == rw.toPath {
		baseVars = rw.toPrefab.Vars()
	}

	instances := p.collectReplaceTargets()
	cache := make(map[uint64]*dmmprefab.Prefab) // Old prefab ID -> new prefab.

	for _, i := range instances {
		newPrefab, ok := cache[i.Prefab().Id()]
		if !ok {
			newPrefab = dmmap.PrefabStorage.Get(rw.toPath, p.replaceVars(i.Prefab(), baseVars, target.Vars))
			cache[i.Prefab().Id()] = newPrefab
		}
		i.SetPrefab(newPrefab)
	}

	if len(instances) > 0 {
		p.editor.CommitChanges(fmt.Sprintf("Замена: %s → %s (%d)", dm.PathLast(rw.fromPath), dm.PathLast(rw.toPath), len(instances)))
		p.app.SyncPrefabs()
		p.app.SyncVarEditor()
	}

	log.Printf("replaced [%d] objects [%s] -> [%s]", len(instances), rw.fromPath, rw.toPath)
	return len(instances)
}

// replaceVars builds variables of the new object, copying the needed variables from the old one.
func (p *PaneMap) replaceVars(old *dmmprefab.Prefab, base, targetInitial *dmvars.Variables) *dmvars.Variables {
	rw := &p.replaceWin
	vars := base

	copyVar := func(name string) {
		value, ok := old.Vars().Value(name)
		if !ok {
			return
		}
		// Only variables declared on the map are copied, initial values of the old type are not.
		if !mapDeclared(old.Vars(), name) {
			return
		}
		if _, exists := targetInitial.Value(name); !exists {
			return // The new type doesn't have such variable.
		}
		if mapDeclared(vars, name) {
			return // The target prefab has its own value.
		}
		if targetInitial.ValueV(name, "") == value {
			return
		}
		vars = dmvars.Set(vars, name, value)
	}

	if rw.keepAllVars {
		for _, name := range old.Vars().Iterate() {
			copyVar(name)
		}
	} else if rw.keepPlacement {
		for _, name := range placementVars {
			copyVar(name)
		}
	}

	return vars
}

// mapDeclared returns true if the variable is declared directly in the prefab (on the map), not in its type.
func mapDeclared(vars *dmvars.Variables, name string) bool {
	for _, n := range vars.Iterate() {
		if n == name {
			return true
		}
	}
	return false
}
