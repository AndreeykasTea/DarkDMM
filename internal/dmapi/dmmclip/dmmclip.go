package dmmclip

import (
	"sort"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

type PasteData struct {
	Filter dm.PathsFilter
	Buffer []dmmap.Tile
	// UnderFloorHidden has source coords of tiles, where objects under the floor were not visible on the copy.
	// The mapper hasn't copied them, so objects under the floor of the target tile must stay on the paste.
	UnderFloorHidden map[util.Point]bool
}

// UnderFloor tells which objects are not visible for the mapper under floors.
type UnderFloor interface {
	// IsTileUnderFloorHidden returns true if objects under the floor of the tile are not visible.
	IsTileUnderFloorHidden(tile *dmmap.Tile) bool
	// IsHiddenUnderFloor returns true if the object on the tile is not visible because it's under the floor.
	IsHiddenUnderFloor(tile *dmmap.Tile, prefab *dmmprefab.Prefab) bool
}

// Clipboard is a global storage for tiles to provide a copy/paste experience.
type Clipboard struct {
	pasteData PasteData
}

func New() *Clipboard {
	return &Clipboard{}
}

func (c *Clipboard) Free() {
	c.pasteData = PasteData{}
	log.Print("clipboard free")
}

// Copy stores tiles in the buffer. Objects which are not visible under floors are not copied.
func (c *Clipboard) Copy(pathsFilter *dm.PathsFilter, dmm *dmmap.Dmm, tiles []util.Point, underFloor UnderFloor) {
	if len(tiles) == 0 {
		return
	}

	log.Printf("copy tiles to the clipboard buffer: %v", tiles)

	c.pasteData.Filter = pathsFilter.Copy()
	c.pasteData.Buffer = make([]dmmap.Tile, 0, len(tiles))
	c.pasteData.UnderFloorHidden = make(map[util.Point]bool)

	for _, pos := range tiles {
		if !dmm.HasTile(pos) {
			continue
		}

		original := dmm.GetTile(pos)
		tile := original.Copy()
		if underFloor.IsTileUnderFloorHidden(original) {
			c.pasteData.UnderFloorHidden[pos] = true
		}

		var prefabs dmmdata.Prefabs
		for _, instance := range tile.Instances() {
			if pathsFilter.IsVisiblePath(instance.Prefab().Path()) && !underFloor.IsHiddenUnderFloor(original, instance.Prefab()) {
				prefabs = append(prefabs, instance.Prefab())
			}
		}

		tile.InstancesSet(prefabs)

		c.pasteData.Buffer = append(c.pasteData.Buffer, tile)
	}

	sort.SliceStable(c.pasteData.Buffer, func(i, j int) bool {
		return c.pasteData.Buffer[i].Coord.Y < c.pasteData.Buffer[j].Coord.Y
	})
	sort.SliceStable(c.pasteData.Buffer, func(i, j int) bool {
		return c.pasteData.Buffer[i].Coord.X < c.pasteData.Buffer[j].Coord.X
	})
}

func (c *Clipboard) Buffer() PasteData {
	return c.pasteData
}

func (c *Clipboard) HasData() bool {
	return len(c.pasteData.Buffer) != 0
}

// SetData puts provided tiles to the clipboard (used to paste templates).
func (c *Clipboard) SetData(data PasteData) {
	c.pasteData = data
}
