package tools

import (
	"sdmm/internal/app/prefs"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// fakeEditor is a minimal editor to test tools without the UI.
type fakeEditor struct {
	// hiddenPath is a path of objects, which the mapper does not see under a floor.
	hiddenPath string

	dmm      *dmmap.Dmm
	env      *dmenv.Dme
	selected *dmmprefab.Prefab
	commits  []string
}

func (f *fakeEditor) Dmm() *dmmap.Dmm                                     { return f.dmm }
func (f *fakeEditor) Environment() *dmenv.Dme                             { return f.env }
func (f *fakeEditor) CommitChanges(msg string)                            { f.commits = append(f.commits, msg) }
func (f *fakeEditor) UpdateCanvasByCoords([]util.Point)                   {}
func (f *fakeEditor) UpdateCanvasByTiles([]dmmap.Tile)                    {}
func (f *fakeEditor) OverlayPushTile(util.Point, util.Color, util.Color)  {}
func (f *fakeEditor) OverlayPushArea(util.Bounds, util.Color, util.Color) {}
func (f *fakeEditor) InstanceSelect(*dmminstance.Instance)                {}
func (f *fakeEditor) InstanceDelete(*dmminstance.Instance)                {}
func (f *fakeEditor) TileDeleteSelected()                                 {}
func (f *fakeEditor) IsHiddenUnderFloor(_ *dmmap.Tile, p *dmmprefab.Prefab) bool {
	return f.hiddenPath != "" && p.Path() == f.hiddenPath
}
func (f *fakeEditor) HoveredInstance() *dmminstance.Instance    { return nil }
func (f *fakeEditor) ZoomLevel() float32                        { return 1 }
func (f *fakeEditor) Prefs() prefs.Prefs                        { return prefs.Prefs{} }
func (f *fakeEditor) SelectedPrefab() (*dmmprefab.Prefab, bool) { return f.selected, f.selected != nil }
func (f *fakeEditor) TileReplace(coord util.Point, prefabs dmmdata.Prefabs) {
	f.dmm.GetTile(coord).InstancesSet(prefabs)
}
func (f *fakeEditor) TileDelete(coord util.Point) {
	tile := f.dmm.GetTile(coord)
	tile.InstancesSet(nil)
	tile.InstancesRegenerate()
}

// newFakeEditor creates a map w*h with a floor and an area on every tile and registers the editor for tools.
func newFakeEditor(w, h int, objects map[string]map[string]string) *fakeEditor {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, vars map[string]string) {
		mv := &dmvars.MutableVariables{}
		mv.Put("dir", "2")
		for k, v := range vars {
			mv.Put(k, v)
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/area/space", nil)
	add("/turf/floor", nil)
	for path, vars := range objects {
		add(path, vars)
	}

	dmmap.PrefabStorage.Free()
	dmmap.BaseArea = dmmap.PrefabStorage.Get("/area/space", dmvars.FromParent(env.Objects["/area/space"].Vars))
	dmmap.BaseTurf = dmmap.PrefabStorage.Get("/turf/floor", dmvars.FromParent(env.Objects["/turf/floor"].Vars))

	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(w, h, 1)

	f := &fakeEditor{dmm: dmm, env: env}
	SetEditor(f)
	return f
}

func (f *fakeEditor) prefab(path string, vars map[string]string) *dmmprefab.Prefab {
	v := dmvars.FromParent(f.env.Objects[path].Vars)
	for name, value := range vars {
		v = dmvars.Set(v, name, value)
	}
	return dmmap.PrefabStorage.Get(path, v)
}

// objectsAt returns prefabs of the tile which are under the provided path.
func (f *fakeEditor) objectsAt(x, y int, path string) []*dmmprefab.Prefab {
	var res []*dmmprefab.Prefab
	for _, i := range f.dmm.GetTile(util.Point{X: x, Y: y, Z: 1}).Instances() {
		if len(i.Prefab().Path()) >= len(path) && i.Prefab().Path()[:len(path)] == path {
			res = append(res, i.Prefab())
		}
	}
	return res
}
