package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sdmm/third_party/sdmmparser"
	"sort"
	"time"

	"sdmm/internal/app/ui/cpwsarea/workspace"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/imguiext/style"
	w "sdmm/internal/imguiext/widget"

	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

func (a *app) loadResource(path string) {
	a.loadResourceV(path, nil)
}

// Universal method to open any editor resource.
// If it gets a map file, then the code will try to find an environment to open it.
func (a *app) loadResourceV(path string, ws *workspace.Workspace) {
	path, err := filepath.Abs(path)
	if err != nil {
		log.Print("unable to get resource absolute path:", err)
		return
	}

	if filepath.Ext(path) == ".dme" {
		a.loadEnvironment(path)
		return
	}

	if filepath.Ext(path) != ".dmm" {
		log.Print("invalid resource to load:", path)
		return
	}

	environmentPath, err := findEnvironmentFileFromBase(path)

	if a.HasLoadedEnvironment() {
		a.loadMap(path, ws)
		return
	}

	if err == nil {
		a.loadEnvironmentV(environmentPath, func() {
			a.loadMap(path, ws)
		})
	} else {
		log.Print("unable to find environment from file:", path)
		dialog.Open(dialog.TypeInformation{
			Title: "Не найден .dme!",
			Information: "Не найден файл окружения (.dme).\n" +
				"Проверьте доступ к нему или откройте вручную.\n" +
				path,
		})
	}
}

// Goes through all parents starting from the current file location and look for a ".dme" file.
func findEnvironmentFileFromBase(path string) (string, error) {
	for {
		dir := filepath.Dir(path)

		if dir == path {
			return "", fmt.Errorf("unable to find environment")
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			log.Print("unable to read dir while looking for environment:", err)
			return "", fmt.Errorf("unable to read dir: "+dir, err)
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".dme" {
				return filepath.Join(dir, file.Name()), nil
			}
		}

		path = filepath.Dir(path)
	}
}

func (a *app) loadEnvironment(path string) {
	a.loadEnvironmentV(path, nil)
}

func (a *app) loadEnvironmentV(path string, callback func()) {
	a.closeEnvironment(func(closed bool) {
		if closed {
			a.forceLoadEnvironment(path, callback)
		}
	})
}

func (a *app) forceLoadEnvironment(path string, callback func()) {
	log.Printf("opening environment [%s]...", path)

	afterLoad := func(env *dmenv.Dme) {
		a.freeEnvironmentResources()

		a.projectConfig().AddProject(path)
		a.loadedEnvironment = env
		a.pathsFilter = newPathsFilter(env)

		dmicon.Cache.SetRootDirPath(env.RootDir)
		// Accesses are read from the code in the background, they are needed only for the access window.
		dmaccess.SetCurrent(nil)
		go func(root string) {
			defer recoverBackground(root)
			dmaccess.SetCurrent(dmaccess.Load(root))
		}(env.RootDir)
		dmmap.Init(env)

		a.layout.WsArea.AddEmptyWorkspaceIfNone()
		a.UpdateTitle()

		runtime.GC()

		log.Print("environment opened:", path)

		if callback != nil {
			callback()
		}
	}

	go func() {
		dlg := makeLoadingDialog(path)
		dialog.Open(dlg)
		defer dialog.Close(dlg)
		defer recoverBackground(path)

		start := time.Now()
		log.Printf("parsing environment: [%s]...", path)

		env, err := dmenv.New(path)

		if err != nil {
			log.Print("unable to open environment by path:", path, err)

			if sdmmparser.IsParserError(err) {
				dialog.Open(dialog.TypeCustom{
					Title:       "Ошибка парсера!",
					CloseButton: true,
					Layout: w.Layout{
						w.Text("Не удалось открыть окружение: " + path),
						w.Separator(),
						w.Text(err.Error()),
					},
				})
			} else {
				dialog.Open(dialog.TypeInformation{
					Title:       "Ошибка!",
					Information: "Не удалось открыть окружение: " + path,
				})
			}
			return
		}

		log.Printf("environment [%s] parsed in [%d] ms", path, time.Since(start).Milliseconds())
		log.Print("environment parse stats: ", sdmmparser.EnvironmentParseStats)

		window.RunLater(func() {
			afterLoad(env)
		})
	}()
}

func makeLoadingDialog(path string) dialog.Type {
	start := time.Now()
	return dialog.TypeCustom{
		Title: "Загрузка",
		Layout: w.Layout{
			w.Text(path),
			w.Custom(func() {
				passed := fmt.Sprint(time.Since(start).Round(time.Second))

				width := imgui.WindowWidth()
				textW := imgui.CalcTextSize(passed, false, 0).X

				imgui.SetCursorPos(imgui.Vec2{X: (width - textW) * .5, Y: imgui.CursorPosY()})
				imgui.TextColored(style.ColorGold, passed)
			}),
		},
	}
}

// Configure paths filter to access a newly opened environment.
func newPathsFilter(env *dmenv.Dme) *dm.PathsFilter {
	return dm.NewPathsFilter(func(path string) []string {
		return env.Objects[path].DirectChildren
	})
}

func (a *app) loadMap(path string, workspace *workspace.Workspace) {
	log.Printf("opening map [%s]...", path)

	env := a.loadedEnvironment
	if env == nil {
		log.Print("unable to open map without environment:", path)
		return
	}

	// Map parsing and icons decoding are heavy operations, so they are made in the background.
	// Everything that touches the editor state or OpenGL is done in the main thread after that.
	go func() {
		dlg := makeLoadingDialog(path)
		dialog.Open(dlg)
		defer dialog.Close(dlg)
		defer recoverBackground(path)

		start := time.Now()
		log.Printf("parsing map: [%s]...", path)
		data, err := dmmdata.New(path)
		if err != nil {
			log.Printf("unable to open map by path [%s]: %v", path, err)
			dialog.Open(dialog.TypeInformation{
				Title:       "Ошибка: не удалось открыть карту",
				Information: fmt.Sprintf("Ошибка при разборе карты:\n - %s\n - %s", path, err),
			})
			return
		}
		log.Printf("map [%s] parsed in [%d] ms", path, time.Since(start).Milliseconds())

		dmicon.Cache.Preload(dmmap.CollectIcons(env, data))

		done := make(chan struct{})
		window.RunLater(func() {
			defer close(done)
			if a.loadedEnvironment != env {
				log.Print("environment was changed while loading the map, skip:", path)
				return
			}
			a.openParsedMap(path, data, workspace)
		})
		<-done // Keep the loading dialog visible until the map is fully opened.
	}()
}

func (a *app) openParsedMap(path string, data *dmmdata.DmmData, workspace *workspace.Workspace) {
	start := time.Now()

	// Add map to the recent only if it is a part of the currently opened environment.
	if isInsideDir(path, a.loadedEnvironment.RootDir) {
		log.Print("adding map path to the recent:", path)
		cfg := a.projectConfig()
		cfg.AddMap(path)
	} else {
		log.Print("ignoring map path add to the recent, since it's an outside resource")
	}

	backup := a.backupMap(path)
	log.Printf("map backup made in [%d] ms", time.Since(start).Milliseconds())
	dmm, unknownPrefabs := dmmap.New(a.loadedEnvironment, data, backup)
	log.Printf("map data created in [%d] ms", time.Since(start).Milliseconds())
	if a.layout.WsArea.OpenMap(dmm, workspace) {
		log.Printf("map workspace opened in [%d] ms", time.Since(start).Milliseconds())
		a.layout.Prefabs.Sync()

		if len(unknownPrefabs) != 0 {
			// Collect keys
			var prefabPaths []string
			for path := range unknownPrefabs {
				prefabPaths = append(prefabPaths, path)
			}

			// Sort them alphabetically
			sort.Strings(prefabPaths)

			// Build the string
			var prefabsNames string
			for _, path := range prefabPaths {
				prefabsNames += " - " + path + "\n"
			}

			dialog.Open(dialog.TypeInformation{
				Title: "Неизвестные типы",
				Information: fmt.Sprintf(
					"На карте есть неизвестные типы: %s\n"+
						"Типы ниже будут удалены при сохранении:\n"+
						"%s", dmm.Name, prefabsNames,
				),
			})
		}
	}
	a.layout.Search.Free()

	runtime.GC()

	log.Printf("map opened in [%d] ms: %s", time.Since(start).Milliseconds(), path)
}

func (a *app) closeEnvironment(callback func(bool)) {
	// NewMap workspaces depend on the opened environment, so we close them too.
	a.layout.WsArea.CloseAllCreateMaps()
	a.layout.WsArea.CloseAllMaps(func(closed bool) {
		if callback != nil {
			callback(closed)
		}
	})
}

// Frees all resources connected with opened environment.
func (a *app) freeEnvironmentResources() {
	log.Print("free environment resources...")

	a.pathsFilter = dm.NewPathsFilterEmpty()

	a.layout.Prefabs.Free()
	a.layout.Search.Free()
	a.layout.Environment.Free()
	a.layout.WsArea.Free()
	a.layout.VarEditor.Free()

	a.commandStorage.Free()
	a.clipboard.Free()

	tools.Scatter.Palette = nil // Palette prefabs belong to the closed environment.
	dmicon.Cache.Free()
	dmmap.PrefabStorage.Free()
	dmmap.Free()

	a.loadedEnvironment = nil

	a.UpdateTitle()

	log.Print("environment resources free!")
}

func (a *app) environmentName() string {
	if a.loadedEnvironment != nil {
		return a.loadedEnvironment.Name
	}
	return ""
}

func (a *app) backupMap(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		// Do not crash the whole editor. The original file will be used as the save base.
		log.Print("unable to read map to backup:", path, err)
		dialog.Open(dialog.TypeInformation{
			Title:       "Резервная копия",
			Information: "Не удалось прочитать карту для резервной копии:\n" + path,
		})
		return path
	}

	// format: backup/environment.dme/map.dmm/time.dmm
	dst := filepath.FromSlash(a.backupDir + "/" +
		a.environmentName() + "/" +
		filepath.Base(path) + "/" +
		time.Now().Format(util.TimeFormat) + ".dmm",
	)

	_ = os.MkdirAll(filepath.Dir(dst), os.ModePerm)

	err = os.WriteFile(dst, data, os.ModePerm)
	if err != nil {
		// Do not crash the whole editor. The original file will be used as the save base.
		log.Print("unable to write map backup to a file:", dst, err)
		dialog.Open(dialog.TypeInformation{
			Title:       "Резервная копия",
			Information: "Не удалось записать резервную копию карты:\n" + dst,
		})
		return path
	}
	log.Print("map backup created:", dst)

	return dst
}

// recoverBackground prevents the editor from crashing when a background loading fails.
func recoverBackground(path string) {
	if r := recover(); r != nil {
		log.Error().Msgf("background loading failed [%s]: %v\n%s", path, r, debug.Stack())
		dialog.Open(dialog.TypeInformation{
			Title:       "Ошибка загрузки",
			Information: fmt.Sprintf("Не удалось загрузить:\n%s\n\n%v", path, r),
		})
	}
}
