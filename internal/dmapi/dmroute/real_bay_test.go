package dmroute

import (
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/util"
)

func TestRealBayMap(t *testing.T) {
	dmePath, dmmPath := os.Getenv("DMROUTE_BAY_DME"), os.Getenv("DMROUTE_BAY_DMM")
	if dmePath == "" {
		t.Skip()
	}
	env, err := dmenv.New(dmePath)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	dmmap.Init(env)
	data, _ := dmmdata.New(dmmPath)
	dmm, _ := dmmap.New(env, data, "")
	db := dmaccess.Load(env.RootDir)

	g := Build(dmm, 1)
	// The start is a tile with the most reachable area without accesses among hallway tiles.
	var start *util.Point
	best := 0
	checked := 0
	for _, tile := range dmm.Tiles {
		if checked > 40 {
			break
		}
		for _, i := range tile.Instances() {
			p := i.Prefab().Path()
			if strings.HasPrefix(p, "/area") && (strings.Contains(p, "hallway") || strings.Contains(p, "corridor")) {
				if r := g.Reach(tile.Coord, nil, false); r.Reachable > best {
					c := tile.Coord
					start, best = &c, r.Reachable
				}
				checked++
			}
		}
	}
	if start == nil {
		t.Fatal("no hallway")
	}

	all := map[string]bool{}
	for _, a := range db.Accesses {
		all[a.Value] = true
	}
	none := g.Reach(*start, nil, false)
	full := g.Reach(*start, all, false)
	t.Logf("start %v: no access %d tiles (%d closed doors), all accesses %d", *start, none.Reachable, len(none.ClosedDoors), full.Reachable)
	if !(none.Reachable < full.Reachable) {
		t.Fatal("accesses must open more doors")
	}
}
