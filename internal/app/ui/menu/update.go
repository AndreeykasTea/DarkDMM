package menu

import (
	"sdmm/internal/imguiext"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

var loadingDotTypes = []string{".", "..", "...", "...."}

func (m *Menu) showUpdateMenu() {
	w.Button(icon.SystemUpdate, nil).
		Style(style.ButtonFrame{}).
		TextColor(style.ColorGreen1Lighter).
		Build()

	imguiext.SetItemHoveredTooltip("Доступно обновление!")

	imgui.OpenPopupOnItemClickV("update_menu", imgui.PopupFlagsMouseButtonLeft)

	if imgui.BeginPopup("update_menu") {
		imgui.TextColored(style.ColorGold, m.updateVersion)
		if len(m.updateDescription) > 0 {
			imgui.Text(m.updateDescription)
		}
		imgui.Separator()

		switch m.updateStatus {
		case upStatusAvailable:
			m.showUpdateLayout()
		case upStatusUpdating:
			dotType := loadingDotTypes[(int(imgui.Time()/0.25) & 3)]
			imgui.Text("Обновление " + dotType)
		case upStatusUpdated:
			w.Button("Перезапустить", m.app.DoRestart).Build()
		case upStatusError:
			imgui.TextColored(style.ColorRed, "Что-то пошло не так.\nПопробуйте позже.")
			imgui.Separator()
			m.showUpdateLayout()
		}

		imgui.EndPopup()
	}
}

func (m *Menu) showUpdateLayout() {
	w.Layout{
		w.Button("Обновить", m.app.DoSelfUpdate).
			Style(style.ButtonGreen{}),
		w.SameLine(),
		w.Button("Скрыть", m.doHideUpdateButton),
		w.SameLine(),
		w.Button("Игнорировать", m.doIgnoreUpdate).
			Style(style.ButtonRed{}),
	}.Build()
}

func (m *Menu) doHideUpdateButton() {
	log.Print("do hide update")
	m.updateStatus = upStatusNone
}

func (m *Menu) doIgnoreUpdate() {
	log.Print("do ignore update")
	m.doHideUpdateButton()
	m.app.DoIgnoreUpdate()
}
