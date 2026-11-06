package tools

import (
	"testing"

	"sdmm/internal/dmapi/dm"
)

func bayDisposalEditor() *fakeEditor {
	return newFakeEditor(12, 12, map[string]map[string]string{
		"/obj/structure/disposalpipe/segment":                       {"dir": "0", "icon_state": `"pipe-s"`},
		"/obj/structure/disposalpipe/segment/bent":                  {"dir": "0", "icon_state": `"pipe-c"`},
		"/obj/structure/disposalpipe/trunk":                         {"dir": "0", "icon_state": `"pipe-t"`},
		"/obj/structure/disposalpipe/junction":                      {"dir": "0", "icon_state": `"pipe-j1"`},
		"/obj/structure/disposalpipe/junction/mirrored":             {"dir": "0", "icon_state": `"pipe-j2"`},
		"/obj/structure/disposalpipe/sortjunction":                  {"dir": "0", "icon_state": `"pipe-j1s"`},
		"/obj/structure/disposalpipe/sortjunction/wildcard":         {"dir": "0", "icon_state": `"pipe-j1s"`},
		"/obj/structure/disposalpipe/sortjunction/wildcard/flipped": {"dir": "0", "icon_state": `"pipe-j2s"`},
	})
}

// site53 has hundreds of pipes without a dir (dir = 0 in the code). They must really turn,
// and straight ones must stay south or east, otherwise the disposal unit test fails.
func TestRotateStraightDisposalWithoutDir(t *testing.T) {
	f := bayDisposalEditor()
	grab := newGrab()
	f.dmm.GetTile(pt(5, 5)).InstancesAdd(f.prefab("/obj/structure/disposalpipe/segment", nil))
	f.dmm.GetTile(pt(7, 5)).InstancesAdd(f.prefab("/obj/structure/disposalpipe/trunk", nil))

	selectArea(grab, 4, 4, 8, 8)
	wantDirs := []int{dm.DirEast, dm.DirSouth, dm.DirEast, dm.DirSouth}
	wantTrunks := []int{dm.DirWest, dm.DirNorth, dm.DirEast, dm.DirSouth}
	for step := range wantDirs {
		if err := grab.Transform(OpRotateCW); err != nil {
			t.Fatal(err)
		}
		pipes := 0
		for _, tile := range f.dmm.Tiles {
			for _, i := range tile.Instances() {
				p := i.Prefab()
				dir := dm.FixDir(p.Vars().IntV("dir", dm.DirSouth))
				switch p.Path() {
				case "/obj/structure/disposalpipe/segment":
					pipes++
					if dir != wantDirs[step] {
						t.Fatalf("step %d: straight pipe dir %d, want %d", step, dir, wantDirs[step])
					}
				case "/obj/structure/disposalpipe/trunk":
					pipes++
					if dir != wantTrunks[step] {
						t.Fatalf("step %d: trunk dir %d, want %d", step, dir, wantTrunks[step])
					}
				}
			}
		}
		if pipes != 2 {
			t.Fatalf("step %d: pipes %d", step, pipes)
		}
	}
	// After 4 rotations both of them have no dir variable again.
	for _, tile := range f.dmm.Tiles {
		for _, i := range tile.Instances() {
			if dm.IsPath(i.Prefab().Path(), "/obj/structure/disposalpipe") && isOwn(i.Prefab().Vars(), "dir") {
				t.Fatalf("useless dir on %s", i.Prefab().Path())
			}
		}
	}
}

func TestFlipBayJunctions(t *testing.T) {
	cases := []struct {
		name         string
		path         string
		vars         map[string]string
		wantPath     string
		wantState    string
		wantOwnState bool
	}{
		{"by the type", "/obj/structure/disposalpipe/junction", nil, "/obj/structure/disposalpipe/junction/mirrored", "pipe-j2", false},
		{"back by the type", "/obj/structure/disposalpipe/junction/mirrored", nil, "/obj/structure/disposalpipe/junction", "pipe-j1", false},
		{"mirrored by the state", "/obj/structure/disposalpipe/junction", map[string]string{"icon_state": `"pipe-j2"`}, "/obj/structure/disposalpipe/junction", "pipe-j1", false},
		{"y junction", "/obj/structure/disposalpipe/junction", map[string]string{"icon_state": `"pipe-y"`}, "/obj/structure/disposalpipe/junction", "pipe-y", true},
		{"sort junction", "/obj/structure/disposalpipe/sortjunction", nil, "/obj/structure/disposalpipe/sortjunction", "pipe-j2s", true},
		{"wildcard", "/obj/structure/disposalpipe/sortjunction/wildcard", nil, "/obj/structure/disposalpipe/sortjunction/wildcard/flipped", "pipe-j2s", false},
	}
	for _, c := range cases {
		f := bayDisposalEditor()
		grab := newGrab()
		f.dmm.GetTile(pt(5, 5)).InstancesAdd(f.prefab(c.path, c.vars))
		selectArea(grab, 5, 5, 5, 5)
		if err := grab.Transform(OpFlipX); err != nil {
			t.Fatal(err)
		}
		found := f.objectsAt(5, 5, "/obj/structure/disposalpipe")
		if len(found) != 1 {
			t.Fatalf("%s: %v", c.name, found)
		}
		p := found[0]
		if p.Path() != c.wantPath || p.Vars().TextV("icon_state", "") != c.wantState || isOwn(p.Vars(), "icon_state") != c.wantOwnState {
			t.Errorf("%s: %s state %s own %t", c.name, p.Path(), p.Vars().TextV("icon_state", ""), isOwn(p.Vars(), "icon_state"))
		}
	}
}
