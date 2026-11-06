package dmmsave

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func changedLines(a, b []byte) (int, int) {
	al := strings.Split(string(a), "\n")
	bl := strings.Split(string(b), "\n")
	set := func(l []string) map[string]int {
		m := make(map[string]int, len(l))
		for _, s := range l {
			m[s]++
		}
		return m
	}
	am, bm := set(al), set(bl)
	removed, added := 0, 0
	for s, n := range am {
		if d := n - bm[s]; d > 0 {
			removed += d
		}
	}
	for s, n := range bm {
		if d := n - am[s]; d > 0 {
			added += d
		}
	}
	return removed, added
}

// Edits and saves real maps through the save process of the editor, if E2E_ROOT is set.
// Only copies of maps are touched.
func TestE2E(t *testing.T) {
	root := os.Getenv("E2E_ROOT")
	if root == "" {
		t.Skip()
	}
	env, err := dmenv.New(filepath.Join(root, "baystation12.dme"))
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"maps/site53/site53.dmm", "maps/site47/site47.dmm"} {
		orig, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		load := func(path string) *dmmap.Dmm {
			dmmap.PrefabStorage.Free()
			dmmap.Init(env)
			data, err := dmmdata.New(path)
			if err != nil {
				t.Fatal(err)
			}
			backup := path + ".bak"
			origData, _ := os.ReadFile(path)
			_ = os.WriteFile(backup, origData, 0o644)
			dmm, _ := dmmap.New(env, data, backup)
			return dmm
		}

		for _, cfg := range []struct {
			name   string
			format Format
		}{{"Initial", FormatInitial}, {"TGM", FormatTGM}} {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.Base(rel))
			if err := os.WriteFile(path, orig, 0o644); err != nil {
				t.Fatal(err)
			}

			// Without changes.
			dmm := load(path)
			if err := SaveV(env, dmm, path, Config{Format: cfg.format}); err != nil {
				t.Fatal(err)
			}
			out, _ := os.ReadFile(path)
			r, a := changedLines(orig, out)
			t.Logf("%s, %-7s no edits: removed %d, added %d, identical %t, crlf %d->%d", rel, cfg.name, r, a, bytes.Equal(orig, out), bytes.Count(orig, []byte("\r\n")), bytes.Count(out, []byte("\r\n")))

			if cfg.format != FormatInitial {
				continue
			}

			// Edit: a disposal branch from a real straight pipe without a dir and a cable piece.
			var pipeTile *dmmap.Tile
			var pipe *dmmprefab.Prefab
			for _, tile := range dmm.Tiles {
				for _, i := range tile.Instances() {
					p := i.Prefab()
					if p.Path() != "/obj/structure/disposalpipe/segment" || p.Vars().TextV("icon_state", "") != "pipe-s" || p.Vars().IntV("dir", 2) != 0 {
						continue
					}
					east := util.Point{X: tile.Coord.X + 1, Y: tile.Coord.Y, Z: tile.Coord.Z}
					if !dmm.HasTile(east) {
						continue
					}
					free := true
					for _, n := range dmm.GetTile(east).Instances() {
						if dm.IsPath(n.Prefab().Path(), "/obj/structure/disposalpipe") {
							free = false
						}
					}
					if free {
						pipeTile, pipe = tile, p
						break
					}
				}
				if pipeTile != nil {
					break
				}
			}
			if pipeTile == nil {
				t.Logf("%s: no suitable straight pipe", rel)
				continue
			}
			net := dmnet.Detect(env, pipe)
			if changed, warning := net.Connect(pipeTile, dm.DirEast); !changed || warning != "" {
				t.Fatalf("connect: %t %q", changed, warning)
			}
			east := dmm.GetTile(util.Point{X: pipeTile.Coord.X + 1, Y: pipeTile.Coord.Y, Z: pipeTile.Coord.Z})
			if changed, warning := net.Connect(east, dm.DirWest); !changed || warning != "" {
				t.Fatalf("connect east: %t %q", changed, warning)
			}
			cable := dmmap.PrefabStorage.Get("/obj/structure/cable/green", dmvars.FromParent(env.Objects["/obj/structure/cable/green"].Vars))
			cnet := dmnet.Detect(env, cable)
			if changed, _ := cnet.Connect(east, dm.DirNorth|dm.DirSouth); !changed {
				t.Fatal("cable")
			}
			coord := pipeTile.Coord
			if err := SaveV(env, dmm, path, Config{Format: FormatInitial}); err != nil {
				t.Fatal(err)
			}
			edited, _ := os.ReadFile(path)
			r, a = changedLines(out, edited)
			t.Logf("%s edit at %v: removed %d lines, added %d lines (crlf %d->%d)", rel, coord, r, a, bytes.Count(out, []byte("\r\n")), bytes.Count(edited, []byte("\r\n")))

			// Read it back: the pipe is a junction with the old ends and the new one.
			back := load(path)
			var got []string
			for _, i := range back.GetTile(coord).Instances() {
				if p := i.Prefab(); dm.IsPath(p.Path(), "/obj/structure/disposalpipe") {
					d, _ := dmnet.DisposalDirs(p.Path(), p.Vars().IntV("dir", 2), p.Vars().TextV("icon_state", ""))
					got = append(got, p.Path())
					if d != dm.DirNorth|dm.DirSouth|dm.DirEast {
						t.Errorf("connections after the reload: %d", d)
					}
				}
			}
			t.Logf("    after reload on that tile: %v", got)
		}
	}
}
