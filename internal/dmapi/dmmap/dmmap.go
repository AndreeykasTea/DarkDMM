package dmmap

import (
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"

	"github.com/rs/zerolog/log"
)

var (
	WorldIconSize int

	/*
		Tiles should have at least one area and one turf.
		Those vars will store them to ensure that the tile has a proper content.
	*/
	BaseArea *dmmprefab.Prefab
	BaseTurf *dmmprefab.Prefab

	environment *dmenv.Dme
)

func Init(dme *dmenv.Dme) {
	environment = dme

	WorldIconSize = dme.Objects["/world"].Vars.IntV("icon_size", 32)

	baseAreaPath := basePath(dme, "area", "/area")
	baseTurfPath := basePath(dme, "turf", "/turf")
	BaseArea = PrefabStorage.Initial(baseAreaPath)
	BaseTurf = PrefabStorage.Initial(baseTurfPath)

	log.Print("initialized with:", dme.RootFile)
	log.Print("base area:", baseAreaPath)
	log.Print("base turf:", baseTurfPath)
}

// basePath returns a type from the world variable. The type could be missing in the environment,
// if the code base defines it in a map file, which is not included into the .dme by default.
func basePath(dme *dmenv.Dme, varName, fallback string) string {
	path, _ := dme.Objects["/world"].Vars.Value(varName)
	if _, ok := dme.Objects[path]; ok {
		return path
	}
	log.Printf("world.%s [%s] is not found in the environment, using [%s]", varName, path, fallback)
	return fallback
}

func Free() {
	environment = nil
	WorldIconSize = 0
	BaseArea = nil
	BaseTurf = nil
}

// TypeVars returns variables of the type from the loaded environment.
func TypeVars(path string) (*dmvars.Variables, bool) {
	if environment == nil {
		return nil, false
	}
	obj, ok := environment.Objects[path]
	if !ok {
		return nil, false
	}
	return obj.Vars, true
}
