package pmap

import (
	"fmt"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"

	"github.com/SpaiR/imgui-go"
)

func (p *PaneMap) showScatterPanel() {
	s := tools.Scatter

	imgui.Dummy(imgui.Vec2{X: window.PointSize() * 230})

	imgui.Text("Разброс")
	imgui.Separator()

	mode := int(s.Mode)
	if imgui.RadioButtonInt("Область##scatter_mode_area", &mode, int(tools.ScatterModeArea)) {
		s.Mode = tools.ScatterMode(mode)
	}
	imgui.SameLine()
	if imgui.RadioButtonInt("Кисть##scatter_mode_brush", &mode, int(tools.ScatterModeBrush)) {
		s.Mode = tools.ScatterMode(mode)
	}

	imgui.SetNextItemWidth(-1)
	imgui.SliderIntV("##scatter_chance", &s.Chance, 1, 100, "Шанс: %d%%", imgui.SliderFlagsAlwaysClamp)

	if s.Mode == tools.ScatterModeBrush {
		imgui.SetNextItemWidth(-1)
		imgui.SliderIntV("##scatter_radius", &s.BrushRadius, 0, 20, "Радиус кисти: %d", imgui.SliderFlagsAlwaysClamp)
	}

	imgui.SetNextItemWidth(-1)
	imgui.SliderIntV("##scatter_spacing", &s.Spacing, 0, 10, "Мин. расстояние: %d", imgui.SliderFlagsAlwaysClamp)

	imgui.Checkbox("Только свободные тайлы##scatter_only_free", &s.OnlyFree)
	w.Layout{w.Tooltip(w.Text("Пропускать стены и тайлы с плотными (dense) объектами"))}.Build()
	imgui.Checkbox("Пропускать занятые##scatter_skip_occupied", &s.SkipOccupied)
	w.Layout{w.Tooltip(w.Text("Не ставить на тайлы, где уже есть объект из палитры"))}.Build()
	imgui.Checkbox("Случайное направление##scatter_random_dir", &s.RandomDir)
	if s.RandomDir {
		imgui.SameLine()
		imgui.Checkbox("8 направлений##scatter_random_dir8", &s.RandomDir8)
	}
	imgui.SetNextItemWidth(-1)
	imgui.SliderIntV("##scatter_jitter", &s.PixelJitter, 0, 16, "Сдвиг в пикселях: ±%d", imgui.SliderFlagsAlwaysClamp)

	imgui.Separator()
	imgui.Text("Палитра")

	if len(s.Palette) == 0 {
		imgui.TextDisabled("Пусто: используется выбранный объект")
	}

	toRemove := -1
	for idx := range s.Palette {
		entry := &s.Palette[idx]
		imgui.PushID(fmt.Sprint("scatter_entry_", idx))

		if entry.Prefab != nil {
			iconName, _ := entry.Prefab.Vars().Text("icon")
			iconState, _ := entry.Prefab.Vars().Text("icon_state")
			sprite := dmicon.Cache.GetSpriteOrPlaceholder(iconName, iconState)
			size := window.PointSize() * 20
			imgui.ImageV(imgui.TextureID(sprite.Texture()), imgui.Vec2{X: size, Y: size},
				imgui.Vec2{X: sprite.U1, Y: sprite.V1}, imgui.Vec2{X: sprite.U2, Y: sprite.V2},
				imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}, imgui.Vec4{})
			if imgui.IsItemHovered() {
				imgui.SetTooltip(entry.Prefab.Path())
			}
			imgui.SameLine()
		}

		imgui.SetNextItemWidth(window.PointSize() * 120)
		imgui.SliderIntV("##weight", &entry.Weight, 0, 100, "Вес: %d", imgui.SliderFlagsAlwaysClamp)
		imgui.SameLine()
		if imgui.Button(icon.Clear) {
			toRemove = idx
		}

		imgui.PopID()
	}
	if toRemove >= 0 {
		s.Palette = append(s.Palette[:toRemove], s.Palette[toRemove+1:]...)
	}

	selected, hasSelected := p.app.SelectedPrefab()
	w.Layout{
		w.Disabled(!hasSelected, w.Layout{
			w.Button(icon.Add+" Добавить выбранный", func() {
				for _, e := range s.Palette {
					if e.Prefab.Id() == selected.Id() {
						return
					}
				}
				s.Palette = append(s.Palette, tools.ScatterEntry{Prefab: selected, Weight: 10})
			}).Style(style.ButtonGreen{}),
		}),
		w.SameLine(),
		w.Disabled(len(s.Palette) == 0, w.Layout{
			w.Button("Очистить##scatter_clear", func() {
				s.Palette = nil
			}),
		}),
	}.Build()
}
