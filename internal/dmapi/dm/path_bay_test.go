package dm

import "testing"

func TestIsPathComparesWholeParts(t *testing.T) {
	cases := []struct {
		orig, path string
		want       bool
	}{
		{"/obj/machinery/light", "/obj/machinery/light", true},
		{"/obj/machinery/light/small", "/obj/machinery/light", true},
		{"/obj/machinery/light_switch", "/obj/machinery/light", false},
		{"/obj/machinery/light_construct", "/obj/machinery/light", false},
		{"/obj/machinery/door_control", "/obj/machinery/door", false},
		{"/obj/machinery/disposal_switch", "/obj/machinery/disposal", false},
		{"/obj/machinery/light/small", "/obj/machinery/light/", true},
		{"/obj", "/obj/machinery", false},
	}
	for _, c := range cases {
		if got := IsPath(c.orig, c.path); got != c.want {
			t.Errorf("IsPath(%s, %s) = %t", c.orig, c.path, got)
		}
	}
}

func TestFixDir(t *testing.T) {
	if FixDir(0) != DirSouth || FixDir(DirEast) != DirEast {
		t.Fatal("dir = 0 is the south for BYOND")
	}
}
