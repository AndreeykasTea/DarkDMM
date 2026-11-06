package dmlint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
)

// Runs the linter on real maps, if DMLINT_DME and DMLINT_DMM (separated with ";") are set.
// Maps which pass the CI must have no issues.
func TestRealMap(t *testing.T) {
	dmePath, dmmPaths := os.Getenv("DMLINT_DME"), os.Getenv("DMLINT_DMM")
	if dmePath == "" || dmmPaths == "" {
		t.Skip("DMLINT_DME/DMLINT_DMM are not set")
	}
	env, err := dmenv.New(dmePath)
	if err != nil {
		t.Fatal(err)
	}
	ci := LoadCIRules(env.RootDir)
	if ci.Maplint != nil && len(ci.Maplint.Errors) > 0 {
		t.Fatalf("maplint rules errors: %v", ci.Maplint.Errors)
	}
	t.Logf("CI: maplint %t, text %t, greps %d", ci.Maplint != nil, ci.TextChecks, len(ci.Greps))

	for _, dmmPath := range strings.Split(dmmPaths, ";") {
		dmmap.PrefabStorage.Free()
		dmmap.Init(env)
		data, err := dmmdata.New(dmmPath)
		if err != nil {
			t.Logf("%s: not parsed: %v", dmmPath, err)
			continue
		}
		dmm, _ := dmmap.New(env, data, "")
		rel, _ := filepath.Rel(env.RootDir, dmmPath)

		issues := Check(env, dmm, Options{Maplint: ci.Maplint, TextChecks: ci.TextChecks, RequireTGM: ci.TextChecks, Greps: ci.Greps, MapFile: filepath.ToSlash(rel), SavesAsTGM: IsTGMFile(dmmPath)})
		t.Logf("%s: %d issues", rel, len(issues))
		for i, issue := range issues {
			if i < 10 {
				t.Logf("    [%s] %s %s", issue.Category, issue.Coord, issue.Message)
			}
		}
		if errors, _, _ := Summary(issues); errors > 0 {
			t.Errorf("%s passes the CI, but has %d errors", rel, errors)
		}
	}
}
