package app

import (
	"os"
	"path/filepath"
)

// Check program arguments to load dme/dmm files passed by.
func (a *app) checkProgramArgs() {
	// The first argument is always a path to the executable.
	if len(os.Args) < 2 {
		return
	}

	var envPath string
	var mapPaths []string

	for _, arg := range os.Args {
		switch filepath.Ext(arg) {
		case ".dme":
			envPath = arg
		case ".dmm":
			mapPaths = append(mapPaths, arg)
		}
	}

	if len(envPath) > 0 {
		if abs, err := filepath.Abs(envPath); err == nil {
			envPath = abs
		}
		// Maps are opened only after the environment is loaded, otherwise the environment would be loaded twice.
		a.loadEnvironmentV(envPath, func() {
			for _, mapPath := range mapPaths {
				a.loadResource(mapPath)
			}
		})
		return
	}

	for _, mapPath := range mapPaths {
		a.loadResource(mapPath)
	}
}
