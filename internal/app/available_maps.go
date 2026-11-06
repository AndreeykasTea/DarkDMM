package app

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Directories which never contain maps, but could be huge (node_modules has hundreds of thousands of files).
var availableMapsSkipDirs = map[string]bool{
	"node_modules": true,
	"cache":        true,
	"target":       true,
}

var availableMapsCache struct {
	mtx      sync.Mutex
	root     string
	maps     []string
	scanning bool
}

// AvailableMaps returns all maps inside the environment directory.
// The directory is scanned in the background once per environment, so the UI won't freeze.
// Returns nil while the scan is in progress.
func (a *app) AvailableMaps() []string {
	if !a.HasLoadedEnvironment() {
		return nil
	}

	root := a.LoadedEnvironment().RootDir

	c := &availableMapsCache
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.root == root && c.maps != nil {
		res := make([]string, len(c.maps))
		copy(res, c.maps)
		return res
	}

	if c.root != root {
		c.root = root
		c.maps = nil
		c.scanning = false
	}

	if !c.scanning {
		c.scanning = true
		go scanAvailableMaps(root)
	}

	return nil
}

func scanAvailableMaps(root string) {
	start := time.Now()
	maps := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable entries.
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || availableMapsSkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".dmm" {
			maps = append(maps, path)
		}
		return nil
	})
	if err != nil {
		log.Print("unable to find available maps:", err)
	}

	log.Printf("found [%d] available maps in [%d] ms", len(maps), time.Since(start).Milliseconds())

	c := &availableMapsCache
	c.mtx.Lock()
	if c.root == root {
		c.maps = maps
		c.scanning = false
	}
	c.mtx.Unlock()
}

// isInsideDir returns true if the path is inside the directory.
func isInsideDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
