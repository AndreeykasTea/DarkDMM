package wsmap

import (
	"sdmm/internal/app/prefs"
	"sdmm/internal/dmapi/dmmsave"

	"github.com/rs/zerolog/log"
)

func (ws *WsMap) Save() bool {
	log.Print("saving map workspace:", ws.CommandStackId())

	editorPrefs := ws.app.Prefs().Editor

	var saveFormat dmmsave.Format
	switch editorPrefs.SaveFormat {
	case prefs.SaveFormatInitial:
		saveFormat = dmmsave.FormatInitial
	case prefs.SaveFormatTGM:
		saveFormat = dmmsave.FormatTGM
	case prefs.SaveFormatDMM:
		saveFormat = dmmsave.FormatDM
	}

	err := dmmsave.Save(ws.app.LoadedEnvironment(), ws.paneMap.Dmm(), dmmsave.Config{
		Format:            saveFormat,
		SanitizeVariables: editorPrefs.SanitizeVariables,
	})
	if err != nil {
		// The map is still unsaved, so the user won't lose changes on close.
		log.Print("map is not saved:", err)
		return false
	}

	ws.app.CommandStorage().ForceBalance(ws.CommandStackId())

	// Check the saved map the same way the CI does, so problems are visible before the push.
	ws.paneMap.LintAfterSave()
	return true
}
