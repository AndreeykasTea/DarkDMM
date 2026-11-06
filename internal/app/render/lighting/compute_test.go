package lighting

import (
	"math/rand"
	"testing"
	"time"
)

func makeGrid(w, h int) *Grid {
	return &Grid{
		W: w, H: h,
		Opaque:     make([]bool, w*h),
		Fullbright: make([]bool, w*h),
		BaseR:      make([]float32, w*h),
		BaseG:      make([]float32, w*h),
		BaseB:      make([]float32, w*h),
	}
}

func TestFalloffMatchesGame(t *testing.T) {
	g := makeGrid(21, 21)
	res := Compute(g, []Source{{X: 11, Y: 11, Range: 8, Power: 1, R: 1, G: 1, B: 1, Angle: 360}})

	// Corner right next to the source: dist^2 = .5^2+.5^2 = .5, sqrt(.5+1)/8 = .153 -> 0.847
	r, _, _ := res.Corner(10, 10)
	if r < .84 || r > .85 {
		t.Fatalf("unexpected near corner lum: %f", r)
	}
	// Far corner, out of range.
	r, _, _ = res.Corner(0, 0)
	if r != 0 {
		t.Fatalf("far corner must be dark: %f", r)
	}
}

func TestWallBlocksLight(t *testing.T) {
	g := makeGrid(20, 5)
	// Vertical wall at x=10.
	for y := 1; y <= 5; y++ {
		g.Opaque[g.idx(10, y)] = true
	}
	res := Compute(g, []Source{{X: 5, Y: 3, Range: 8, Power: 1, R: 1, G: 1, B: 1, Angle: 360}})

	// Corners behind the wall (x >= 11) must be dark.
	for cy := 0; cy <= 5; cy++ {
		for cx := 11; cx <= 20; cx++ {
			if r, _, _ := res.Corner(cx, cy); r != 0 {
				t.Fatalf("corner (%d,%d) behind the wall is lit: %f", cx, cy, r)
			}
		}
	}
	// The wall face itself (corner x=9) is lit.
	if r, _, _ := res.Corner(9, 2); r <= 0 {
		t.Fatalf("wall face must be lit")
	}
}

func TestDirectionalLight(t *testing.T) {
	g := makeGrid(21, 21)
	// Light pointing south (180 degrees) with 170 degrees cone.
	res := Compute(g, []Source{{X: 11, Y: 11, Range: 8, Power: 1, R: 1, G: 1, B: 1, Angle: 170, DirAngle: 180, OffsetY: .5}})
	south, _, _ := res.Corner(10, 7)
	north, _, _ := res.Corner(10, 14)
	if south <= 0 || north != 0 {
		t.Fatalf("directional light is wrong: south=%f north=%f", south, north)
	}
}

func TestColorNormalization(t *testing.T) {
	g := makeGrid(5, 5)
	res := Compute(g, []Source{{X: 3, Y: 3, Range: 4, Power: 10, R: 1, G: .5, B: 0, Angle: 360}})
	r, gg, b := res.Corner(2, 2)
	if r != 1 || gg < .49 || gg > .51 || b != 0 {
		t.Fatalf("color is not normalized: %f %f %f", r, gg, b)
	}
}

// A big station-like map with a lot of lights must be computed fast.
func TestPerformanceBigMap(t *testing.T) {
	const w, h = 255, 255
	g := makeGrid(w, h)
	rnd := rand.New(rand.NewSource(1))
	for i := range g.Opaque {
		g.Opaque[i] = rnd.Intn(5) == 0
	}
	var sources []Source
	for i := 0; i < 3000; i++ {
		sources = append(sources, Source{X: rnd.Intn(w) + 1, Y: rnd.Intn(h) + 1, Range: 8, Power: 1, R: 1, G: 1, B: 1, Angle: 170, DirAngle: 180})
	}
	start := time.Now()
	Compute(g, sources)
	elapsed := time.Since(start)
	t.Logf("computed %d sources on %dx%d in %s", len(sources), w, h, elapsed)
	if elapsed > 2*time.Second {
		t.Fatalf("lighting is too slow: %s", elapsed)
	}
}

func TestBayFalloff(t *testing.T) {
	// Full brightness up to the inner range, then linear fade to the outer range, brightness is squared.
	src := Source{Bay: true, Range: 5, Inner: 1, Power: .8, Curve: 2}
	if got := bayLum(src, .5, .5); got < .63 || got > .65 { // .8^2
		t.Fatalf("inside inner range: %f", got)
	}
	if got := bayLum(src, 4.5, 0); got < .079 || got > .081 { // (5-4.5)/4 * .64
		t.Fatalf("near outer range: %f", got)
	}
	if got := bayLum(src, 6, 0); got != 0 {
		t.Fatalf("out of range: %f", got)
	}
	// Cone to the north: nothing behind the light.
	cone := Source{Bay: true, Range: 5, Inner: 1, Power: 1, Curve: 2, DirY: 1}
	if bayLum(cone, 0, -2) != 0 || bayLum(cone, 0, 2) <= 0 {
		t.Fatal("cone light")
	}
}
