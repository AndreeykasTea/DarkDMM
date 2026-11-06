package dmnet

import (
	"os"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func TestRealBluePipe(t *testing.T) {
	path := os.Getenv("DMNET_DME")
	if path == "" {
		t.Skip()
	}
	env, err := dmenv.New(path)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	p := "/obj/machinery/atmospherics/pipe/simple/hidden/blue"
	t.Logf("type vars: dir=%q icon_state=%q", env.Objects[p].Vars.ValueV("dir", "?"), env.Objects[p].Vars.ValueV("icon_state", "?"))
	net := Detect(env, dmmap.PrefabStorage.Get(p, dmvars.FromParent(env.Objects[p].Vars)))
	t.Logf("simple=%s manifold=%q cap=%q", net.simplePath, net.manifoldPath, net.capPath)
	for _, dirs := range []int{4 | 8, 1 | 4, 4} {
		tile := &dmmap.Tile{Coord: util.Point{X: 1, Y: 1, Z: 1}}
		changed, warn := net.Connect(tile, dirs)
		for _, i := range tile.Instances() {
			t.Logf("dirs %d -> changed=%v warn=%q %s own=%v dir=%s", dirs, changed, warn, i.Prefab().Path(), i.Prefab().Vars().Iterate(), i.Prefab().Vars().ValueV("dir", "?"))
		}
	}
}
