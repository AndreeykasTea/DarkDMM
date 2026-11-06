package pmap

import (
	"sdmm/internal/app/render/lighting"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"

	"github.com/rs/zerolog/log"
)

type toolDesc struct {
	btnIcon string
	tooltip w.Layout
}

const tSeparator = "toolsSeparator"

var (
	toolsOrder = []string{
		tools.TNAdd,
		tools.TNFill,
		tools.TNGrab,
		tools.TNMove,
		tools.TNScatter,
		tools.TNRoute,
		tools.TNRouteMap,
		tSeparator,
		tools.TNPick,
		tools.TNDelete,
		tools.TNReplace,
	}

	toolsDesc = map[string]toolDesc{
		tools.TNAdd: {
			btnIcon: icon.Add,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNAdd),
				w.SameLine(),
				w.TextFrame("1"),
				w.Separator(),
				w.Text("Поставить выбранный объект"),
				w.Line(w.TextFrame("Зажать Alt"), w.Text("Поставить выбранный объект с заменой")),
			},
		},
		tools.TNFill: {
			btnIcon: icon.BorderAll,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNFill),
				w.SameLine(),
				w.TextFrame("2"),
				w.Separator(),
				w.Text("Залить область выбранным объектом"),
				w.Line(w.TextFrame("Зажать Alt"), w.Text("Залить область выбранным объектом с заменой")),
				w.Line(w.TextFrame("Зажать Ctrl"), w.Text("Залить только границы области")),
			},
		},
		tools.TNGrab: {
			btnIcon: icon.BorderStyle,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNGrab),
				w.SameLine(),
				w.TextFrame("3"),
				w.Separator(),
				w.Text("Выделить область / переместить выделение с видимыми объектами"),
			},
		},
		tools.TNMove: {
			btnIcon: icon.Shrink,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNMove),
				w.SameLine(),
				w.TextFrame("4"),
				w.Separator(),
				w.Text("Переместить один объект"),
				w.Line(w.TextFrame("Зажать Shift"), w.Text("Сдвигать объект по пикселям перетаскиванием")),
			},
		},
		tools.TNScatter: {
			btnIcon: icon.Grain,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNScatter),
				w.SameLine(),
				w.TextFrame("5"),
				w.Separator(),
				w.Text("Разбросать объекты из палитры с шансом (лес, трава, мусор)"),
				w.Line(w.TextFrame("Зажать Alt"), w.Text("Стирать объекты палитры с тем же шансом")),
			},
		},
		tools.TNRoute: {
			btnIcon: icon.Timeline,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNRoute),
				w.SameLine(),
				w.TextFrame("6"),
				w.Separator(),
				w.Text("Протянуть кабель, трубу или мусоропровод мышью"),
				w.Text("Повороты, развилки, узлы и заглушки ставятся сами"),
				w.Line(w.TextFrame("Назад мышью"), w.Text("Убрать последний тайл пути")),
			},
		},
		tools.TNRouteMap: {
			btnIcon: icon.Lock,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNRouteMap),
				w.SameLine(),
				w.TextFrame("7"),
				w.Separator(),
				w.Text("Куда может пройти член экипажа с выбранными доступами"),
				w.Line(w.TextFrame("Клик"), w.Text("Начальная точка")),
				w.Line(w.TextFrame("Alt+клик"), w.Text("Цель: покажет кратчайший путь")),
			},
		},
		tools.TNPick: {
			btnIcon: icon.EyeDropper,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNPick),
				w.SameLine(),
				w.TextFrame("Зажать S"),
				w.Separator(),
				w.Text("Взять объект под курсором"),
			},
		},
		tools.TNDelete: {
			btnIcon: icon.Eraser,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNDelete),
				w.SameLine(),
				w.TextFrame("Зажать D"),
				w.Separator(),
				w.Text("Удалить объект под курсором"),
				w.Line(w.TextFrame("+Alt"), w.Text("Удалить весь тайл")),
			},
		},
		tools.TNReplace: {
			btnIcon: icon.Repeat,
			tooltip: w.Layout{
				w.AlignTextToFramePadding(),
				w.Text(tools.TNReplace),
				w.SameLine(),
				w.TextFrame("Зажать R"),
				w.Separator(),
				w.Text("Заменить объект под курсором на выбранный"),
			},
		},
	}
)

func (p *PaneMap) showToolsPanel() {
	w.Layout{
		p.panelToolsLayoutTools(),
		w.SameLine(),
		w.Layout{
			w.AlignRight,
			p.panelToolsLayoutSettings(),
		},
	}.Build()
}

func (p *PaneMap) panelToolsLayoutTools() (layout w.Layout) {
	for idx, toolName := range toolsOrder {
		var toolName = toolName // Closure (hello, js)

		if idx > 0 || idx < len(toolsOrder)-1 {
			layout = append(layout, w.SameLine())
		}

		if toolName == tSeparator {
			layout = append(layout, w.TextDisabled("|"))
			continue
		}

		tool := tools.Tools()[toolName]
		desc := toolsDesc[toolName]

		btn := w.Button(desc.btnIcon, func() {
			tools.SetSelected(toolName)
		}).Round(true)

		if tools.Selected() == tool {
			if tool.AltBehaviour() {
				btn.Style(style.ButtonGold{}).TextColor(style.ColorBlack)
			} else {
				btn.Style(style.ButtonGreen{})
			}
		}

		layout = append(layout, btn, w.Tooltip(desc.tooltip))
	}
	return layout
}

func (p *PaneMap) panelToolsLayoutSettings() w.Layout {
	var bntStyle w.ButtonStyle
	if p.showSettings {
		bntStyle = style.ButtonGreen{}
	} else {
		bntStyle = style.ButtonDefault{}
	}
	var lightStyle w.ButtonStyle
	if lighting.Current.Enabled {
		lightStyle = style.ButtonGold{}
	} else {
		lightStyle = style.ButtonDefault{}
	}
	return w.Layout{
		w.Button(icon.Lightbulb, p.doToggleLighting).
			Tooltip("Предпросмотр освещения (Ctrl+L)").
			Style(lightStyle).
			Round(true),
		w.SameLine(),
		w.Button(icon.Cog, p.doToggleSettings).
			Tooltip("Настройки").
			Style(bntStyle).
			Round(true),
	}
}

func (p *PaneMap) doToggleLighting() {
	lighting.Current.Enabled = !lighting.Current.Enabled
	log.Print("toggle lighting:", lighting.Current.Enabled)
}

func (p *PaneMap) doToggleSettings() {
	log.Print("toggle settings:")
	p.showSettings = !p.showSettings
}

func (p *PaneMap) doPreviousLevel() {
	if p.hasPreviousLevel() {
		p.activeLevel--
		log.Print("active level switched to previous:", p.activeLevel)
	}
}

func (p *PaneMap) doNextLevel() {
	if p.hasNextLevel() {
		p.activeLevel++
		log.Print("active level switched to next:", p.activeLevel)
	}
}

func (p *PaneMap) hasPreviousLevel() bool {
	return p.activeLevel > 1
}

func (p *PaneMap) hasNextLevel() bool {
	return p.activeLevel < p.dmm.MaxZ
}
