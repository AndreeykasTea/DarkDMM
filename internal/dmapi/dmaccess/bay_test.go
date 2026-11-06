package dmaccess

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBayContainmentRegionAndNames(t *testing.T) {
	db := parse([]string{`
#define ACCESS_CONTAINMENT_SCP_012 "ACCESS_CONTAINMENT_SCP_012"
#define ACCESS_DCLASS_LUXURY "ACCESS_DCLASS_LUXURY"
/datum/access/containment_scp_012
	id = ACCESS_CONTAINMENT_SCP_012
	desc = "SCP-012 Containment"
	access_type = ACCESS_TYPE_CONTAINMENT

/datum/access/dclass_luxury
	id = ACCESS_DCLASS_LUXURY
	desc = "D-Class Luxury"
	region = ACCESS_REGION_GENERAL
`})
	a, ok := db.ByValue(`"ACCESS_CONTAINMENT_SCP_012"`)
	if !ok {
		t.Fatal("no access")
	}
	if a.Name() != "Содержание SCP-012" {
		t.Fatal(a.Name())
	}
	regions := db.RegionsOf(a.Value)
	if len(regions) != 1 || regions[0].Name != "Содержание объектов" {
		t.Fatalf("regions: %+v", regions)
	}
	if a, _ := db.ByValue(`"ACCESS_DCLASS_LUXURY"`); a.Name() != "Класс D: люкс" {
		t.Fatal(a.Name())
	}
}

func TestBayMapValues(t *testing.T) {
	db := parse([]string{`#define ACCESS_SCIENCE_LVL3 "ACCESS_SCIENCE_LEVEL3"`})
	SetCurrent(db)
	defer SetCurrent(nil)

	// site53 has "req_access = newlist()" and defines without quotes.
	if got := ParseList("newlist()"); got != nil {
		t.Fatal(got)
	}
	if got := ParseList("list(ACCESS_SCIENCE_LVL3)"); !reflect.DeepEqual(got, []string{`"ACCESS_SCIENCE_LEVEL3"`}) {
		t.Fatal(got)
	}
}

func TestUnusedMapsAreSkipped(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("x.dme", "#include \"maps\\site53\\site53_define.dm\"\r\n#include \"code\\access.dm\"\r\n")
	write("code/access.dm", "#define ACCESS_A \"A\"\n")
	write("maps/site53/site53_access.dm", "#define ACCESS_B \"B\"\n")
	write("maps/torch/torch_access.dm", "#define ACCESS_TORCH \"T\"\n")

	db := Load(root)
	if _, ok := db.ByValue(`"T"`); ok {
		t.Fatal("accesses of an unused map must be skipped")
	}
	for _, v := range []string{`"A"`, `"B"`} {
		if _, ok := db.ByValue(v); !ok {
			t.Fatalf("no access %s", v)
		}
	}
}
