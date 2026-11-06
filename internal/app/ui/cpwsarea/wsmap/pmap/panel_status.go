package pmap

import (
	"fmt"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/ui/shortcut"
	"sdmm/internal/imguiext/icon"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/platform"
)

func (p *PaneMap) showStatusPanel() {
	w.Layout{
		p.panelStatusLayoutStatus(),
		w.SameLine(),
		w.Custom(func() {
			if p.dmm.MaxZ != 1 {
				w.Layout{
					p.panelStatusLayoutLevels(),
				}.BuildV(w.AlignRight)
			}
		}),
	}.Build()
}

func (p *PaneMap) panelStatusLayoutStatus() (layout w.Layout) {
	if p.canvasState.HoverOutOfBounds() {
		layout = append(layout, w.TextFrame("за пределами карты"))
	} else {
		t := p.canvasState.HoveredTile()
		layout = append(layout, w.TextFrame(fmt.Sprintf("X:%03d Y:%03d", t.X, t.Y)))
	}

	layout = append(layout, w.Tooltip(w.Text("Координаты тайла под курсором")))

	if hovered := p.canvasState.HoveredInstance(); hovered != nil && IsAccessRelevant(hovered.Prefab().Path(), hovered.Prefab().Vars()) {
		layout = append(layout, w.TextFrame(hoveredAccessText(hovered.Prefab().Vars())))
	}

	if isQuickToolToggled() && !tools.Selected().AltBehaviour() {
		if hoveredInstance := p.canvasState.HoveredInstance(); hoveredInstance != nil {
			layout = append(layout, w.TextFrame(hoveredInstance.Prefab().Path()))
		}
	} else if tool, ok := tools.Selected().(*tools.ToolGrab); ok && tool.HasSelectedArea() {
		bounds := tool.Bounds()
		layout = append(layout,
			w.TextFrame(fmt.Sprintf("W:%d H:%d", int(bounds.X2-bounds.X1)+1, int(bounds.Y2-bounds.Y1)+1)),
			w.Tooltip(w.Text("Размер выделения")),
			w.TextFrame(bounds.String()),
			w.Tooltip(w.Text("Границы выделения")),
		)
	}

	return w.Layout{
		w.Line(layout...),
	}
}

func isQuickToolToggled() bool {
	return tools.IsSelected(tools.TNPick) || tools.IsSelected(tools.TNDelete) || tools.IsSelected(tools.TNReplace)
}

func (p *PaneMap) panelStatusLayoutLevels() (layout w.Layout) {
	return w.Layout{
		w.TextFrame(fmt.Sprintf("Z:%d", p.activeLevel)),
		w.Tooltip(w.Text("Текущий Z-уровень")).OnHover(true),
		w.SameLine(),
		w.Disabled(!p.hasPreviousLevel(), w.Layout{
			w.Button(icon.ArrowDownward, p.doPreviousLevel).
				Tooltip(fmt.Sprintf("Предыдущий Z-уровень (%s)", shortcut.Combine(platform.KeyModName(), "Вниз"))).
				Round(true),
		}),
		w.SameLine(),
		w.Disabled(!p.hasNextLevel(), w.Layout{
			w.Button(icon.ArrowUpward, p.doNextLevel).
				Tooltip(fmt.Sprintf("Следующий Z-уровень (%s)", shortcut.Combine(platform.KeyModName(), "Вверх"))).
				Round(true),
		}),
	}
}
