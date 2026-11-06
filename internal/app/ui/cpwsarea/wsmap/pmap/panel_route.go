package pmap

import (
	"fmt"
	"sort"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/imguiext/style"

	"github.com/SpaiR/imgui-go"
)

// Quick presets of the Route tool. Only types existing in the environment are shown.
var routePresets = []struct {
	name, path string
}{
	{"Подача воздуха", "/obj/machinery/atmospherics/pipe/simple/hidden/supply"},
	{"Скрубберы", "/obj/machinery/atmospherics/pipe/simple/hidden/scrubbers"},
	{"Труба (скрытая)", "/obj/machinery/atmospherics/pipe/simple/hidden"},
	{"Труба (видимая)", "/obj/machinery/atmospherics/pipe/simple/visible"},
	{"Мусоропровод", "/obj/structure/disposalpipe/segment"},
}

func (p *PaneMap) showRoutePanel() {
	imgui.Dummy(imgui.Vec2{X: window.PointSize() * 280})
	imgui.Text("Прокладка")
	imgui.Separator()

	env := p.app.LoadedEnvironment()

	// Quick selection of typical networks, so the user doesn't need to look for proper types.
	imgui.TextDisabled("Быстрый выбор:")
	p.showRouteCables()
	for _, preset := range routePresets {
		if _, ok := env.Objects[preset.path]; !ok {
			continue
		}
		if imgui.Button(preset.name + "##route_preset_" + preset.path) {
			p.app.DoSelectPrefab(dmmap.PrefabStorage.Initial(preset.path))
		}
		if imgui.IsItemHovered() {
			imgui.SetTooltip(preset.path)
		}
		imgui.SameLine()
	}
	imgui.NewLine()

	imgui.Separator()

	prefab, ok := p.app.SelectedPrefab()
	if !ok {
		imgui.TextWrapped("Выберите сеть выше или тип в окружении.")
		return
	}

	net := dmnet.Detect(env, prefab)
	imgui.TextColored(style.ColorGold, net.Kind.String())
	imgui.TextDisabled(prefab.Path())

	switch net.Kind {
	case dmnet.KindCable:
		imgui.TextWrapped("Ставятся куски \"a-b\", на концах — узлы \"0-d\" (к ним подключаются APC и машины). Соединяется с кабелем того же типа.")
	case dmnet.KindPipe:
		imgui.TextWrapped("Прямые, изгибы, тройники (manifold) и крестовины (manifold4w) ставятся сами. Конец на вентиляции/скруббере поворачивает её к трубе.")
		imgui.Checkbox("Заглушки на открытых концах##route_caps", &tools.Route.UseCaps)
	case dmnet.KindDisposal:
		imgui.TextWrapped("Прямые, повороты и развилки. Конец на мусорке ставит trunk. Особые узлы (сортировка) не изменяются.")
	case dmnet.KindSmartCable:
		imgui.TextWrapped("Кабель сам вычисляет соединения: ставится по одному на тайл.")
	default:
		if dm.IsPath(prefab.Path(), "/obj/item") {
			imgui.TextColored(style.ColorRedLighter, "Это предмет, а не проложенная сеть!")
			imgui.TextWrapped("Например, /obj/item/pipe — трубная деталь, которую носят в руках. Для труб на карте нужен /obj/machinery/atmospherics/pipe/..., выберите его кнопками выше.")
		} else {
			imgui.TextWrapped("Это не кабель и не труба: объект просто ставится на каждый тайл пути.")
		}
	}

	if tools.Route.LastResult != "" {
		imgui.Separator()
		imgui.Text(tools.Route.LastResult)
		for _, warning := range tools.Route.LastWarnings {
			imgui.TextColored(style.ColorRedLighter, warning)
		}
	}
}

// showRouteCables shows a combo with cable types (colors and layers).
func (p *PaneMap) showRouteCables() {
	env := p.app.LoadedEnvironment()
	const base = "/obj/structure/cable"
	obj, ok := env.Objects[base]
	if !ok {
		return
	}

	cables := []string{base}
	for _, child := range obj.DirectChildren {
		if c, ok := env.Objects[child]; ok && len(c.DirectChildren) == 0 {
			cables = append(cables, child)
		}
	}
	sort.Strings(cables[1:])

	preview := "Кабель"
	if prefab, ok := p.app.SelectedPrefab(); ok && dm.IsPath(prefab.Path(), base) {
		preview = fmt.Sprintf("Кабель: %s", dm.PathLast(prefab.Path()))
	}

	imgui.SetNextItemWidth(window.PointSize() * 180)
	if imgui.BeginCombo("##route_cable", preview) {
		for _, path := range cables {
			label := dm.PathLast(path)
			if path == base {
				label = "cable (обычный)"
			}
			if imgui.Selectable(label + "##" + path) {
				p.app.DoSelectPrefab(dmmap.PrefabStorage.Initial(path))
			}
		}
		imgui.EndCombo()
	}
}
