package dmlint

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"sdmm/internal/app/render/lighting"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmroute"
	"sdmm/internal/util"
)

// Checks all maps of a code base by all tools, if ALL_ROOT is set.
func TestAllMaps(t *testing.T) {
	root := os.Getenv("ALL_ROOT")
	if root == "" {
		t.Skip()
	}
	dmes, _ := filepath.Glob(filepath.Join(root, "*.dme"))
	if len(dmes) == 0 {
		t.Fatal("no .dme")
	}
	env, err := dmenv.New(dmes[0])
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	dmmap.Init(env)
	t.Logf("objects %d, base area %s, base turf %s", len(env.Objects), dmmap.BaseArea.Path(), dmmap.BaseTurf.Path())
	left, _ := filepath.Glob(filepath.Join(root, ".~dmm_env_*"))
	t.Logf("temp files left: %d", len(left))

	db := dmaccess.Load(root)
	dmaccess.SetCurrent(db)
	defer dmaccess.SetCurrent(nil)
	t.Logf("access %d, regions %d, jobs %d", len(db.Accesses), len(db.Regions), len(db.Jobs))

	rules := LoadCIRules(root)
	lintsCount := 0
	if rules.Maplint != nil {
		lintsCount = len(rules.Maplint.Lints)
		for _, e := range rules.Maplint.Errors {
			t.Errorf("maplint rule is not loaded: %s", e)
		}
	}
	t.Logf("rules: maplint %d, greps %d, unit %t", lintsCount, len(rules.Greps), rules.UnitTests)

	var maps []string
	mapsDir := filepath.Join(root, "maps")
	if _, err := os.Stat(mapsDir); err != nil {
		mapsDir = filepath.Join(root, "_maps") // Paradise and TG.
	}
	_ = filepath.WalkDir(mapsDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".dmm") {
			maps = append(maps, p)
		}
		return nil
	})
	sort.Strings(maps)
	t.Logf("maps: %d", len(maps))

	for _, full := range maps {
		rel, _ := filepath.Rel(root, full)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: PANIC %v", rel, r)
				}
			}()
			data, err := dmmdata.New(full)
			if err != nil {
				t.Logf("%-52s parse error: %v", rel, err)
				return
			}

			// Save without changes.
			tmp := filepath.Join(t.TempDir(), "out.dmm")
			orig, _ := os.ReadFile(full)
			save := data.SaveDM
			if data.IsTgm {
				save = data.SaveTGM
			}
			if err := save(tmp); err != nil {
				t.Errorf("%s: save: %v", rel, err)
				return
			}
			out, _ := os.ReadFile(tmp)
			same := bytes.Equal(orig, out)
			note := ""
			if !same {
				norm := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
				note = fmt.Sprintf(" SAVE-DIFF(crlf %d->%d, same text %t)", bytes.Count(orig, []byte("\r\n")), bytes.Count(out, []byte("\r\n")), bytes.Equal(norm(orig), norm(out)))
			}

			dmm, unknown := dmmap.New(env, data, "")

			start := time.Now()
			issues := Check(env, dmm, Options{Maplint: rules.Maplint, TextChecks: rules.TextChecks, RequireTGM: rules.TextChecks, Greps: rules.Greps, UnitTests: rules.UnitTests, MapFile: filepath.ToSlash(rel), SavesAsTGM: data.IsTgm})
			errs, warns, _ := Summary(issues)
			lintMs := time.Since(start).Milliseconds()

			ex := lighting.NewExtractor()
			totalSources, off := 0, 0
			worst := float32(0)
			tiles := dmm.MaxX * dmm.MaxY
			for z := 1; z <= dmm.MaxZ; z++ {
				grid, sources := ex.Build(dmm, z)
				res := lighting.Compute(grid, sources)
				totalSources += len(sources)
				off += len(grid.Unpowered)
				if share := float32(len(sources)) / float32(tiles); share > worst {
					worst = share
				}
				g := dmroute.Build(dmm, z)
				g.Reach(util.Point{X: dmm.MaxX / 2, Y: dmm.MaxY / 2, Z: z}, nil, false)
				_ = res
			}
			t.Logf("%-52s %3dx%3dx%d tgm=%-5t unknown=%-3d err=%-3d warn=%-4d lint=%4dms light=%-5d off=%-3d max-share=%.2f%s", rel, dmm.MaxX, dmm.MaxY, dmm.MaxZ, data.IsTgm, len(unknown), errs, warns, lintMs, totalSources, off, worst, note)
			// Small shuttles and arenas could have floors glowing everywhere (lava, light floors), it is fine for them.
			if worst > .25 && tiles >= 2500 {
				t.Errorf("%s: too many light sources", rel)
			}
			if errs > 0 {
				shown := 0
				for _, i := range issues {
					if i.Severity == SeverityError && shown < 3 {
						t.Logf("      ERR [%d,%d,%d] %s: %s", i.Coord.X, i.Coord.Y, i.Coord.Z, i.Category, i.Message)
						shown++
					}
				}
			}
			if len(unknown) > 0 {
				t.Logf("      unknown: %v", first(unknown, 4))
			}
		}()
	}
}

func first(v any, n int) string {
	s := fmt.Sprint(v)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
