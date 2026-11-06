package tools

import (
	"testing"

	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/util"
)

func TestBrushTiles(t *testing.T) {
	if n := len(brushTiles(util.Point{X: 10, Y: 10}, 0)); n != 1 {
		t.Fatalf("radius 0 must give one tile, got %d", n)
	}
	tiles := brushTiles(util.Point{X: 10, Y: 10, Z: 2}, 3)
	if len(tiles) < 28 || len(tiles) > 49 {
		t.Fatalf("unexpected brush size: %d", len(tiles))
	}
	for _, c := range tiles {
		if c.Z != 2 {
			t.Fatalf("brush must keep z-level")
		}
	}
}

func TestPickWeighted(t *testing.T) {
	a := dmmprefab.New(10, "/obj/a", nil)
	b := dmmprefab.New(11, "/obj/b", nil)
	palette := []ScatterEntry{{Prefab: a, Weight: 90}, {Prefab: b, Weight: 10}}

	counts := map[string]int{}
	for i := 0; i < 10000; i++ {
		counts[pickWeighted(palette).Path()]++
	}
	if counts["/obj/a"] < 8500 || counts["/obj/a"] > 9500 {
		t.Fatalf("weights are not respected: %v", counts)
	}
}
