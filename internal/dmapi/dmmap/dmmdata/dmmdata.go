package dmmdata

import (
	"fmt"
	"os"
	"sort"

	"sdmm/internal/util"
)

type (
	DataDictionary map[Key]Prefabs
	DataGrid       map[util.Point]Key
)

// DmmData stores raw information about the map. Mostly needed to for parsing and saving.
type DmmData struct {
	Filepath string

	IsTgm     bool
	LineBreak string

	KeyLength        int
	MaxX, MaxY, MaxZ int

	Dictionary DataDictionary
	Grid       DataGrid
}

// Save writes the map to its file. The data is written to a temporary file at first and then the file is replaced,
// so the map won't be corrupted if the editor crashes or the power goes off during the save.
func (d DmmData) Save() error {
	tmp := d.Filepath + ".tmp"

	var err error
	if d.IsTgm {
		err = d.SaveTGM(tmp)
	} else {
		err = d.SaveDM(tmp)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err = os.Rename(tmp, d.Filepath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (d DmmData) Keys() []Key {
	keys := make([]Key, 0, len(d.Dictionary))
	for key := range d.Dictionary {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].ToNum() < keys[j].ToNum()
	})

	return keys
}

func (d DmmData) String() string {
	var winLineBreak bool
	if d.LineBreak == "\r\n" {
		winLineBreak = true
	}
	return fmt.Sprintf(
		"Filepath: %s, IsTgm: %t, WinLineBreak: %v, KeyLength: %d, MaxX: %d, MaxY: %d, MaxZ: %d",
		d.Filepath, d.IsTgm, winLineBreak, d.KeyLength, d.MaxX, d.MaxY, d.MaxZ)
}

func New(path string) (*DmmData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := parse(file)
	if err != nil {
		return nil, err
	}
	if err := data.validate(); err != nil {
		return nil, err
	}
	return data, nil
}

// validate checks that the map is complete. A truncated file (e.g. after a crash during the save)
// is parsed without errors, but opening and saving it would destroy the map.
func (d *DmmData) validate() error {
	if d.MaxX <= 0 || d.MaxY <= 0 || d.MaxZ <= 0 {
		return fmt.Errorf("карта пустая или повреждена (размер %dx%dx%d)", d.MaxX, d.MaxY, d.MaxZ)
	}
	if expected := d.MaxX * d.MaxY * d.MaxZ; len(d.Grid) < expected {
		return fmt.Errorf("в карте не хватает тайлов: %d из %d, файл обрезан?", len(d.Grid), expected)
	}
	for coord, key := range d.Grid {
		if _, ok := d.Dictionary[key]; !ok {
			return fmt.Errorf("ключ \"%s\" на %v не описан в карте, файл повреждён", key, coord)
		}
	}
	return nil
}
