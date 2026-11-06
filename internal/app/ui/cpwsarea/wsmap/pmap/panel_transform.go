package pmap

import (
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/imguiext/style"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

var transformError string

func hasGrabSelection() bool {
	grab, ok := tools.Selected().(*tools.ToolGrab)
	return ok && grab.HasSelectedArea()
}

func (p *PaneMap) showTransformPanel() {
	imgui.Text("Выделение")
	imgui.SameLine()
	if imgui.Button("Повернуть влево##rot_ccw") {
		p.doTransformSelection(tools.OpRotateCCW)
	}
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Против часовой стрелки ( [ )")
	}
	imgui.SameLine()
	if imgui.Button("Повернуть вправо##rot_cw") {
		p.doTransformSelection(tools.OpRotateCW)
	}
	if imgui.IsItemHovered() {
		imgui.SetTooltip("По часовой стрелке ( ] )")
	}
	imgui.SameLine()
	if imgui.Button("Отразить по X##flip_x") {
		p.doTransformSelection(tools.OpFlipX)
	}
	imgui.SameLine()
	if imgui.Button("Отразить по Y##flip_y") {
		p.doTransformSelection(tools.OpFlipY)
	}
	if transformError != "" {
		imgui.TextColored(style.ColorRed, transformError)
	}
	p.showSaveTemplate()
}

func (p *PaneMap) doTransformSelection(op tools.TransformOp) {
	grab, ok := tools.Selected().(*tools.ToolGrab)
	if !ok || !grab.HasSelectedArea() {
		return
	}
	if err := grab.Transform(op); err != nil {
		transformError = err.Error()
		log.Print("unable to transform the selection:", err)
		return
	}
	transformError = ""
}
