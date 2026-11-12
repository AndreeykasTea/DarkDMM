package menu

import (
	"sdmm/internal/app/command"
	"sdmm/internal/app/ui/shortcut"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmclip"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/platform"
	"sdmm/internal/rsc"

	"github.com/SpaiR/imgui-go"
)

//goland:noinspection GoCommentStart
type app interface {
	// File
	DoNewWorkspace()
	DoNewMap()
	DoOpen()
	DoLoadResource(path string)
	DoClearRecentMaps()
	DoCloseEnvironment()
	DoClose()
	DoCloseAll()
	DoSave()
	DoSaveAll()
	DoOpenPreferences()
	DoExit()

	// Edit
	DoUndo()
	DoRedo()
	DoCopy()
	DoPaste()
	DoCut()
	DoDelete()
	DoSearch()
	DoDeselect()
	DoOpenJumpWindow()

	// View
	DoAreaBorders()
	DoMultiZRendering()
	DoLighting()
	DoAnimations()
	DoShowUnderFloor()
	DoOpenReplaceWindow()
	DoOpenLintWindow()
	DoOpenTemplatesWindow()
	DoMirrorCanvasCamera()

	// Window
	DoResetLayout()

	// Help
	DoOpenChangelog()
	DoOpenAbout()
	DoOpenLogs()
	DoOpenSourceCode()
	DoCheckForUpdates()
	DoOpenSupport()

	// Other
	DoSelfUpdate()
	DoRestart()
	DoIgnoreUpdate()

	// Helpers

	RecentMapsByLoadedEnvironment() []string
	RecentMaps() []string

	LoadedEnvironment() *dmenv.Dme
	HasLoadedEnvironment() bool

	HasActiveMap() bool

	PathsFilter() *dm.PathsFilter
	CommandStorage() *command.Storage
	Clipboard() *dmmclip.Clipboard

	AreaBordersRendering() bool
	MultiZRendering() bool
	LightingEnabled() bool
	AnimationsEnabled() bool
	ShowUnderFloor() bool
	MirrorCanvasCamera() bool
}

type upStatus int

const (
	upStatusNone upStatus = iota
	upStatusAvailable
	upStatusUpdating
	upStatusUpdated
	upStatusError
)

type Menu struct {
	app app

	shortcuts shortcut.Shortcuts

	updateStatus      upStatus
	updateVersion     string
	updateDescription string
}

func New(app app) *Menu {
	m := &Menu{app: app}
	m.addShortcuts()
	return m
}

func (m *Menu) Process() {
	w.MainMenuBar(w.Layout{
		w.Menu("Файл", w.Layout{
			w.MenuItem("Новая рабочая область", m.app.DoNewWorkspace).
				Icon(icon.File).
				Shortcut(platform.KeyModName(), "N"),
			w.MenuItem("Новая карта", m.app.DoNewMap).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()),
			w.Separator(),
			w.MenuItem("Открыть...", m.app.DoOpen).
				Icon(icon.FolderOpen).
				Shortcut(platform.KeyModName(), "O"),
			w.Menu("Недавние карты", w.Layout{
				w.Custom(func() {
					for _, recentMap := range m.app.RecentMaps() {
						w.MenuItem(recentMap, func() {
							m.app.DoLoadResource(recentMap)
						}).IconEmpty().Build()
					}
					w.Layout{
						w.Separator(),
						w.MenuItem("Очистить недавние карты", m.app.DoClearRecentMaps).
							Icon(icon.Delete),
					}.Build()
				}),
			}).Icon(icon.AccessTime).Enabled(len(m.app.RecentMaps()) != 0),
			w.MenuItem("Закрыть окружение", m.app.DoCloseEnvironment).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()),
			w.Separator(),
			w.MenuItem("Закрыть", m.app.DoClose).
				IconEmpty().
				Shortcut(platform.KeyModName(), "W"),
			w.MenuItem("Закрыть все", m.app.DoCloseAll).
				IconEmpty().
				Shortcut(platform.KeyModName(), "Shift", "W"),
			w.Separator(),
			w.MenuItem("Сохранить", m.app.DoSave).
				Icon(icon.Save).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "S"),
			w.MenuItem("Сохранить все", m.app.DoSaveAll).
				Icon(icon.Save).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "Shift", "S"),
			w.Separator(),
			w.MenuItem("Настройки", m.app.DoOpenPreferences).
				Icon(icon.Wrench),
			w.Separator(),
			w.MenuItem("Выход", m.app.DoExit).
				IconEmpty().
				Shortcut(shortcut.Combine(platform.KeyModName(), "Q")),
		}),

		w.Menu("Правка", w.Layout{
			w.MenuItem("Отменить", m.app.DoUndo).
				Icon(icon.Undo).
				Enabled(m.app.CommandStorage().HasUndo()).
				Shortcut(platform.KeyModName(), "Z"),
			w.MenuItem("Повторить", m.app.DoRedo).
				Icon(icon.Redo).
				Enabled(m.app.CommandStorage().HasRedo()).
				Shortcut(platform.KeyModName(), "Shift", "Z"),
			w.Separator(),
			w.MenuItem("Копировать", m.app.DoCopy).
				Icon(icon.ContentCopy).
				Shortcut(platform.KeyModName(), "C"),
			w.MenuItem("Вставить", m.app.DoPaste).
				Icon(icon.ContentPaste).
				Enabled(m.app.Clipboard().HasData()).
				Shortcut(platform.KeyModName(), "V"),
			w.MenuItem("Вырезать", m.app.DoCut).
				Icon(icon.ContentCut).
				Shortcut(platform.KeyModName(), "X"),
			w.MenuItem("Удалить", m.app.DoDelete).
				Icon(icon.Eraser).
				Shortcut("Del"),
			w.MenuItem("Снять выбор", m.app.DoDeselect).
				IconEmpty().
				Shortcut(platform.KeyModName(), "D"),
			w.Separator(),
			w.MenuItem("Поиск", m.app.DoSearch).
				Icon(icon.Search).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "F"),
			w.MenuItem("Перейти к координатам", m.app.DoOpenJumpWindow).
				Icon(icon.Shrink).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "G"),
			w.MenuItem("Замена объектов...", m.app.DoOpenReplaceWindow).
				Icon(icon.Repeat).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "H"),
			w.MenuItem("Шаблоны...", m.app.DoOpenTemplatesWindow).
				Icon(icon.ContentPaste).
				Enabled(m.app.HasActiveMap()).
				Shortcut(platform.KeyModName(), "T"),
			w.MenuItem("Проверить карту", m.app.DoOpenLintWindow).
				Icon(icon.Search).
				Enabled(m.app.HasActiveMap()).
				Shortcut("F7"),
		}),

		w.Menu("Вид", w.Layout{
			w.MenuItem("Показывать зоны (area)", m.doToggleArea).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()).
				Selected(m.isAreaToggled()).
				Shortcut(platform.KeyModName(), "1"),
			w.MenuItem("Показывать турфы (turf)", m.doToggleTurf).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()).
				Selected(m.isTurfToggled()).
				Shortcut(platform.KeyModName(), "2"),
			w.MenuItem("Показывать объекты (obj)", m.doToggleObject).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()).
				Selected(m.isObjectToggled()).
				Shortcut(platform.KeyModName(), "3"),
			w.MenuItem("Показывать мобов (mob)", m.doToggleMob).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()).
				Selected(m.isMobToggled()).
				Shortcut(platform.KeyModName(), "4"),
			w.MenuItem("Показать всё", m.doShowAll).
				IconEmpty().
				Enabled(m.app.HasLoadedEnvironment()),
			w.Separator(),
			w.MenuItem("Границы зон", m.app.DoAreaBorders).
				IconEmpty().
				Selected(m.app.AreaBordersRendering()),
			w.MenuItem("Отрисовка нижних Z-уровней", m.app.DoMultiZRendering).
				IconEmpty().
				Selected(m.app.MultiZRendering()).
				Shortcut(platform.KeyModName(), "0"),
			w.MenuItem("Синхронизировать камеры карт", m.app.DoMirrorCanvasCamera).
				IconEmpty().
				Selected(m.app.MirrorCanvasCamera()),
			w.Separator(),
			w.MenuItem("Провода и трубы под полом", m.app.DoShowUnderFloor).
				IconEmpty().
				Selected(m.app.ShowUnderFloor()),
			w.MenuItem("Анимации иконок", m.app.DoAnimations).
				IconEmpty().
				Selected(m.app.AnimationsEnabled()),
			w.MenuItem("Предпросмотр освещения", m.app.DoLighting).
				Icon(icon.Lightbulb).
				Selected(m.app.LightingEnabled()).
				Shortcut(platform.KeyModName(), "L"),
		}),

		w.Menu("Окно", w.Layout{
			w.MenuItem("Сбросить раскладку окон", m.app.DoResetLayout).Shortcut("F5").
				Icon(icon.WindowRestore),
		}),

		w.Menu("Справка", w.Layout{
			w.MenuItem("Список изменений", m.app.DoOpenChangelog).
				Icon(icon.ClipboardMultiple),
			w.MenuItem("Исходный код", m.app.DoOpenSourceCode).
				Icon(icon.GitHub),
			w.Separator(),
			w.MenuItem("Открыть папку логов", m.app.DoOpenLogs).
				IconEmpty(),
			w.MenuItem("О программе", m.app.DoOpenAbout).
				IconEmpty(),
			w.Separator(),
			w.Button("Поддержать", m.app.DoOpenSupport).
				Size(imgui.Vec2{X: -1}).
				Style(style.ButtonFireCoral{}).
				Tooltip(rsc.SupportTxt).
				Icon(icon.KoFi),
		}),

		w.Custom(func() {
			if m.updateStatus != upStatusNone {
				m.showUpdateMenu()
			}
		}),
	}).Build()
}

func (m *Menu) SetUpdateAvailable(version, description string) {
	m.updateStatus = upStatusAvailable
	m.updateVersion = version
	m.updateDescription = description
}

func (m *Menu) SetUpdating() {
	m.updateStatus = upStatusUpdating
}

func (m *Menu) SetUpdated() {
	m.updateStatus = upStatusUpdated
}

func (m *Menu) SetUpdateError() {
	m.updateStatus = upStatusError
}

func (m *Menu) doToggleArea() {
	m.app.PathsFilter().TogglePath("/area")
}

func (m *Menu) doToggleTurf() {
	m.app.PathsFilter().TogglePath("/turf")
}

func (m *Menu) doToggleObject() {
	m.app.PathsFilter().TogglePath("/obj")
}

func (m *Menu) doToggleMob() {
	m.app.PathsFilter().TogglePath("/mob")
}

func (m *Menu) doShowAll() {
	m.app.PathsFilter().Clear()
}

func (m *Menu) isAreaToggled() bool {
	return m.app.PathsFilter().IsVisiblePath("/area")
}

func (m *Menu) isTurfToggled() bool {
	return m.app.PathsFilter().IsVisiblePath("/turf")
}

func (m *Menu) isObjectToggled() bool {
	return m.app.PathsFilter().IsVisiblePath("/obj")
}

func (m *Menu) isMobToggled() bool {
	return m.app.PathsFilter().IsVisiblePath("/mob")
}
