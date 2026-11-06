package tools

import (
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

func transformTestEditor() *fakeEditor {
	return newFakeEditor(10, 10, map[string]map[string]string{
		"/obj/structure/cable": {"d1": "0", "d2": "1", "icon_state": `"0-1"`},
		"/obj/machinery/atmospherics/pipe/simple/hidden/supply": nil,
		"/obj/machinery/firealarm/directional/north":            {"dir": "1", "pixel_y": "26"},
		"/obj/machinery/firealarm/directional/east":             {"dir": "4", "pixel_x": "26"},
		"/obj/structure/disposalpipe/junction":                  nil,
		"/obj/structure/disposalpipe/junction/reversed":         nil,
		"/obj/item/thing": nil,
	})
}

func selectArea(t *ToolGrab, x1, y1, x2, y2 int) {
	t.Reset()
	t.fillStart = pt(x1, y1)
	t.selectArea(float64(x1), float64(y1), float64(x1), float64(y1), pt(x2, y2))
	t.stopSelectArea()
}

func TestRotateSelection(t *testing.T) {
	f := transformTestEditor()
	grab := newGrab()

	place := func(x, y int, path string, vars map[string]string) {
		f.dmm.GetTile(pt(x, y)).InstancesAdd(f.prefab(path, vars))
	}
	place(2, 2, "/obj/machinery/atmospherics/pipe/simple/hidden/supply", nil)     // Vertical pipe (default dir).
	place(3, 2, "/obj/structure/cable", map[string]string{"icon_state": `"1-4"`}) // North-East cable.
	place(4, 3, "/obj/machinery/firealarm/directional/north", nil)                // Wall mounted.
	place(2, 3, "/obj/item/thing", map[string]string{"pixel_x": "10"})            // Shifted item.

	// Area 3x2: x 2..4, y 2..3. Rotated clockwise it becomes 2x3.
	selectArea(grab, 2, 2, 4, 3)
	if err := grab.Transform(OpRotateCW); err != nil {
		t.Fatal(err)
	}

	if b := grab.Bounds(); b != (util.Bounds{X1: 2, Y1: 2, X2: 3, Y2: 4}) {
		t.Fatalf("new bounds: %v", b)
	}

	// Local (0,0) -> (0, w-1) = (2,4): the pipe becomes horizontal (west dir).
	if p := f.objectsAt(2, 4, "/obj/machinery/atmospherics/pipe"); len(p) != 1 || p[0].Vars().IntV("dir", 2) != 8 {
		t.Fatalf("pipe: %v", p)
	}
	// Local (1,0) -> (0,1) = (2,3): cable 1-4 (N,E) becomes 2-4 (S,E).
	if c := f.objectsAt(2, 3, "/obj/structure/cable"); len(c) != 1 || c[0].Vars().TextV("icon_state", "") != "2-4" {
		t.Fatalf("cable: %v", c)
	}
	// Local (2,1) -> (1,0) = (3,2): fire alarm on the north wall goes to the east wall.
	if a := f.objectsAt(3, 2, "/obj/machinery/firealarm"); len(a) != 1 || a[0].Path() != "/obj/machinery/firealarm/directional/east" || a[0].Vars().Len() != 0 {
		t.Fatalf("alarm: %v", a)
	}
	// Local (0,1) -> (1,2) = (3,4): pixel offset is rotated.
	if i := f.objectsAt(3, 4, "/obj/item/thing"); len(i) != 1 || i[0].Vars().IntV("pixel_y", 0) != -10 || isOwn(i[0].Vars(), "pixel_x") {
		t.Fatalf("item: %v", i)
	}
	// Floors must not get dir variables.
	for _, tile := range f.dmm.Tiles {
		for _, i := range tile.Instances() {
			if i.Prefab().Path() == "/turf/floor" && isOwn(i.Prefab().Vars(), "dir") {
				t.Fatalf("floor got a dir at %v", tile.Coord)
			}
		}
	}
	if len(f.commits) != 1 {
		t.Fatalf("commits: %v", f.commits)
	}
}

func TestRotateFourTimesIsIdentity(t *testing.T) {
	f := transformTestEditor()
	grab := newGrab()

	f.dmm.GetTile(pt(5, 5)).InstancesAdd(f.prefab("/obj/structure/cable", map[string]string{"icon_state": `"2-8"`}))
	f.dmm.GetTile(pt(6, 5)).InstancesAdd(f.prefab("/obj/machinery/atmospherics/pipe/simple/hidden/supply", map[string]string{"dir": "5"}))
	before := snapshot(f.dmm)

	selectArea(grab, 4, 4, 7, 7) // Square area: rotation keeps the bounds.
	for i := 0; i < 4; i++ {
		if err := grab.Transform(OpRotateCW); err != nil {
			t.Fatal(err)
		}
	}
	if after := snapshot(f.dmm); after != before {
		t.Fatalf("4 rotations must give the same map\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestFlipJunction(t *testing.T) {
	f := transformTestEditor()
	grab := newGrab()
	f.dmm.GetTile(pt(5, 5)).InstancesAdd(f.prefab("/obj/structure/disposalpipe/junction", map[string]string{"dir": "1"}))

	selectArea(grab, 5, 5, 5, 5)
	if err := grab.Transform(OpFlipX); err != nil {
		t.Fatal(err)
	}
	j := f.objectsAt(5, 5, "/obj/structure/disposalpipe")
	if len(j) != 1 || j[0].Path() != "/obj/structure/disposalpipe/junction/reversed" {
		t.Fatalf("junction: %v", j)
	}
}

func TestRotateOutOfMap(t *testing.T) {
	f := transformTestEditor()
	grab := newGrab()
	selectArea(grab, 6, 9, 10, 10) // 5x2 -> 2x5 doesn't fit.
	if err := grab.Transform(OpRotateCW); err == nil {
		t.Fatal("must be refused")
	}
	if len(f.commits) != 0 {
		t.Fatal("nothing must be committed")
	}
}

func snapshot(dmm *dmmap.Dmm) string {
	res := ""
	for _, tile := range dmm.Tiles {
		for _, i := range tile.Instances() {
			if i.Prefab().Path() == "/turf/floor" || i.Prefab().Path() == "/area/space" {
				continue
			}
			res += tile.Coord.String() + i.Prefab().Path() + i.Prefab().Vars().ValueV("icon_state", "") + i.Prefab().Vars().ValueV("dir", "") + ";"
		}
	}
	return res
}

func TestFlipBayCorner(t *testing.T) {
	f := newFakeEditor(10, 10, map[string]map[string]string{
		"/obj/structure/disposalpipe/segment": {"icon_state": `"pipe-s"`},
		"/obj/structure/cable":                {"d1": "0", "d2": "1", "icon_state": `"0-1"`},
	})
	grab := newGrab()
	// Corner north+east: dir north, "pipe-c".
	f.dmm.GetTile(pt(5, 5)).InstancesAdd(f.prefab("/obj/structure/disposalpipe/segment", map[string]string{"dir": "1", "icon_state": `"pipe-c"`}))
	f.dmm.GetTile(pt(6, 5)).InstancesAdd(f.prefab("/obj/structure/cable", map[string]string{"d1": "1", "d2": "4", "icon_state": `"1-4"`}))

	selectArea(grab, 5, 5, 6, 5)
	if err := grab.Transform(OpFlipX); err != nil {
		t.Fatal(err)
	}
	// After the horizontal flip the corner connects north and west: dir west, clockwise(west) = north.
	c := f.objectsAt(6, 5, "/obj/structure/disposalpipe")
	if len(c) != 1 || c[0].Vars().IntV("dir", 2) != 8 || c[0].Vars().TextV("icon_state", "") != "pipe-c" {
		t.Fatalf("corner: %v", c)
	}
	cable := f.objectsAt(5, 5, "/obj/structure/cable")
	if len(cable) != 1 || cable[0].Vars().TextV("icon_state", "") != "1-8" || cable[0].Vars().IntV("d1", 0) != 1 || cable[0].Vars().IntV("d2", 0) != 8 {
		t.Fatalf("cable: %v", cable)
	}
}
