// Package dmmtemplate stores parts of maps (rooms, corridors etc.) as templates to reuse them on any map.
// Templates are plain JSON files, so they could be shared between mappers.
package dmmtemplate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

const fileVersion = 1

// Template is a saved part of the map.
type Template struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Created string `json:"created"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Tiles   []Tile `json:"tiles"`
}

// Tile is a tile of the template. Coordinates are 1-based and relative to the bottom-left tile.
type Tile struct {
	X       int      `json:"x"`
	Y       int      `json:"y"`
	Prefabs []Prefab `json:"prefabs"`
}

// Prefab is a type with variables declared on the map.
type Prefab struct {
	Path string      `json:"path"`
	Vars [][2]string `json:"vars,omitempty"`
}

// Info is a short description of the template file.
type Info struct {
	Name          string
	File          string
	Width, Height int
}

// Options of the template creation.
type Options struct {
	WithAreas bool
	WithTurfs bool
}

// Make creates a template from tiles of the map.
func Make(name string, tiles []dmmap.Tile, opts Options) *Template {
	t := &Template{Version: fileVersion, Name: name, Created: time.Now().Format(time.RFC3339)}
	if len(tiles) == 0 {
		return t
	}

	minX, minY := tiles[0].Coord.X, tiles[0].Coord.Y
	maxX, maxY := minX, minY
	for _, tile := range tiles {
		minX, minY = min(minX, tile.Coord.X), min(minY, tile.Coord.Y)
		maxX, maxY = max(maxX, tile.Coord.X), max(maxY, tile.Coord.Y)
	}
	t.Width, t.Height = maxX-minX+1, maxY-minY+1

	for _, tile := range tiles {
		td := Tile{X: tile.Coord.X - minX + 1, Y: tile.Coord.Y - minY + 1}
		for _, i := range tile.Instances() {
			p := i.Prefab()
			if dm.IsPath(p.Path(), "/area") && !opts.WithAreas {
				continue
			}
			if dm.IsPath(p.Path(), "/turf") && !opts.WithTurfs {
				continue
			}
			pd := Prefab{Path: p.Path()}
			for _, name := range p.Vars().Iterate() {
				value, _ := p.Vars().Value(name)
				pd.Vars = append(pd.Vars, [2]string{name, value})
			}
			td.Prefabs = append(td.Prefabs, pd)
		}
		t.Tiles = append(t.Tiles, td)
	}
	return t
}

// HasAreas returns true if the template stores areas.
func (t *Template) HasAreas() bool {
	return t.has("/area")
}

// HasTurfs returns true if the template stores turfs.
func (t *Template) HasTurfs() bool {
	return t.has("/turf")
}

func (t *Template) has(path string) bool {
	for _, tile := range t.Tiles {
		for _, p := range tile.Prefabs {
			if dm.IsPath(p.Path, path) {
				return true
			}
		}
	}
	return false
}

// ToTiles converts the template to map tiles, which could be pasted with the clipboard.
// Returns paths of types which don't exist in the environment, such objects are skipped.
func (t *Template) ToTiles(env *dmenv.Dme) (tiles []dmmap.Tile, missing []string) {
	missingSet := make(map[string]bool)
	for _, td := range t.Tiles {
		tile := dmmap.Tile{Coord: util.Point{X: td.X, Y: td.Y, Z: 1}}
		var prefabs dmmdata.Prefabs
		for _, pd := range td.Prefabs {
			obj, ok := env.Objects[pd.Path]
			if !ok {
				missingSet[pd.Path] = true
				continue
			}
			vars := dmvars.FromParent(obj.Vars)
			for _, v := range pd.Vars {
				vars = dmvars.Set(vars, v[0], v[1])
			}
			prefabs = append(prefabs, dmmap.PrefabStorage.Get(pd.Path, vars))
		}
		tile.InstancesSet(prefabs)
		tiles = append(tiles, tile)
	}

	// The clipboard uses the first tile as the anchor, so it must be the bottom-left one.
	sort.SliceStable(tiles, func(i, j int) bool {
		if tiles[i].Coord.X != tiles[j].Coord.X {
			return tiles[i].Coord.X < tiles[j].Coord.X
		}
		return tiles[i].Coord.Y < tiles[j].Coord.Y
	})

	for path := range missingSet {
		missing = append(missing, path)
	}
	sort.Strings(missing)
	return tiles, missing
}

var reUnsafeName = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// FileName returns a safe file name for the template name.
func FileName(name string) string {
	name = strings.TrimSpace(reUnsafeName.ReplaceAllString(name, "_"))
	if name == "" {
		name = "template"
	}
	return name + ".json"
}

// Save writes the template to the directory.
func Save(dir string, t *Template) (string, error) {
	if strings.TrimSpace(t.Name) == "" {
		return "", fmt.Errorf("пустое название шаблона")
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(t, "", "\t")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, FileName(t.Name))
	return path, os.WriteFile(path, data, 0o644)
}

// Load reads the template file.
func Load(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("файл шаблона повреждён: %w", err)
	}
	return &t, nil
}

// List returns templates of the directory sorted by the name.
func List(dir string) []Info {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var res []Info
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		t, err := Load(path)
		if err != nil {
			continue
		}
		res = append(res, Info{Name: t.Name, File: path, Width: t.Width, Height: t.Height})
	}
	sort.Slice(res, func(i, j int) bool { return strings.ToLower(res[i].Name) < strings.ToLower(res[j].Name) })
	return res
}
