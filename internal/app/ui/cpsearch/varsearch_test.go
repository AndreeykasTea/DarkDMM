package cpsearch

import (
	"testing"

	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func TestValueEquals(t *testing.T) {
	cases := []struct {
		value, want string
		eq          bool
	}{
		{`"Bar"`, "Bar", true},
		{`"Bar"`, `"Bar"`, true},
		{"list(12, 13)", "list(12,13)", true},
		{"12", "13", false},
	}
	for _, c := range cases {
		if valueEquals(c.value, c.want) != c.eq {
			t.Errorf("valueEquals(%s, %s) != %v", c.value, c.want, c.eq)
		}
	}
}

func TestVarMatcher(t *testing.T) {
	parent := &dmvars.MutableVariables{}
	parent.Put("req_access", "null")
	parent.Put("name", `"airlock"`)
	typeVars := parent.ToImmutable()

	edited := dmmprefab.New(100, "/obj/machinery/door/airlock", dmvars.Set(dmvars.FromParent(typeVars), "req_access", "list(12)"))
	plain := dmmprefab.New(101, "/obj/machinery/door/airlock", dmvars.FromParent(typeVars))
	other := dmmprefab.New(102, "/obj/structure/table", dmvars.Set(dmvars.FromParent(typeVars), "req_access", "list(12)"))
	inst := func(p *dmmprefab.Prefab) *dmminstance.Instance {
		return dmminstance.New(util.Point{X: 1, Y: 1, Z: 1}, p)
	}

	s := &Search{}
	s.vars = varSearch{typeFilter: "/obj/machinery/door", name: "req_access", op: varOpEqual, value: "list(12)"}
	match, err := s.varMatcher()
	if err != nil {
		t.Fatal(err)
	}
	if !match(inst(edited)) || match(inst(plain)) || match(inst(other)) {
		t.Fatal("equal operator")
	}

	s.vars.op = varOpNotSet
	match, _ = s.varMatcher()
	if match(inst(edited)) || !match(inst(plain)) {
		t.Fatal("not set operator")
	}

	s.vars = varSearch{name: "name", op: varOpContains, value: "AIR"}
	match, _ = s.varMatcher()
	if !match(inst(plain)) {
		t.Fatal("contains must be case insensitive and use the type value")
	}
	s.vars.onlyEdited = true
	match, _ = s.varMatcher()
	if match(inst(plain)) {
		t.Fatal("only edited must ignore type values")
	}
}
