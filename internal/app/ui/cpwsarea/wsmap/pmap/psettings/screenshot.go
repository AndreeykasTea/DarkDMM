package psettings

import (
	"fmt"
	"image/png"
	"os"
	"time"

	"sdmm/internal/app/render/bucket/level/chunk/unit"
	"sdmm/internal/app/render/lighting"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/canvas"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	appdialog "sdmm/internal/app/ui/dialog"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/env"
	"sdmm/internal/imguiext"
	"sdmm/internal/imguiext/icon"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
	"github.com/sqweek/dialog"
	"golang.design/x/clipboard"
)

type sessionScreenshot struct {
	saving bool
}

func (p *Panel) showScreenshot() {
	if imgui.CollapsingHeader("Скриншот") {
		imgui.BeginDisabledV(cfg.ToClipboardMode)

		if imgui.Button(icon.FolderOpen) {
			p.selectScreenshotDir()
		}
		imguiext.SetItemHoveredTooltip("Папка скриншотов")

		imgui.SameLine()

		imgui.SetNextItemWidth(-1)
		imgui.InputText("##screenshot_dir", &cfg.ScreenshotDir)
		imguiext.SetItemHoveredTooltip(cfg.ScreenshotDir)

		imgui.EndDisabled()

		if imgui.Checkbox("Скриншот выделенной области", &cfg.InSelectionMode) {
			tools.SetSelected(tools.TNGrab)
		}

		imgui.Checkbox("В буфер обмена", &cfg.ToClipboardMode)

		var createBtnLabel string
		if p.sessionScreenshot.saving {
			createBtnLabel = "Создание" + []string{".", "..", "...", "...."}[int(imgui.Time()/.25)&3] + "###create"
		} else {
			createBtnLabel = icon.Save + "Создать###create"
		}

		w.Layout{
			w.Disabled(p.sessionScreenshot.saving,
				w.Button(createBtnLabel, p.createScreenshot).
					Size(imgui.Vec2{X: -1}).
					Style(style.ButtonGreen{}),
			),
		}.Build()
	}
}

func (p *Panel) createScreenshot() {
	p.sessionScreenshot.saving = true
	selectedTool := tools.Selected()

	boundX, boundY := float32(0), float32(0)

	var width, height int
	if cfg.InSelectionMode {
		hasSelectedArea := selectedTool.Name() == tools.TNGrab && selectedTool.(*tools.ToolGrab).HasSelectedArea()
		if hasSelectedArea {
			bounds := selectedTool.(*tools.ToolGrab).Bounds() //get grab tool bounds, so we can calculate boundX and boundY
			width, height = (int(bounds.X2-bounds.X1)+1)*dmmap.WorldIconSize, (int(bounds.Y2-bounds.Y1)+1)*dmmap.WorldIconSize
			boundX = -float32((int(bounds.X1) - 1) * dmmap.WorldIconSize) //now change bounds so we can use them in Translate
			boundY = -float32((int(bounds.Y1) - 1) * dmmap.WorldIconSize)
		} else {
			appdialog.Open(appdialog.TypeInformation{
				Title:       "Ничего не выделено!",
				Information: "Включён скриншот выделения, но ничего не выделено. Используйте инструмент «Выделение»!",
			})
			p.sessionScreenshot.saving = false
			return
		}
	} else {
		width, height = p.editor.Dmm().MaxX*dmmap.WorldIconSize, p.editor.Dmm().MaxY*dmmap.WorldIconSize
	}

	c := canvas.New()
	c.ClearColor = canvas.Color{} // Empty clear color with no alpha
	c.Render().Camera.Level = p.editor.ActiveLevel()
	c.Render().Camera.Translate(boundX, boundY)
	c.Render().SetUnitProcessor(p)
	for level := 1; level <= p.editor.ActiveLevel(); level++ {
		c.Render().UpdateBucket(p.editor.Dmm(), level) // Prepare for render all available levels
	}
	if lighting.Current.Enabled {
		c.Render().ComputeLightingNow() // The lighting is computed in the background usually, but here it must be ready right now.
	}
	c.Process(imgui.Vec2{X: float32(width), Y: float32(height)})
	c.Dispose()

	var pixels = c.ReadPixels()

	go func() {
		if err := p.saveScreenshot(pixels, width, height); err != nil {
			appdialog.Open(appdialog.TypeInformation{
				Title:       "Ошибка: создание скриншота",
				Information: fmt.Sprint("Не удалось создать скриншот:", err),
			})
		}
		p.sessionScreenshot.saving = false
	}()
}

func (p *Panel) selectScreenshotDir() {
	if dir, err := dialog.
		Directory().
		Title("Папка скриншотов").
		SetStartDir(cfg.ScreenshotDir).
		Browse(); err == nil {
		log.Print("screenshot directory selected:", dir)
		cfg.ScreenshotDir = dir
	}
}

func (p *Panel) ProcessUnit(u unit.Unit) bool {
	if p.editor.IsUnderFloorHidden(u.CoveredBy(), u.Instance().Coord()) {
		return false
	}
	return p.app.PathsFilter().IsVisiblePath(u.Instance().Prefab().Path())
}

func (p *Panel) saveScreenshot(pixels []byte, w, h int) error {
	if !cfg.ToClipboardMode {
		if err := os.MkdirAll(cfg.ScreenshotDir, os.ModeDir); err != nil {
			log.Print("unable to create screenshot directory:", err)
			return err
		}
	}

	var directory string
	if cfg.ToClipboardMode {
		directory = os.TempDir()
	} else {
		directory = cfg.ScreenshotDir
	}

	dstFilePath := directory + "/" + env.AppDir + "-" + time.Now().Format(util.TimeFormat) + ".png"

	if err := saveScreenshotToFile(dstFilePath, pixels, w, h); err != nil {
		log.Print("unable to save screenshot to file:", err)
		return err
	}

	if cfg.ToClipboardMode {
		if err := saveScreenshotToClipboard(dstFilePath); err != nil {
			log.Print("unable to save screenshot to clipboard:", err)
			return err
		}
	}

	return nil
}

func saveScreenshotToFile(dstFilePath string, pixels []byte, w, h int) error {
	out, err := os.Create(dstFilePath)
	if err != nil {
		log.Print("unable to create screenshot file:", dstFilePath, err)
		return err
	}

	err = png.Encode(out, util.PixelsToRGBA(pixels, w, h))
	defer out.Close()
	return err
}

func saveScreenshotToClipboard(srcFilePath string) error {
	if err := clipboard.Init(); err != nil {
		log.Print("unable to init clipboard:", err)
		return err
	}

	contents, err := os.ReadFile(srcFilePath)
	if err != nil {
		log.Print("unable to read resulting screenshot file:", err)
		return err
	}

	clipboard.Write(clipboard.FmtImage, contents)

	if err := os.Remove(srcFilePath); err != nil {
		log.Print("unable to delete clipboard screenshot file:", err)
		return err
	}

	return nil
}
