package pmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmclip"
	"sdmm/internal/dmapi/dmmtemplate"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

// TemplatesDir is a root folder for templates. Every environment has its own subfolder.
var TemplatesDir string

// State of the template saving, shared by all maps.
var templateSave = struct {
	name      string
	withAreas bool
	withTurfs bool
	message   string
}{withTurfs: true}

type templatesWindow struct {
	open    bool
	list    []dmmtemplate.Info
	filter  string
	message string
}

func (p *PaneMap) templatesDir() string {
	return filepath.Join(TemplatesDir, p.app.LoadedEnvironment().Name)
}

// OpenTemplatesWindow opens the library of templates.
func (p *PaneMap) OpenTemplatesWindow() {
	p.templatesWin.open = true
	p.templatesWin.list = dmmtemplate.List(p.templatesDir())
}

// showSaveTemplate is shown in the selection panel.
func (p *PaneMap) showSaveTemplate() {
	imgui.SetNextItemWidth(window.PointSize() * 160)
	imgui.InputTextWithHint("##template_name", "Название шаблона", &templateSave.name)
	imgui.SameLine()
	if imgui.Button("Сохранить как шаблон") {
		p.doSaveTemplate()
	}
	imgui.SameLine()
	imgui.Checkbox("зоны##template_areas", &templateSave.withAreas)
	imgui.SameLine()
	imgui.Checkbox("турфы##template_turfs", &templateSave.withTurfs)
	imgui.SameLine()
	if imgui.Button("Библиотека##open_templates") {
		p.OpenTemplatesWindow()
	}
	if templateSave.message != "" {
		imgui.TextDisabled(templateSave.message)
	}
}

func (p *PaneMap) doSaveTemplate() {
	grab, ok := tools.Selected().(*tools.ToolGrab)
	if !ok || !grab.HasSelectedArea() {
		return
	}
	name := strings.TrimSpace(templateSave.name)
	if name == "" {
		templateSave.message = "Введите название шаблона"
		return
	}

	b := grab.Bounds()
	filter := p.app.PathsFilter()
	var tiles []dmmap.Tile
	for x := int(b.X1); x <= int(b.X2); x++ {
		for y := int(b.Y1); y <= int(b.Y2); y++ {
			coord := util.Point{X: x, Y: y, Z: p.activeLevel}
			if !p.dmm.HasTile(coord) {
				continue
			}
			tile := p.dmm.GetTile(coord).Copy()
			// Only visible objects are saved, the same as the copy does.
			var visible dmmap.Instances
			for _, i := range tile.Instances() {
				if filter.IsVisiblePath(i.Prefab().Path()) {
					visible = append(visible, i)
				}
			}
			tile.Set(visible)
			tiles = append(tiles, tile)
		}
	}

	tpl := dmmtemplate.Make(name, tiles, dmmtemplate.Options{WithAreas: templateSave.withAreas, WithTurfs: templateSave.withTurfs})
	path, err := dmmtemplate.Save(p.templatesDir(), tpl)
	if err != nil {
		templateSave.message = "Не удалось сохранить: " + err.Error()
		return
	}
	templateSave.message = fmt.Sprintf("Шаблон «%s» %dx%d сохранён", name, tpl.Width, tpl.Height)
	templateSave.name = ""
	p.templatesWin.list = dmmtemplate.List(p.templatesDir())
	log.Print("template saved:", path)
}

func (p *PaneMap) processTemplatesWindow() {
	tw := &p.templatesWin
	if !tw.open {
		return
	}

	imgui.SetNextWindowSizeV(imgui.Vec2{X: window.PointSize() * 380, Y: window.PointSize() * 420}, imgui.ConditionFirstUseEver)
	open := true
	if imgui.BeginV("Шаблоны###templates_window_"+p.dmm.Name, &open, imgui.WindowFlagsNoDocking) {
		p.showTemplatesContent()
	}
	imgui.End()
	if !open {
		tw.open = false
	}
}

func (p *PaneMap) showTemplatesContent() {
	tw := &p.templatesWin

	imgui.TextWrapped("Сохраняйте шаблоны из выделения (инструмент «Выделение»). Вставка идёт в буфер обмена: потом Ctrl+V на нужном месте, выделение можно повернуть клавишами [ и ].")
	if imgui.Button("Обновить") {
		tw.list = dmmtemplate.List(p.templatesDir())
	}
	imgui.SameLine()
	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##templates_filter", "Поиск", &tw.filter)
	if tw.message != "" {
		imgui.TextColored(style.ColorGold, tw.message)
	}
	imgui.Separator()

	if len(tw.list) == 0 {
		imgui.TextDisabled("Шаблонов пока нет")
		return
	}

	filter := strings.ToLower(tw.filter)
	toDelete := ""
	imgui.BeginChildV("templates_list", imgui.Vec2{}, false, 0)
	for _, info := range tw.list {
		if filter != "" && !strings.Contains(strings.ToLower(info.Name), filter) {
			continue
		}
		imgui.PushID(info.File)
		if imgui.Button("Вставить") {
			p.pasteTemplate(info, true)
		}
		if imgui.IsItemHovered() {
			imgui.SetTooltip("Вставить у последнего тайла под курсором")
		}
		imgui.SameLine()
		if imgui.Button("В буфер") {
			p.pasteTemplate(info, false)
		}
		imgui.SameLine()
		if imgui.SmallButton("Удалить") {
			toDelete = info.File
		}
		imgui.SameLine()
		imgui.Text(fmt.Sprintf("%s  (%dx%d)", info.Name, info.Width, info.Height))
		imgui.PopID()
	}
	imgui.EndChild()

	if toDelete != "" {
		if err := os.Remove(toDelete); err != nil {
			tw.message = "Не удалось удалить: " + err.Error()
		}
		tw.list = dmmtemplate.List(p.templatesDir())
	}
}

// pasteTemplate puts the template to the clipboard and, if needed, pastes it to the last hovered tile.
func (p *PaneMap) pasteTemplate(info dmmtemplate.Info, pasteNow bool) {
	tw := &p.templatesWin
	tpl, err := dmmtemplate.Load(info.File)
	if err != nil {
		tw.message = err.Error()
		return
	}

	tiles, missing := tpl.ToTiles(p.app.LoadedEnvironment())

	// Objects which are not stored in the template must stay on the map, so they are excluded from the paste.
	filter := p.app.PathsFilter().Copy()
	if !tpl.HasAreas() && filter.IsVisiblePath("/area") {
		filter.TogglePath("/area")
	}
	if !tpl.HasTurfs() && filter.IsVisiblePath("/turf") {
		filter.TogglePath("/turf")
	}

	p.app.Clipboard().SetData(dmmclip.PasteData{Filter: filter, Buffer: tiles})

	tw.message = fmt.Sprintf("«%s» в буфере обмена", tpl.Name)
	if len(missing) > 0 {
		tw.message += fmt.Sprintf(", пропущено неизвестных типов: %d (%s)", len(missing), strings.Join(missing, ", "))
	}

	if pasteNow {
		p.editor.TilePasteSelected()
		p.editor.CommitChanges("Шаблон: " + tpl.Name)
	}
	log.Printf("template [%s] pasted, now: %t, missing types: %d", tpl.Name, pasteNow, len(missing))
}
