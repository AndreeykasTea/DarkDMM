package lighting

import (
	"math"
	"testing"

	"sdmm/internal/dmapi/dmvars"
)

func varsOf(kv ...string) *dmvars.Variables {
	mv := &dmvars.MutableVariables{}
	for i := 0; i+1 < len(kv); i += 2 {
		mv.Put(kv[i], kv[i+1])
	}
	return mv.ToImmutable()
}

// The reason of the "every tile is a light source" bug on Baystation builds: /turf has luminosity = 1.
func TestTurfLuminosityIsNotALight(t *testing.T) {
	if has, _ := extractGeneric(varsOf("luminosity", "1"), true); has {
		t.Fatal("turf luminosity is not a light source")
	}
	if has, _ := extractGeneric(varsOf("light_range", "3"), true); !has {
		t.Fatal("a turf with light_range must shine")
	}
	if has, _ := extractGeneric(varsOf("luminosity", "4"), false); !has {
		t.Fatal("an old object with luminosity must shine")
	}
}

func TestBayEmitters(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		vars  *dmvars.Variables
		has   bool
		power bool
		outer float32
	}{
		{"computer", "/obj/machinery/computer/foo", varsOf("light_max_bright_on", ".2", "light_inner_range_on", ".1", "light_outer_range_on", "2", "light_outer_range", "0"), true, true, 2},
		{"lamp on", "/obj/item/device/flashlight/lamp", varsOf("on", "1", "flashlight_max_bright", ".3", "flashlight_inner_range", "2", "flashlight_outer_range", "5"), true, false, 5},
		{"flashlight off", "/obj/item/device/flashlight", varsOf("on", "0", "flashlight_max_bright", ".6", "flashlight_outer_range", "4"), false, false, 0},
		{"airlock", "/obj/machinery/door/airlock", varsOf("locked", "0"), false, true, 0},
		{"bolted airlock", "/obj/machinery/door/airlock/x", varsOf("locked", "1"), true, true, 1},
		{"apc", "/obj/machinery/power/apc/critical", varsOf(), true, false, 1},
		{"plain floor", "/turf/simulated/floor", varsOf("light_outer_range", "0"), false, false, 0},
		{"bluegrid", "/turf/simulated/floor/bluegrid", varsOf("light_outer_range", "2", "light_max_bright", ".5"), true, false, 2},
	}
	for _, c := range cases {
		has, power, src := extractBayAtom(c.path, c.vars)
		if has != c.has || power != c.power || (has && (src.Range != c.outer || !src.Bay)) {
			t.Errorf("%s: has %t, power %t, src %+v", c.name, has, power, src)
		}
	}
}

func TestAreaPower(t *testing.T) {
	scan := &powerScan{areas: map[string]*areaPower{}, hasBelow: map[int]bool{}}
	scan.area("/area/apc").hasAPC, scan.area("/area/apc").apcLightOn = true, true
	scan.area("/area/apc_off").hasAPC = true
	a := scan.area("/area/switch_off")
	a.hasAPC, a.apcLightOn, a.switched, a.switchOn = true, true, true, false

	normal := &prefabInfo{requiresPower: true, lightswitch: true}
	free := &prefabInfo{requiresPower: false, lightswitch: true}
	dead := &prefabInfo{requiresPower: true, alwaysUnpowered: true, lightswitch: true}

	cases := []struct {
		area *prefabInfo
		path string
		lamp bool
		want bool
	}{
		{normal, "/area/none", true, false},
		{normal, "/area/none", false, false},
		{normal, "/area/apc", true, true},
		{normal, "/area/apc", false, true},
		{normal, "/area/apc_off", true, false},
		{normal, "/area/apc_off", false, true},
		{normal, "/area/switch_off", true, false},
		{normal, "/area/switch_off", false, true},
		{free, "/area/none", true, true},
		{dead, "/area/apc", true, false},
	}
	for i, c := range cases {
		if got := scan.powered(c.area, c.path, c.lamp); got != c.want {
			t.Errorf("case %d (%s, lamp %t): got %t", i, c.path, c.lamp, got)
		}
	}
}

// A lamp must be as bright as in the game: LIGHTING_MULT_FACTOR * sqrt(max_bright^2 * falloff).
func TestBayCornerIsNotLinear(t *testing.T) {
	g := newGrid(11, 11)
	src := Source{X: 6, Y: 6, Bay: true, Range: 5, Inner: 1, Power: .5, Curve: 2, Angle: 360, R: 1, G: 1, B: 1}
	res := Compute(g, []Source{src})

	// The corner next to the lamp: dist = .7071, falloff = (5 - .7071) / 4 > 1, so it is clamped to 1.
	got, _, _ := res.Corner(5, 5)
	want := float32(.9 * math.Sqrt(.25))
	if math.Abs(float64(got-want)) > .001 {
		t.Fatalf("near corner: got %f, want %f", got, want)
	}
	// dx = 3.5, dy = 2.5, dist = 4.301: falloff = (5 - 4.301) / 4 = .1748, so .9 * sqrt(.25 * .1748) = .188.
	got, _, _ = res.Corner(9, 8)
	if math.Abs(float64(got-.1882)) > .002 {
		t.Fatalf("far corner: got %f", got)
	}
}

func anyLit(res *Result) bool {
	for _, v := range res.Lum {
		if v != 0 {
			return true
		}
	}
	return false
}

func TestBayLightGoesThroughOpenTiles(t *testing.T) {
	src := Source{X: 5, Y: 5, Bay: true, Range: 4, Inner: 1, Power: 1, Curve: 2, Angle: 360, R: 1, G: 1, B: 1}

	build := func(openX, openY int) *Result {
		lower, upper := newGrid(9, 9), newGrid(9, 9)
		if openX > 0 {
			upper.Open[upper.idx(openX, openY)] = true
		}
		lower.Upper = []UpperLevel{{Grid: upper, Sources: []Source{src}}}
		return Compute(lower, nil)
	}

	if res := build(0, 0); anyLit(res) {
		t.Fatal("the floor of the upper level must block the light")
	}
	res := build(6, 5)
	if !anyLit(res) {
		t.Fatal("the light must go through the open tile")
	}
	if r, _, _ := res.Corner(5, 4); r <= 0 { // A corner of the tile (6, 5).
		t.Fatal("the tile below the open one must be lit")
	}
	if r, _, _ := res.Corner(1, 1); r != 0 {
		t.Fatal("other tiles must stay dark")
	}

	// A wall on the lower level has no own corners.
	lower, upper := newGrid(9, 9), newGrid(9, 9)
	upper.Open[upper.idx(6, 5)] = true
	lower.Opaque[lower.idx(6, 5)] = true
	lower.Upper = []UpperLevel{{Grid: upper, Sources: []Source{src}}}
	if anyLit(Compute(lower, nil)) {
		t.Fatal("an opaque tile below must not get the light")
	}

	// Two levels: all tiles between must be open.
	lower, mid, top := newGrid(9, 9), newGrid(9, 9), newGrid(9, 9)
	top.Open[top.idx(6, 5)] = true
	lower.Upper = []UpperLevel{{Grid: mid}, {Grid: top, Sources: []Source{src}}}
	if anyLit(Compute(lower, nil)) {
		t.Fatal("a closed middle level must block the light")
	}
	mid.Open[mid.idx(6, 5)] = true
	if !anyLit(Compute(lower, nil)) {
		t.Fatal("open middle level must pass the light")
	}
}
