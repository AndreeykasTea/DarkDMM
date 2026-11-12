package tools

import "testing"

// The replace mode of adding removes objects of the tile, but not those which the mapper can't see.
func TestAltAddKeepsObjectsHiddenUnderFloor(t *testing.T) {
	f := newFakeEditor(5, 5, map[string]map[string]string{
		"/obj/structure/cable": nil,
		"/obj/item/one":        nil,
		"/obj/item/two":        nil,
		"/obj/item/three":      nil,
		"/obj/item/new":        nil,
	})
	f.hiddenPath = "/obj/structure/cable"

	tile := f.dmm.GetTile(pt(2, 2))
	tile.InstancesAdd(f.prefab("/obj/item/one", nil))
	tile.InstancesAdd(f.prefab("/obj/item/two", nil))
	tile.InstancesAdd(f.prefab("/obj/structure/cable", nil))
	tile.InstancesAdd(f.prefab("/obj/item/three", nil))

	add := &tool{altBehaviour: true}
	add.basicPrefabAdd(tile, f.prefab("/obj/item/new", nil))

	for _, path := range []string{"/obj/item/one", "/obj/item/two", "/obj/item/three"} {
		if got := f.objectsAt(2, 2, path); len(got) != 0 {
			t.Errorf("%s must be replaced", path)
		}
	}
	if got := f.objectsAt(2, 2, "/obj/structure/cable"); len(got) != 1 {
		t.Errorf("the hidden cable must stay: %d", len(got))
	}
	if got := f.objectsAt(2, 2, "/obj/item/new"); len(got) != 1 {
		t.Errorf("the new object must be added: %d", len(got))
	}

	// The same cable is replaced when the mapper sees it.
	f.hiddenPath = ""
	add.basicPrefabAdd(tile, f.prefab("/obj/item/one", nil))
	if got := f.objectsAt(2, 2, "/obj/structure/cable"); len(got) != 0 {
		t.Errorf("a visible cable must be replaced: %d", len(got))
	}
}
