package lighting

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
)

// Checks real Baystation maps, if LIGHTING_BAY_DME and LIGHTING_BAY_DMM (";" separated) are set.
// With LIGHTING_BAY_PNG (a directory) it also draws what the lighting looks like.
func TestBayRealMaps(t *testing.T) {
	dmePath, maps := os.Getenv("LIGHTING_BAY_DME"), os.Getenv("LIGHTING_BAY_DMM")
	if dmePath == "" {
		t.Skip()
	}
	env, err := dmenv.New(dmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !func() bool { dmmap.PrefabStorage.Free(); dmmap.Init(env); return dmmap.IsBaystation() }() {
		t.Fatal("the environment is not detected as Baystation")
	}
	for _, leftover := range mustGlob(t, filepath.Join(filepath.Dir(dmePath), ".~dmm_env_*")) {
		t.Errorf("temporary file is not removed: %s", leftover)
	}
	t.Logf("world area: %s, turf: %s", dmmap.BaseArea.Path(), dmmap.BaseTurf.Path())

	for _, path := range strings.Split(maps, ";") {
		dmmap.PrefabStorage.Free()
		dmmap.Init(env)
		data, err := dmmdata.New(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		dmm, unknown := dmmap.New(env, data, "")
		t.Logf("=== %s %dx%dx%d, unknown prefabs: %d", filepath.Base(path), dmm.MaxX, dmm.MaxY, dmm.MaxZ, len(unknown))
		if len(unknown) > 0 && len(unknown) < 10 {
			t.Logf("unknown: %v", unknown)
		}

		ex := NewExtractor()
		for z := 1; z <= dmm.MaxZ; z++ {
			start := time.Now()
			grid, sources := ex.Build(dmm, z)
			buildTime := time.Since(start)
			start = time.Now()
			res := Compute(grid, sources)
			computeTime := time.Since(start)

			tiles := dmm.MaxX * dmm.MaxY
			if len(sources) > tiles/4 {
				t.Errorf("%s z%d: %d sources on %d tiles", path, z, len(sources), tiles)
			}

			// What shines.
			kinds := map[string]int{}
			turfs := 0
			for _, tile := range dmm.Tiles {
				if tile.Coord.Z != z {
					continue
				}
				for _, i := range tile.Instances() {
					if ex.info(i.Prefab()).hasLight {
						p := i.Prefab().Path()
						if dm.IsPath(p, "/turf") {
							turfs++
						}
						kinds[p]++
					}
				}
			}
			var keys []string
			for k := range kinds {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(a, b int) bool { return kinds[keys[a]] > kinds[keys[b]] })
			var top []string
			for n, k := range keys {
				if n < 7 {
					top = append(top, fmt.Sprintf("%d %s", kinds[k], strings.TrimPrefix(k, "/obj/machinery/")))
				}
			}

			fromUpper, open, full, lit := 0, 0, 0, 0
			for _, u := range grid.Upper {
				fromUpper += len(u.Sources)
			}
			for i := range grid.Open {
				if grid.Open[i] {
					open++
				}
				if grid.Fullbright[i] {
					full++
				}
			}
			maxLum := float32(0)
			for i := 0; i < len(res.Lum); i += 3 {
				m := max(res.Lum[i], res.Lum[i+1], res.Lum[i+2])
				maxLum = max(maxLum, m)
				if m > 0 {
					lit++
				}
			}
			t.Logf("z%d: sources %d (turfs %d), no power %d, from above %d (open tiles %d), fullbright %d, lit corners %d, max %.2f, build %d ms, compute %d ms",
				z, len(sources), turfs, len(grid.Unpowered), fromUpper, open, full, lit, maxLum, buildTime.Milliseconds(), computeTime.Milliseconds())
			t.Logf("    %s", strings.Join(top, "; "))
			if maxLum > 1.0001 {
				t.Errorf("%s z%d: not normalized: %f", path, z, maxLum)
			}

			if dir := os.Getenv("LIGHTING_BAY_PNG"); dir != "" {
				name := fmt.Sprintf("%s_z%d.png", strings.TrimSuffix(filepath.Base(path), ".dmm"), z)
				if err := drawLightingPng(filepath.Join(dir, name), grid, sources, res); err != nil {
					t.Error(err)
				}
			}
		}
	}
}

func mustGlob(t *testing.T, pattern string) []string {
	res, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// drawLightingPng draws walls and floors multiplied with lighting. The same as the render does, without icons.
func drawLightingPng(path string, grid *Grid, sources []Source, res *Result) error {
	const scale = 6
	const ambient = .15
	img := image.NewRGBA(image.Rect(0, 0, grid.W*scale, grid.H*scale))
	toByte := func(v float32) uint8 { return uint8(math.Min(255, float64(v)*255)) }

	for y := 1; y <= grid.H; y++ {
		for x := 1; x <= grid.W; x++ {
			idx := grid.idx(x, y)
			base := float32(.85)
			if grid.Opaque[idx] {
				base = .4
			}
			for py := 0; py < scale; py++ {
				for px := 0; px < scale; px++ {
					u, v := (float32(px)+.5)/scale, (float32(py)+.5)/scale
					var c [3]float32
					r00, g00, b00 := res.Corner(x-1, y-1)
					r10, g10, b10 := res.Corner(x, y-1)
					r01, g01, b01 := res.Corner(x-1, y)
					r11, g11, b11 := res.Corner(x, y)
					mix := func(a, b, c, d float32) float32 {
						return max(0, (a*(1-u)+b*u)*(1-v)+(c*(1-u)+d*u)*v)
					}
					c[0], c[1], c[2] = mix(r00, r10, r01, r11), mix(g00, g10, g01, g11), mix(b00, b10, b01, b11)
					if grid.Fullbright[idx] {
						c = [3]float32{1, 1, 1}
					}
					k := func(l float32) uint8 { return toByte(base * (ambient + (1-ambient)*l)) }
					img.SetRGBA((x-1)*scale+px, (grid.H-y)*scale+scale-1-py, color.RGBA{R: k(c[0]), G: k(c[1]), B: k(c[2]), A: 255})
				}
			}
		}
	}

	mark := func(s Source, c color.RGBA) {
		cx, cy := (s.X-1)*scale+scale/2, (grid.H-s.Y)*scale+scale/2
		img.SetRGBA(cx, cy, c)
		img.SetRGBA(cx-1, cy, c)
		img.SetRGBA(cx, cy-1, c)
		img.SetRGBA(cx-1, cy-1, c)
	}
	for _, s := range grid.Unpowered {
		mark(s, color.RGBA{R: 255, A: 255})
	}
	_ = sources

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
