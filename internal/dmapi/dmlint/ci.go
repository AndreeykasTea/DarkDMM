package dmlint

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/rs/zerolog/log"
)

// CIRules are map checks of the code base CI. Every code base has its own set:
//   - Paradise/TG: tools/maplint rules + tools/ci/check_grep.py (text checks, TGM format);
//   - Baystation (Foundation-19 etc.): grep checks in test/run-test.sh.
type CIRules struct {
	Maplint *MaplintRules
	// TextChecks enables the check_grep.py text checks (dashes, Nanotrasen spelling, HTML tags) and TGM format.
	TextChecks bool
	// Greps are "run_test_fail ... grep '<pattern>' maps/**/*.dmm" checks.
	Greps []GrepRule
	// UnitTests is true when the code base has map unit tests (code/unit_tests/map_tests.dm).
	UnitTests bool
}

// GrepRule is a grep check applied to every line of the map.
type GrepRule struct {
	Name    string
	Pattern *regexp.Regexp
}

var reRunTestFail = regexp.MustCompile(`run_test_fail\s+"([^"]+)"\s+"grep\s+(?:-\w+\s+)*'([^']+)'\s+maps/`)

// LoadCIRules reads CI checks of the code base.
func LoadCIRules(rootDir string) *CIRules {
	rules := &CIRules{Maplint: LoadMaplint(rootDir)}

	if _, err := os.Stat(filepath.Join(rootDir, "tools", "ci", "check_grep.py")); err == nil {
		rules.TextChecks = true
	}

	if data, err := os.ReadFile(filepath.Join(rootDir, "test", "run-test.sh")); err == nil {
		for _, m := range reRunTestFail.FindAllStringSubmatch(string(data), -1) {
			re, err := regexp.Compile(m[2])
			if err != nil {
				log.Printf("unable to compile CI grep [%s]: %v", m[2], err)
				continue
			}
			rules.Greps = append(rules.Greps, GrepRule{Name: m[1], Pattern: re})
		}
	}

	if _, err := os.Stat(filepath.Join(rootDir, "code", "unit_tests", "map_tests.dm")); err == nil {
		rules.UnitTests = true
	}

	log.Printf("CI rules: maplint [%t], text checks [%t], greps [%d], unit tests [%t]",
		rules.Maplint != nil, rules.TextChecks, len(rules.Greps), rules.UnitTests)
	return rules
}
