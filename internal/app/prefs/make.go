package prefs

import (
	"math"

	"sdmm/internal/app/ui/cpwsarea/wsprefs"
	"sdmm/internal/app/window"
)

type App interface {
	UpdateScale()
}

func Make(app App, prefs *Prefs) wsprefs.Prefs {
	p := wsprefs.MakePrefs()

	var preferencesPrefabs = map[wsprefs.PrefGroup][]prefPrefab{
		wsprefs.GPEditor: {
			optionPrefPrefab{
				name:    "Формат сохранения",
				desc:    "Формат, в котором редактор сохраняет карту.",
				label:   "##save_format",
				value:   &prefs.Editor.SaveFormat,
				options: SaveFormats,
				help:    SaveFormatHelp,
			},
			optionPrefPrefab{
				name:    "Редактор кода",
				desc:    "Какой редактор кода открывать при «Перейти к определению».",
				label:   "##code_editor",
				value:   &prefs.Editor.CodeEditor,
				options: CodeEditors,
				help:    CodeEditorHelp,
			},
			boolPrefPrefab{
				name:  "Очищать переменные",
				desc:  "Удалять с карты переменные, значение которых совпадает с исходным.",
				label: "##sanitize_variables",
				value: &prefs.Editor.SanitizeVariables,
			},
			intPrefPrefab{
				name:  "Автосохранение (минут)",
				desc:  "Копии изменённых карт сохраняются в папку backup/autosave. Исходные файлы не меняются. 0 - выключено.",
				label: "##autosave_minutes",
				min:   0,
				max:   120,
				value: &prefs.Editor.AutosaveMinutes,
			},
			optionPrefPrefab{
				name:    "Режим сдвига",
				desc:    "Какие переменные меняются при сдвиге объекта.",
				label:   "##nudge_mode",
				value:   &prefs.Editor.NudgeMode,
				options: SaveNudgeModes,
			},
		},

		wsprefs.GPControls: {
			boolPrefPrefab{
				name:  "Альтернативная прокрутка",
				desc:  "Колесо мыши двигает карту. Масштаб — колесо с зажатым пробелом.",
				label: "##alternative_scroll_behavior",
				value: &prefs.Controls.AltScrollBehaviour,
			},
			boolPrefPrefab{
				name:  "Быстрая правка: в меню тайла",
				desc:  "Показывать быструю правку в контекстном меню тайла.",
				label: "##quick_edit:tile_context_menu",
				value: &prefs.Controls.QuickEditContextMenu,
			},
			boolPrefPrefab{
				name:  "Быстрая правка: на панели карты",
				desc:  "Показывать быструю правку на панели карты.",
				label: "##quick_edit:map_pane",
				value: &prefs.Controls.QuickEditMapPane,
			},
		},

		wsprefs.GPInterface: {
			intPrefPrefab{
				name:  "Масштаб",
				desc:  "Масштаб интерфейса.",
				label: "%##scale",
				min:   50,
				max:   250,
				value: &prefs.Interface.Scale,
				post: func(int) {
					app.UpdateScale()
				},
			},
			intPrefPrefab{
				name:  "Кадров в секунду",
				desc:  "Частота кадров приложения.",
				label: "##fps",
				min:   30,
				max:   math.MaxInt,
				value: &prefs.Interface.Fps,
				post:  window.SetFps,
			},
		},
	}

	for group, prefabs := range preferencesPrefabs {
		for _, prefab := range prefabs {
			p.Add(group, prefab.make())
		}
	}

	return p
}
