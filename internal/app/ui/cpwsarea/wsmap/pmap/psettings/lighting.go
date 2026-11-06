package psettings

import (
	"fmt"

	"sdmm/internal/app/render/lighting"

	"github.com/SpaiR/imgui-go"
)

func (p *Panel) showLighting() {
	if !imgui.CollapsingHeader("Освещение") {
		return
	}

	s := lighting.Current

	imgui.Checkbox("Включить предпросмотр##lighting_enabled", &s.Enabled)

	ambient := int32(s.Ambient)
	imgui.Text("Минимальная яркость")
	imgui.SetNextItemWidth(-1)
	if imgui.SliderIntV("##lighting_ambient", &ambient, 0, 100, "%d%%", imgui.SliderFlagsAlwaysClamp) {
		s.Ambient = int(ambient)
	}

	intensity := int32(s.Intensity)
	imgui.Text("Сила света")
	imgui.SetNextItemWidth(-1)
	if imgui.SliderIntV("##lighting_intensity", &intensity, 10, 300, "%d%%", imgui.SliderFlagsAlwaysClamp) {
		s.Intensity = int(intensity)
	}

	imgui.Checkbox("Показывать источники##lighting_sources", &s.ShowSources)

	if p.editor.LightingPowerAware() {
		imgui.TextDisabled("Модель света: Baystation")
		imgui.Checkbox("Лампы горят без APC##lighting_ignore_power", &s.IgnorePower)
		if imgui.IsItemHovered() {
			imgui.SetTooltip("В игре лампа в зоне без APC не горит. Включите, чтобы показать свет так, будто питание есть везде.")
		}
	}

	if s.Enabled {
		if count := p.editor.LightingSourcesCount(); count >= 0 {
			imgui.TextDisabled(fmt.Sprintf("Источников света на уровне: %d", count))
			if off := p.editor.LightingUnpoweredCount(); off > 0 {
				imgui.TextDisabled(fmt.Sprintf("Без питания (нет APC): %d", off))
				if imgui.IsItemHovered() {
					imgui.SetTooltip("Эти лампы и приборы в игре не будут светить: в их зоне нет APC.\nС включёнными источниками они обведены красной рамкой.")
				}
			}
		} else {
			imgui.TextDisabled("Вычисление...")
		}
	}
}
