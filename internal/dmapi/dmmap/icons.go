package dmmap

import (
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap/dmmdata"
)

// CollectIcons returns a list of unique icons used by the map data.
// Doesn't modify any data, so it's safe to call it in the background goroutine.
func CollectIcons(dme *dmenv.Dme, data *dmmdata.DmmData) []string {
	unique := make(map[string]bool)
	for _, prefabs := range data.Dictionary {
		for _, prefab := range prefabs {
			if icon, ok := prefab.Vars().Text("icon"); ok {
				unique[icon] = true
				continue
			}
			if obj, ok := dme.Objects[prefab.Path()]; ok {
				if icon, ok := obj.Vars.Text("icon"); ok {
					unique[icon] = true
				}
			}
		}
	}

	icons := make([]string, 0, len(unique))
	for icon := range unique {
		icons = append(icons, icon)
	}
	return icons
}
