package dmlint

import (
	"regexp"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func testMap(t *testing.T) (*dmenv.Dme, *dmmap.Dmm) {
	env := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	add := func(path string, vars ...string) {
		mv := &dmvars.MutableVariables{}
		mv.Put("name", `"thing"`)
		mv.Put("desc", "null")
		mv.Put("dir", "2")
		for i := 0; i+1 < len(vars); i += 2 {
			mv.Put(vars[i], vars[i+1])
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: mv.ToImmutable()}
	}
	add("/area/space")
	add("/turf/floor")
	add("/turf/wall")
	add("/obj/structure/table", "layer", "2")
	add("/obj/item/paper", "info", "null")
	dmmap.PrefabStorage.Free()
	dmmap.BaseArea = dmmap.PrefabStorage.Get("/area/space", dmvars.FromParent(env.Objects["/area/space"].Vars))
	dmmap.BaseTurf = dmmap.PrefabStorage.Get("/turf/floor", dmvars.FromParent(env.Objects["/turf/floor"].Vars))
	dmm := &dmmap.Dmm{}
	dmm.SetMapSize(5, 5, 1)
	return env, dmm
}

func put(env *dmenv.Dme, dmm *dmmap.Dmm, x, y int, path string, vars ...string) {
	v := dmvars.FromParent(env.Objects[path].Vars)
	for i := 0; i+1 < len(vars); i += 2 {
		v = dmvars.Set(v, vars[i], vars[i+1])
	}
	dmm.GetTile(util.Point{X: x, Y: y, Z: 1}).InstancesAdd(dmmap.PrefabStorage.Get(path, v))
}

func find(issues []Issue, cat Category) []Issue {
	var res []Issue
	for _, i := range issues {
		if i.Category == cat {
			res = append(res, i)
		}
	}
	return res
}

func TestDashes(t *testing.T) {
	env, dmm := testMap(t)
	put(env, dmm, 2, 2, "/obj/item/paper", "name", "\"Записка - важная, 5 - штук\"")
	put(env, dmm, 3, 3, "/obj/item/paper", "desc", "\"Correct english - text — ok\"")

	issues := find(Check(env, dmm, Options{SavesAsTGM: true, TextChecks: true}), CatText)
	if len(issues) != 1 || issues[0].Fix == nil {
		t.Fatalf("issues: %v", issues)
	}
	issues[0].Fix()
	tile := dmm.GetTile(util.Point{X: 2, Y: 2, Z: 1}).Instances()
	name, _ := tile[len(tile)-1].Prefab().Vars().Value("name")
	if name != "\"Записка — важная, 5 — штук\"" {
		t.Fatalf("fixed value: %s", name)
	}
	if again := find(Check(env, dmm, Options{SavesAsTGM: true, TextChecks: true}), CatText); len(again) != 0 {
		t.Fatalf("after fix: %v", again)
	}
}

func TestTextChecks(t *testing.T) {
	cases := map[string]int{
		`"NanoTrasen station"`:      1,
		`"Станция \"Нанотрейзен\""`: 0,
		`"Станция Нанотрейзен!"`:    1,
		`"<B>bold</B>"`:             1,
		`"<b>bold</b>"`:             0,
		`"Nanotrasen"`:              0,
	}
	for value, want := range cases {
		if got := len(textProblems(value)); got != want {
			t.Errorf("textProblems(%s) = %d, want %d", value, got, want)
		}
	}
}

func TestUnknownVariable(t *testing.T) {
	env, dmm := testMap(t)
	put(env, dmm, 1, 1, "/obj/structure/table", "no_such_var", "1")
	issues := find(Check(env, dmm, Options{SavesAsTGM: true, TextChecks: true}), CatUnknownVar)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "no_such_var") {
		t.Fatalf("issues: %v", issues)
	}
	issues[0].Fix()
	if again := find(Check(env, dmm, Options{SavesAsTGM: true, TextChecks: true}), CatUnknownVar); len(again) != 0 {
		t.Fatalf("after fix: %v", again)
	}
}

func TestBadValues(t *testing.T) {
	env, _ := testMap(t)
	cases := map[string]bool{
		`"ok"`:                    false,
		`list(1,2)`:               false,
		`"unclosed`:               true,
		`list(1,2`:                true,
		`/obj/structure/table`:    false,
		`/obj/structure/no_table`: true,
		`"/obj/in/string"`:        false,
		`"escaped \" quote"`:      false,
	}
	for value, bad := range cases {
		if got := badValue(env, value) != ""; got != bad {
			t.Errorf("badValue(%s) = %v, want %v", value, got, bad)
		}
	}
}

func TestDuplicatesAndTurfs(t *testing.T) {
	env, dmm := testMap(t)
	put(env, dmm, 3, 3, "/obj/structure/table")
	put(env, dmm, 3, 3, "/obj/structure/table")
	put(env, dmm, 4, 4, "/turf/wall")

	issues := Check(env, dmm, Options{SavesAsTGM: true, Extra: true})
	dups := find(issues, CatDuplicate)
	turfs := find(issues, CatTurfArea)
	if len(dups) != 1 || len(turfs) != 1 {
		t.Fatalf("dups %d turfs %d", len(dups), len(turfs))
	}
	dups[0].Fix()
	turfs[0].Fix()
	issues = Check(env, dmm, Options{SavesAsTGM: true, Extra: true})
	if len(find(issues, CatDuplicate))+len(find(issues, CatTurfArea)) != 0 {
		t.Fatalf("after fix: %v", issues)
	}
	// The last turf (the wall) is kept.
	for _, i := range dmm.GetTile(util.Point{X: 4, Y: 4, Z: 1}).Instances() {
		if i.Prefab().Path() == "/turf/floor" {
			t.Fatal("the first turf must be removed")
		}
	}
}

func TestMaplintRules(t *testing.T) {
	lint, err := parseLint("test.yml", []byte(`
help: "help text"
/turf:
  banned_neighbors:
    - /turf
/obj/structure/table:
  banned_neighbors:
    /obj/structure/table: { identical: true }
/obj/item/paper:
  banned_variables:
    info:
      deny: ["bad"]
=/obj/item:
  banned: TRUE
`))
	if err != nil {
		t.Fatal(err)
	}
	rules := &MaplintRules{Lints: []*maplintLint{lint}}

	env, dmm := testMap(t)
	put(env, dmm, 1, 1, "/turf/wall")           // Two turfs.
	put(env, dmm, 2, 2, "/obj/structure/table") // Two identical tables.
	put(env, dmm, 2, 2, "/obj/structure/table")
	put(env, dmm, 3, 3, "/obj/item/paper", "info", `"bad"`) // Denied value.
	put(env, dmm, 4, 4, "/obj/item/paper", "info", `"ok"`)  // Allowed.

	issues := find(Check(env, dmm, Options{Maplint: rules, SavesAsTGM: true, TextChecks: true}), CatMaplint)
	byCoord := map[int]int{}
	for _, i := range issues {
		byCoord[i.Coord.X]++
		if i.Help != "help text" {
			t.Fatalf("help is lost: %v", i)
		}
	}
	// Every turf on (1,1) reports the other one, every table reports the other one.
	if byCoord[1] != 2 || byCoord[2] != 2 || byCoord[3] != 1 || byCoord[4] != 0 {
		t.Fatalf("issues: %v", byCoord)
	}
}

func TestCIGreps(t *testing.T) {
	env, dmm := testMap(t)
	put(env, dmm, 2, 2, "/obj/structure/table", "layer", "3")
	greps := []GrepRule{{Name: "maps contain no layer adjustments", Pattern: regexp.MustCompile(`layer = `)}}
	issues := find(Check(env, dmm, Options{Greps: greps}), CatGrep)
	if len(issues) != 1 || issues[0].Fix == nil {
		t.Fatalf("issues: %v", issues)
	}
	issues[0].Fix()
	if again := find(Check(env, dmm, Options{Greps: greps}), CatGrep); len(again) != 0 {
		t.Fatalf("after fix: %v", again)
	}
}
