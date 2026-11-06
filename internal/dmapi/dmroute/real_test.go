package dmroute

import (
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/util"
)

func TestRealCyberiad(t *testing.T) {
	dmePath, dmmPath := os.Getenv("DMROUTE_DME"), os.Getenv("DMROUTE_DMM")
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

	// Start in the central hallway.
	var start *util.Point
	for _, tile := range dmm.Tiles {
		for _, i := range tile.Instances() {
			if strings.HasPrefix(i.Prefab().Path(), "/area/station/hallway/primary/central") {
				dense := false
				for _, j := range tile.Instances() {
					if dm.IsPath(j.Prefab().Path(), "/turf/simulated/wall") {
						dense = true
					}
				}
				if !dense {
					c := tile.Coord
					start = &c
				}
			}
		}
		if start != nil {
			break
		}
	}
	if start == nil {
		t.Fatal("no hallway")
	}
	g := Build(dmm, start.Z)

	all := map[string]bool{}
	for _, r := range db.Regions {
		for _, a := range r.Accesses {
			all[a.Value] = true
		}
	}
	none := g.Reach(*start, nil, false)
	full := g.Reach(*start, all, false)
	var engineer map[string]bool
	for _, j := range db.Jobs {
		if j.Name == "Инженер" {
			engineer = map[string]bool{}
			for _, a := range j.Accesses {
				engineer[a] = true
			}
		}
	}
	eng := g.Reach(*start, engineer, false)
	t.Logf("start %v: no access %d tiles (%d closed doors), engineer %d, all %d", *start, none.Reachable, len(none.ClosedDoors), eng.Reachable, full.Reachable)
	if !(none.Reachable > 500 && none.Reachable < eng.Reachable && eng.Reachable < full.Reachable) {
		t.Fatal("unexpected reachability")
	}
}
