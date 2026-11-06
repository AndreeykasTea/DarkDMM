package tilemenu

import (
	"fmt"

	"sdmm/internal/app/ui/layout/lnode"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/imguiext/icon"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/platform"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

func (t *TileMenu) Process() {
	t.shortcuts.SetVisible(t.opened)

	if !t.opened {
		return
	}

	if imgui.BeginPopup("tileMenu") {
		t.showControls()
		imgui.EndPopup()
	} else {
		t.close()
	}
	if !imgui.IsWindowHovered() && imgui.IsMouseClicked(imgui.MouseButtonMiddle) {
		t.close()
	}
}

func (t *TileMenu) showControls() {
	w.Layout{
		w.MenuItem(t.tile.Coord.String(), nil).Icon(icon.Help).Enabled(false),
		w.Separator(),
		w.MenuItem("Отменить", t.app.DoUndo).
			Icon(icon.Undo).
			Enabled(t.app.CommandStorage().HasUndo()).
			Shortcut(platform.KeyModName(), "Z"),
		w.MenuItem("Повторить", t.app.DoRedo).
			Icon(icon.Redo).
			Enabled(t.app.CommandStorage().HasRedo()).
			Shortcut(platform.KeyModName(), "Shift", "Z"),
		w.Separator(),
		w.MenuItem("Копировать", t.app.DoCopy).
			Icon(icon.ContentCopy).
			Shortcut(platform.KeyModName(), "C"),
		w.MenuItem("Вставить", t.app.DoPaste).
			Icon(icon.ContentPaste).
			Enabled(t.app.Clipboard().HasData()).
			Shortcut(platform.KeyModName(), "V"),
		w.MenuItem("Вырезать", t.app.DoCut).
			Icon(icon.ContentCut).
			Shortcut(platform.KeyModName(), "X"),
		w.MenuItem("Удалить", t.app.DoDelete).
			Icon(icon.Eraser).
			Shortcut("Del"),
		w.Separator(),
		w.Custom(func() {
			for idx, instance := range t.tile.Instances().Sorted() {
				t.showInstance(instance, idx)
			}
		}),
	}.Build()
}

func (t *TileMenu) showInstance(i *dmminstance.Instance, idx int) {
	p := i.Prefab()
	s := getSprite(p)
	iconSize := t.iconSize()
	r, g, b, _ := util.ParseColor(p.Vars().TextV("color", "")).RGBA()
	name := fmt.Sprintf("%s##prefab_row_%d", p.Vars().TextV("name", ""), idx)

	w.Layout{
		w.Menu(name, t.showInstanceControls(i, idx)).
			IconEmpty(),
		w.Custom(func() {
			// Draw the instance icon.
			imgui.WindowDrawList().AddImageV(
				imgui.TextureID(s.Texture()),
				imgui.ItemRectMin(),
				imgui.ItemRectMin().Plus(imgui.Vec2{X: iconSize, Y: iconSize}),
				imgui.Vec2{X: s.U1, Y: s.V1}, imgui.Vec2{X: s.U2, Y: s.V2},
				imgui.PackedColorFromVec4(imgui.Vec4{X: r, Y: g, Z: b, W: 1}),
			)
		}),
		w.SameLine(),
		w.Text(fmt.Sprintf("[%s]\t\t", p.Path())),
	}.Build()
}

func (t *TileMenu) showInstanceControls(i *dmminstance.Instance, idx int) w.Layout {
	p := i.Prefab()

	return w.Layout{
		w.Custom(func() {
			if t.app.Prefs().Controls.QuickEditContextMenu {
				t.pQuickEdit.ProcessV(i)
				imgui.Separator()
			}
		}),
		w.Custom(func() {
			if dm.IsPath(p.Path(), "/obj") || dm.IsPath(p.Path(), "/mob") {
				w.Layout{
					w.MenuItem(fmt.Sprint("Поднять наверх##move_to_top_", idx), t.doMoveToTop(i)).
						Icon(icon.ArrowUpward),
					w.MenuItem(fmt.Sprint("Опустить вниз##move_to_bottom_", idx), t.doMoveToBottom(i)).
						Icon(icon.ArrowDownward),
					w.Separator(),
				}.Build()
			}
		}),
		w.MenuItem(fmt.Sprint("Выбрать##select_", idx), t.doSelect(i)).
			Icon(icon.EyeDropper).
			Shortcut("S"),
		w.MenuItem(fmt.Sprint("Удалить##delete_", idx), t.doDelete(i)).
			Icon(icon.Eraser).
			Shortcut("D"),
		w.MenuItem(fmt.Sprint("Заменить##replace_", idx), t.doReplace(i)).
			Icon(icon.Repeat).
			Shortcut("R").
			Enabled(t.app.HasSelectedPrefab()),
		w.MenuItem(fmt.Sprint("Заменить на...##replace_any_", idx), func() {
			if t.onOpenReplace != nil {
				t.onOpenReplace(i)
			}
		}).
			Icon(icon.Repeat),
		w.MenuItem(fmt.Sprint("Сбросить к исходному##reset_to_default_", idx), t.doResetToDefault(i)).
			IconEmpty(),
		w.Custom(func() {
			_, hasAccess := p.Vars().Value("req_access")
			if _, ok := p.Vars().Value("req_one_access"); ok {
				hasAccess = true
			}
			if hasAccess && t.onOpenAccess != nil {
				w.MenuItem(fmt.Sprint("Доступы...##access_", idx), func() {
					t.onOpenAccess(i)
				}).Icon(icon.Lock).Build()
			}
		}),
		w.Separator(),
		w.MenuItem(fmt.Sprint("Искать по типу##search_by_type_", idx), t.doSearchByType(i)).
			Icon(icon.Search),
		w.MenuItem(fmt.Sprint("Искать по ID префаба##search_by_prefab_id_", idx), t.doSearchByPrefabID(i)).
			Icon(icon.Search),
	}
}

func (t *TileMenu) doMoveToTop(i *dmminstance.Instance) func() {
	return func() {
		log.Printf("do move instance[%s] to top: %d", i.Prefab().Path(), i.Id())
		t.editor.InstanceMoveToTop(i)
		t.editor.CommitChanges("Поднять наверх")
	}
}

func (t *TileMenu) doMoveToBottom(i *dmminstance.Instance) func() {
	return func() {
		log.Printf("do move instance[%s] to bottom: %d", i.Prefab().Path(), i.Id())
		t.editor.InstanceMoveToBottom(i)
		t.editor.CommitChanges("Опустить вниз")
	}
}

func (t *TileMenu) doSelect(i *dmminstance.Instance) func() {
	return func() {
		log.Printf("do select instance[%s]: %d", i.Prefab().Path(), i.Id())
		t.editor.InstanceSelect(i)
	}
}

func (t *TileMenu) doDelete(i *dmminstance.Instance) func() {
	return func() {
		log.Printf("do delete instance[%s]: %d", i.Prefab().Path(), i.Id())
		t.editor.InstanceDelete(i)
		t.editor.CommitChanges("Удаление объекта")
	}
}

func (t *TileMenu) doReplace(i *dmminstance.Instance) func() {
	return func() {
		if prefab, ok := t.app.SelectedPrefab(); ok {
			log.Printf("do replace instance[%s] with [%s]: %d", i.Prefab().Path(), prefab.Path(), i.Id())
			t.editor.InstanceReplace(i, prefab)
			t.editor.CommitChanges("Замена объекта")
		}
	}
}

func (t *TileMenu) doResetToDefault(i *dmminstance.Instance) func() {
	return func() {
		log.Printf("do reset instance[%s] to default: %d", i.Prefab().Path(), i.Id())
		t.editor.InstanceReset(i)
		t.editor.CommitChanges("Сброс объекта")
	}
}

func (t *TileMenu) doSearchByType(i *dmminstance.Instance) func() {
	return func() {
		log.Print("do search prefab by type:", i.Prefab().Path())
		t.app.ShowLayout(lnode.NameSearch, true)
		t.app.DoSearchPrefabByPath(i.Prefab().Path())
	}
}

func (t *TileMenu) doSearchByPrefabID(i *dmminstance.Instance) func() {
	return func() {
		log.Print("do search prefab by ID:", i.Prefab().Id())
		t.app.ShowLayout(lnode.NameSearch, true)
		t.app.DoSearchPrefab(i.Prefab().Id())
	}
}

func getSprite(i *dmmprefab.Prefab) *dmicon.Sprite {
	return dmicon.Cache.GetSpriteOrPlaceholderV(
		i.Vars().TextV("icon", ""),
		i.Vars().TextV("icon_state", ""),
		dm.FixDir(i.Vars().IntV("dir", dm.DirDefault)),
	)
}

func (t *TileMenu) iconSize() float32 {
	return imgui.FrameHeight() - imgui.CurrentStyle().FramePadding().Y
}
