package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sdmm/internal/app/ui/cpwsarea/wsmap"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmmsave"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

// How many autosaves are kept for every map.
const autosaveKeep = 10

var (
	lastAutosave = time.Now()
	// Map path -> editor changes counter at the moment of the last autosave.
	autosavedChanges = make(map[string]int)
	autosaveRunning  bool
)

func (a *app) initAutosave() {
	window.RunRepeat(a.processAutosave)
}

func (a *app) processAutosave() {
	minutes := a.Prefs().Editor.AutosaveMinutes
	if minutes <= 0 || autosaveRunning || a.loadedEnvironment == nil {
		return
	}
	if time.Since(lastAutosave) < time.Duration(minutes)*time.Minute {
		return
	}
	lastAutosave = time.Now()

	env := a.loadedEnvironment
	cfg := dmmsave.Config{
		Format:            saveFormatFromPrefs(a.Prefs().Editor.SaveFormat),
		SanitizeVariables: a.Prefs().Editor.SanitizeVariables,
	}

	for _, workspace := range a.layout.WsArea.MapWorkspaces() {
		ws, ok := workspace.Content().(*wsmap.WsMap)
		if !ok {
			continue
		}
		editor := ws.Map().Editor()
		dmm := ws.Map().Dmm()
		key := dmm.Path.Absolute

		// Only maps with unsaved changes made after the last autosave.
		if !a.commandStorage.IsModified(ws.CommandStackId()) || autosavedChanges[key] == editor.Changes() {
			continue
		}
		autosavedChanges[key] = editor.Changes()

		dir := filepath.Join(a.backupDir, "autosave", a.environmentName(), dmm.Name)
		dst := filepath.Join(dir, time.Now().Format(util.TimeFormat)+".dmm")

		// The copy is made in the main thread, the saving itself is done in the background.
		dmmCopy := dmm.Copy()
		autosaveRunning = true
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("autosave failed [%s]: %v", dst, r)
				}
				window.RunLater(func() { autosaveRunning = false })
			}()
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				log.Print("unable to create autosave dir:", err)
				return
			}
			start := time.Now()
			dmmsave.SaveV(env, &dmmCopy, dst, cfg)
			log.Printf("autosaved [%s] in [%d] ms", dst, time.Since(start).Milliseconds())
			cleanupAutosaves(dir)
		}()
		return // One map per autosave tick, so the background job won't overlap.
	}
}

// cleanupAutosaves removes old autosaves, keeping only the latest ones.
func cleanupAutosaves(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".dmm") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files) // Names are timestamps, so the order is chronological.
	for len(files) > autosaveKeep {
		_ = os.Remove(filepath.Join(dir, files[0]))
		files = files[1:]
	}
}
