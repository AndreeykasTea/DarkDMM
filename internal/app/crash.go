package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"sdmm/internal/app/prefs"
	"sdmm/internal/app/ui/cpwsarea/wsmap"
	"sdmm/internal/dmapi/dmmsave"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

// recoverCrash handles unexpected panics in the main thread.
// Before closing the editor it saves all modified maps to a separate folder, so the user won't lose the work.
func (a *app) recoverCrash() {
	r := recover()
	if r == nil {
		return
	}

	log.Error().Msgf("editor crashed: %v\n%s", r, debug.Stack())

	savedTo := a.emergencySave()

	msg := fmt.Sprintf("Редактор аварийно завершил работу:\n%v\n\nЛоги: %s", r, a.logDir)
	if savedTo != "" {
		msg += "\n\nНесохранённые карты записаны в папку:\n" + savedTo
	}
	util.ShowErrorDialog(msg)
	os.Exit(1)
}

// emergencySave saves all modified maps into the crash folder. Returns the folder path or an empty string.
func (a *app) emergencySave() (dir string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("emergency save failed: %v", r)
			dir = ""
		}
	}()

	if a.layout == nil || a.layout.WsArea == nil || a.loadedEnvironment == nil {
		return ""
	}

	dir = filepath.Join(a.backupDir, "crash", time.Now().Format(util.TimeFormat))

	saved := 0
	for _, workspace := range a.layout.WsArea.MapWorkspaces() {
		ws, ok := workspace.Content().(*wsmap.WsMap)
		if !ok || !a.commandStorage.IsModified(ws.CommandStackId()) {
			continue
		}
		dmm := ws.Map().Dmm()
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return ""
		}
		dst := filepath.Join(dir, dmm.Name)
		log.Print("emergency save:", dst)
		dmmsave.SaveV(a.loadedEnvironment, dmm, dst, dmmsave.Config{
			Format:            saveFormatFromPrefs(a.Prefs().Editor.SaveFormat),
			SanitizeVariables: a.Prefs().Editor.SanitizeVariables,
		})
		saved++
	}

	if saved == 0 {
		return ""
	}
	return dir
}

func saveFormatFromPrefs(format string) dmmsave.Format {
	switch format {
	case prefs.SaveFormatTGM:
		return dmmsave.FormatTGM
	case prefs.SaveFormatDMM:
		return dmmsave.FormatDM
	}
	return dmmsave.FormatInitial
}
