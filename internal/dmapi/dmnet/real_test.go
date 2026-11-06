package dmnet

import (
	"os"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
)

// Checks detection on a real code base, if DMNET_DME is set.
func TestRealEnvironment(t *testing.T) {
	path := os.Getenv("DMNET_DME")
	if path == "" {
		t.Skip("DMNET_DME is not set")
	}
	env, err := dmenv.New(path)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()

	check := func(p string, kind Kind) *Network {
		obj, ok := env.Objects[p]
		if !ok {
			t.Fatalf("no type %s", p)
		}
		net := Detect(env, dmmap.PrefabStorage.Get(p, dmvars.FromParent(obj.Vars)))
		if net.Kind != kind {
			t.Fatalf("%s: kind %v, want %v", p, net.Kind, kind)
		}
		return net
	}

	check("/obj/structure/cable", KindCable)
	check("/obj/structure/cable/yellow", KindCable)
	check("/obj/structure/disposalpipe/segment", KindDisposal)
	for _, p := range []string{"/obj/machinery/atmospherics/pipe/simple/hidden/supply", "/obj/machinery/atmospherics/pipe/simple/hidden/scrubbers"} {
		net := check(p, KindPipe)
		t.Logf("%s: manifold=%s 4w=%s cap=%s", p, net.manifoldPath, net.manifold4wPath, net.capPath)
		if net.manifoldPath == "" || net.manifold4wPath == "" || net.capPath == "" {
			t.Fatal("pipe family is incomplete")
		}
	}
	for _, p := range []string{disposalSegment, disposalJunction, disposalTrunk} {
		if _, ok := env.Objects[p]; !ok {
			t.Fatalf("no disposal type %s", p)
		}
	}
	cable := env.Objects["/obj/structure/cable"].Vars
	t.Logf("cable initial icon_state: %s, dir: %s", cable.ValueV("icon_state", ""), cable.ValueV("dir", ""))
}

// Checks Baystation specifics, if DMNET_BAY_DME is set.
func TestRealBayEnvironment(t *testing.T) {
	path := os.Getenv("DMNET_BAY_DME")
	if path == "" {
		t.Skip()
	}
	env, err := dmenv.New(path)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	get := func(p string) *Network {
		obj, ok := env.Objects[p]
		if !ok {
			t.Fatalf("no type %s", p)
		}
		return Detect(env, dmmap.PrefabStorage.Get(p, dmvars.FromParent(obj.Vars)))
	}

	cable := get("/obj/structure/cable/green")
	if cable.Kind != KindCable || !cable.cableDirs {
		t.Fatalf("cable: kind %v, dirs %t", cable.Kind, cable.cableDirs)
	}
	disposal := get("/obj/structure/disposalpipe/segment")
	if disposal.Kind != KindDisposal || !disposal.bayDisposals {
		t.Fatalf("disposal: kind %v, bay %t", disposal.Kind, disposal.bayDisposals)
	}
	pipe := get("/obj/machinery/atmospherics/pipe/simple/hidden/supply")
	t.Logf("pipe family: manifold=%q 4w=%q cap=%q", pipe.manifoldPath, pipe.manifold4wPath, pipe.capPath)
	if pipe.Kind != KindPipe || pipe.manifoldPath == "" || pipe.manifold4wPath == "" {
		t.Fatal("pipe family is incomplete")
	}
}
