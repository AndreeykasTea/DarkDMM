package tools

import (
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmnet"
)

func mapDirBits(dirs int, f func(int) int) int {
	res := 0
	for _, d := range dm.Cardinals {
		if dirs&d != 0 {
			res |= f(d)
		}
	}
	return res
}

// Checks every real disposal pipe and cable of Baystation maps, if TRANSFORM_BAY_DME and TRANSFORM_BAY_DMM are set:
// connections must turn with the operation, 4 rotations and 2 flips must give the same connections.
func TestTransformRealPrefabs(t *testing.T) {
	dmePath, maps := os.Getenv("TRANSFORM_BAY_DME"), os.Getenv("TRANSFORM_BAY_DMM")
	if dmePath == "" {
		t.Skip()
	}
	env, err := dmenv.New(dmePath)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	dmmap.Init(env)

	oldDirs := iconDirsOf
	iconDirsOf = func(*dmmprefab.Prefab) int { return 4 }
	defer func() { iconDirsOf = oldDirs }()

	connections := func(p *dmmprefab.Prefab) int {
		d, _ := dmnet.DisposalDirs(p.Path(), p.Vars().IntV("dir", dm.DirSouth), p.Vars().TextV("icon_state", ""))
		return d
	}

	checked := 0
	for _, path := range strings.Split(maps, ";") {
		data, err := dmmdata.New(path)
		if err != nil {
			t.Fatal(err)
		}
		dmm, _ := dmmap.New(env, data, "")

		seen := map[uint64]bool{}
		for _, tile := range dmm.Tiles {
			for _, inst := range tile.Instances() {
				p := inst.Prefab()
				if seen[p.Id()] || !dm.IsPath(p.Path(), "/obj/structure/disposalpipe") || dm.IsPath(p.Path(), "/obj/structure/disposalpipe/broken") {
					continue
				}
				seen[p.Id()] = true
				checked++
				before := connections(p)
				if before == 0 {
					t.Errorf("%s dir %d: no connections", p.Path(), p.Vars().IntV("dir", 2))
					continue
				}

				cur := p
				want := before
				for i := 0; i < 4; i++ {
					cur = TransformPrefab(env, cur, OpRotateCW)
					want = mapDirBits(want, dm.DirTurnCW)
					if got := connections(cur); got != want {
						t.Errorf("%s dir %d icon %s, rotation %d: connections %d, want %d", p.Path(), p.Vars().IntV("dir", 2), p.Vars().TextV("icon_state", ""), i+1, got, want)
					}
					if _, bay := env.Objects["/obj/structure/disposalpipe/segment/bent"]; bay && dm.IsPath(cur.Path(), "/obj/structure/disposalpipe/segment") && cur.Vars().TextV("icon_state", "") == "pipe-s" {
						if d := dm.FixDir(cur.Vars().IntV("dir", 2)); d != dm.DirSouth && d != dm.DirEast {
							t.Errorf("%s: straight pipe got dir %d", p.Path(), d)
						}
					}
				}
				if cur.Id() != p.Id() && connections(cur) != before {
					t.Errorf("%s: 4 rotations changed the pipe", p.Path())
				}

				for _, op := range []struct {
					op TransformOp
					f  func(int) int
				}{{OpFlipX, dm.DirFlipX}, {OpFlipY, dm.DirFlipY}} {
					flipped := TransformPrefab(env, p, op.op)
					if got, want := connections(flipped), mapDirBits(before, op.f); got != want {
						t.Errorf("%s dir %d icon %s, flip: connections %d, want %d (%s icon %s dir %d)", p.Path(), p.Vars().IntV("dir", 2), p.Vars().TextV("icon_state", ""), got, want, flipped.Path(), flipped.Vars().TextV("icon_state", ""), flipped.Vars().IntV("dir", 2))
					}
					if got := connections(TransformPrefab(env, flipped, op.op)); got != before {
						t.Errorf("%s: 2 flips changed connections: %d, want %d", p.Path(), got, before)
					}
				}
			}
		}
	}
	t.Logf("checked %d kinds of pipes", checked)
	if checked == 0 {
		t.Fatal("nothing to check")
	}
}
