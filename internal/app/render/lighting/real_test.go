package lighting

import (
	"os"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
)

// Checks the light sources extraction on a real map, if LIGHTING_DME and LIGHTING_DMM are set.
func TestRealMapLighting(t *testing.T) {
	dmePath, dmmPath := os.Getenv("LIGHTING_DME"), os.Getenv("LIGHTING_DMM")
	if dmePath == "" {
		t.Skip()
	}
	env, err := dmenv.New(dmePath)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	dmmap.Init(env)
	data, err := dmmdata.New(dmmPath)
	if err != nil {
		t.Fatal(err)
	}
	dmm, _ := dmmap.New(env, data, "")

	grid, sources := NewExtractor().Build(dmm, 1)
	bay, fullbright := 0, 0
	for _, s := range sources {
		if s.Bay {
			bay++
		}
	}
	for _, f := range grid.Fullbright {
		if f {
			fullbright++
		}
	}
	res := Compute(grid, sources)
	lit := 0
	for i := 0; i < len(res.Lum); i += 3 {
		if res.Lum[i]+res.Lum[i+1]+res.Lum[i+2] > 0 {
			lit++
		}
	}
	t.Logf("sources %d (bay model %d), fullbright tiles %d, lit corners %d", len(sources), bay, fullbright, lit)
	if len(sources) == 0 || lit == 0 {
		t.Fatal("no light")
	}
}
