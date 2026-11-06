package dmaccess

import (
	"os"
	"reflect"
	"testing"
)

func TestParseFormatList(t *testing.T) {
	if got := ParseList("list(12,13)"); !reflect.DeepEqual(got, []string{"12", "13"}) {
		t.Fatal(got)
	}
	if got := ParseList(`list("security", "a,b")`); !reflect.DeepEqual(got, []string{`"security"`, `"a,b"`}) {
		t.Fatal(got)
	}
	if got := ParseList("null"); got != nil {
		t.Fatal(got)
	}
	if got := FormatList([]string{"13", "2", "10"}); got != "list(2,10,13)" {
		t.Fatal(got)
	}
}

func TestParse(t *testing.T) {
	db := parse([]string{`
#define ACCESS_SECURITY 1 // Security equipment
#define ACCESS_BRIG 2
#define ACCESS_MAINT_TUNNELS 12
#define ACCESS_ALIEN 999
/proc/get_region_accesses(code)
	switch(code)
		if(REGION_SECURITY) //security
			return list(ACCESS_SECURITY, ACCESS_BRIG)
		if(REGION_ENGINEERING)
			return list(ACCESS_MAINT_TUNNELS)

/proc/get_region_accesses_name(code)
	switch(code)
		if(REGION_SECURITY) //security
			return "Security"
		if(REGION_ENGINEERING)
			return "Engineering"

/proc/get_access_desc(A)
	switch(A)
		if(ACCESS_BRIG)
			return "Holding Cells"
`})
	if len(db.Accesses) != 4 || len(db.Regions) != 3 {
		t.Fatalf("accesses %d regions %d", len(db.Accesses), len(db.Regions))
	}
	if a, _ := db.ByValue("2"); a.Name() != "Камеры" {
		t.Fatal(a.Name())
	}
	if a, _ := db.ByValue("1"); a.Name() != "Security equipment" {
		t.Fatal(a.Name())
	}
	if r := db.RegionsOf("12"); len(r) != 1 || r[0].Name != "Инженерный отдел" {
		t.Fatal(r)
	}
}

// Checks parsing of the real code base, if it exists on this machine.
func TestRealCodebase(t *testing.T) {
	root := os.Getenv("DMACCESS_ROOT")
	if root == "" {
		t.Skip("DMACCESS_ROOT is not set")
	}
	db := Load(root)
	for _, r := range db.Regions {
		t.Logf("%s: %d", r.Name, len(r.Accesses))
	}
	for i, a := range db.Accesses {
		if i < 6 {
			t.Logf("%s = %s: %s", a.Define, a.Value, a.Name())
		}
	}
	for _, j := range db.Jobs {
		t.Logf("job %s: %d accesses", j.Name, len(j.Accesses))
	}
	if len(db.Accesses) < 50 {
		t.Fatal("too few accesses")
	}
}
