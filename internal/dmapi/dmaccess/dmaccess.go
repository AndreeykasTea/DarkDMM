// Package dmaccess reads access definitions (ACCESS_* defines, their names and departments) from the code base.
// It doesn't depend on a specific code base: Paradise, TG and Bay style declarations are supported.
package dmaccess

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// Access is a single access declared with the "#define ACCESS_*".
type Access struct {
	Define string
	// Value is the value as it's written on the map: a number or a quoted string.
	Value string
	// Desc is a human-readable name of the access (from get_access_desc or the define comment).
	Desc string
}

// Name returns the best name to show in UI.
func (a *Access) Name() string {
	if ru, ok := accessNamesRu[a.Desc]; ok {
		return ru
	}
	if ru, ok := translateLevel(a.Desc); ok {
		return ru
	}
	if a.Desc != "" {
		return a.Desc
	}
	return strings.TrimPrefix(a.Define, "ACCESS_")
}

// Region is a department, which groups accesses.
type Region struct {
	Define   string
	Name     string
	Accesses []*Access
}

// DB stores all accesses of the environment.
type DB struct {
	Accesses []*Access
	Regions  []*Region
	// Jobs are presets of accesses.
	Jobs []Job

	byValue  map[string]*Access
	byDefine map[string]*Access
	// regionsByValue helps to find departments of the access.
	regionsByValue map[string][]*Region
}

func (db *DB) IsEmpty() bool {
	return db == nil || len(db.Accesses) == 0
}

// ByValue returns the access by its map value.
func (db *DB) ByValue(value string) (*Access, bool) {
	if db == nil {
		return nil, false
	}
	a, ok := db.byValue[value]
	return a, ok
}

// RegionsOf returns departments of the access.
func (db *DB) RegionsOf(value string) []*Region {
	if db == nil {
		return nil
	}
	return db.regionsByValue[value]
}

var (
	reDefine     = regexp.MustCompile(`(?m)^[ \t]*#define[ \t]+(ACCESS_[A-Za-z0-9_]+)[ \t]+(-?\d+|"[^"\n]*")[ \t]*(?://[ \t]*(.*))?$`)
	reDesc       = regexp.MustCompile(`if\s*\(\s*(ACCESS_[A-Za-z0-9_]+)\s*\)[^\n]*\n\s*return\s+"([^"]+)"`)
	reRegionList = regexp.MustCompile(`if\s*\(\s*((?:ACCESS_)?REGION_[A-Za-z0-9_]+)\s*\)[^\n]*\n\s*return\s+list\(([^)]*)\)`)
	reRegionName = regexp.MustCompile(`if\s*\(\s*((?:ACCESS_)?REGION_[A-Za-z0-9_]+)\s*\)[^\n]*\n\s*return\s+"([^"]+)"`)
	// TG style: #define REGION_ACCESS_SECURITY list(ACCESS_...)
	reRegionDefine = regexp.MustCompile(`(?m)^[ \t]*#define[ \t]+REGION_ACCESS_([A-Za-z0-9_]+)[ \t]+list\(([^)]*)\)`)
	reIdent        = regexp.MustCompile(`ACCESS_[A-Za-z0-9_]+`)

	// Baystation: /datum/access/x { id = ACCESS_X; desc = "..."; region = ACCESS_REGION_Y }.
	reAccessDatum = regexp.MustCompile(`(?m)^/datum/access/[\w/]+\s*$`)
	reDatumId     = regexp.MustCompile(`(?m)^\s+id\s*=\s*(ACCESS_[A-Za-z0-9_]+)`)
	reDatumDesc   = regexp.MustCompile(`(?m)^\s+desc\s*=\s*"([^"]+)"`)
	reDatumRegion = regexp.MustCompile(`(?m)^\s+region\s*=\s*(ACCESS_REGION_[A-Za-z0-9_]+)`)
	reDatumType   = regexp.MustCompile(`(?m)^\s+access_type\s*=\s*(ACCESS_TYPE_[A-Za-z0-9_]+)`)
	// Maps of the code base included into the .dme: only these are looked through.
	reDmeMapDir = regexp.MustCompile(`(?im)^[ \t]*#include[ \t]+"maps[\\/]([^\\/"]+)[\\/]`)
)

// Russian names of departments. Unknown departments are shown as they are in the code.
var regionNamesRu = map[string]string{
	"general":     "Общие",
	"security":    "Служба безопасности",
	"medbay":      "Медицинский отдел",
	"medical":     "Медицинский отдел",
	"research":    "Научный отдел",
	"science":     "Научный отдел",
	"engineering": "Инженерный отдел",
	"supply":      "Снабжение",
	"command":     "Командование",
	"centcomm":    "Центральное командование",
	"centcom":     "Центральное командование",
	"service":     "Сервис",
	"containment": "Содержание объектов",
	"offsite":     "Вне комплекса",
	"admin":       "Администрация",
	"none":        "Без отдела",
}

func regionName(name string) string {
	if ru, ok := regionNamesRu[strings.ToLower(strings.TrimSpace(name))]; ok {
		return ru
	}
	return name
}

var skipDirs = map[string]bool{"node_modules": true, "cache": true, "target": true, "tgui": true}

// Load scans the code base for access declarations. It's safe to call it in the background.
func Load(rootDir string) *DB {
	start := time.Now()

	unusedMaps := unusedMapDirs(rootDir)

	var sources []string
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != rootDir && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()] || unusedMaps[path]) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".dm") && strings.Contains(strings.ToLower(d.Name()), "access") {
			if data, err := os.ReadFile(path); err == nil {
				sources = append(sources, string(data))
			}
		}
		return nil
	})

	db := parse(sources)
	byDefine := make(map[string]*Access, len(db.Accesses))
	for _, a := range db.Accesses {
		byDefine[a.Define] = a
	}
	db.Jobs = loadJobs(rootDir, byDefine)
	log.Printf("access db loaded: [%d] accesses, [%d] regions, [%d] jobs in [%d] ms",
		len(db.Accesses), len(db.Regions), len(db.Jobs), time.Since(start).Milliseconds())
	return db
}

func parse(sources []string) *DB {
	db := &DB{
		byValue:        make(map[string]*Access),
		regionsByValue: make(map[string][]*Region),
	}
	byDefine := make(map[string]*Access)

	for _, src := range sources {
		for _, m := range reDefine.FindAllStringSubmatch(src, -1) {
			if _, ok := byDefine[m[1]]; ok {
				continue
			}
			// Bay declares regions and access types with the same prefix, they are not accesses.
			if strings.HasPrefix(m[1], "ACCESS_REGION_") || strings.HasPrefix(m[1], "ACCESS_TYPE_") {
				continue
			}
			a := &Access{Define: m[1], Value: m[2], Desc: strings.TrimSpace(m[3])}
			byDefine[a.Define] = a
			db.Accesses = append(db.Accesses, a)
		}
	}

	// Descriptions from get_access_desc and similar procs have a priority over the define comment.
	descSet := make(map[string]bool)
	for _, src := range sources {
		for _, m := range reDesc.FindAllStringSubmatch(src, -1) {
			if a, ok := byDefine[m[1]]; ok && !descSet[a.Define] {
				a.Desc = m[2]
				descSet[a.Define] = true
			}
		}
	}

	regionNames := make(map[string]string)
	for _, src := range sources {
		for _, block := range procBlocks(src, "get_region_accesses_name") {
			for _, m := range reRegionName.FindAllStringSubmatch(block, -1) {
				regionNames[m[1]] = m[2]
			}
		}
	}

	// Baystation datums give descriptions and departments of accesses.
	datumRegions := make(map[string][]string)
	for _, src := range sources {
		starts := reAccessDatum.FindAllStringIndex(src, -1)
		for i, s := range starts {
			end := len(src)
			if i+1 < len(starts) {
				end = starts[i+1][0]
			}
			block := src[s[0]:end]
			idm := reDatumId.FindStringSubmatch(block)
			if idm == nil {
				continue
			}
			a, ok := byDefine[idm[1]]
			if !ok {
				continue
			}
			if d := reDatumDesc.FindStringSubmatch(block); d != nil && !descSet[a.Define] {
				a.Desc = d[1]
				descSet[a.Define] = true
			}
			if r := reDatumRegion.FindStringSubmatch(block); r != nil {
				datumRegions[r[1]] = append(datumRegions[r[1]], a.Define)
			} else if t := reDatumType.FindStringSubmatch(block); t != nil && t[1] == "ACCESS_TYPE_CONTAINMENT" {
				// Foundation-19 containment accesses have only a type.
				datumRegions["ACCESS_REGION_CONTAINMENT"] = append(datumRegions["ACCESS_REGION_CONTAINMENT"], a.Define)
			}
		}
	}

	addRegion := func(define, name, list string) {
		r := &Region{Define: define, Name: regionName(name)}
		for _, ident := range reIdent.FindAllString(list, -1) {
			if a, ok := byDefine[ident]; ok {
				r.Accesses = append(r.Accesses, a)
			}
		}
		if len(r.Accesses) > 0 {
			db.Regions = append(db.Regions, r)
		}
	}

	for _, src := range sources {
		for _, block := range procBlocks(src, "get_region_accesses") {
			for _, m := range reRegionList.FindAllStringSubmatch(block, -1) {
				name := regionNames[m[1]]
				if name == "" {
					name = strings.TrimPrefix(m[1], "REGION_")
				}
				addRegion(m[1], name, m[2])
			}
		}
		for _, m := range reRegionDefine.FindAllStringSubmatch(src, -1) {
			addRegion("REGION_ACCESS_"+m[1], m[1], m[2])
		}
	}

	regionDefines := make([]string, 0, len(datumRegions))
	for region := range datumRegions {
		regionDefines = append(regionDefines, region)
	}
	sort.Strings(regionDefines)
	for _, region := range regionDefines {
		name := regionNames[region]
		if name == "" {
			name = strings.TrimPrefix(region, "ACCESS_REGION_")
		}
		addRegion(region, name, strings.Join(datumRegions[region], ","))
	}

	// Accesses without a department are collected into a separate group.
	inRegion := make(map[*Access]bool)
	for _, r := range db.Regions {
		for _, a := range r.Accesses {
			inRegion[a] = true
		}
	}
	other := &Region{Define: "", Name: "Прочие"}
	for _, a := range db.Accesses {
		if !inRegion[a] {
			other.Accesses = append(other.Accesses, a)
		}
	}
	sortAccesses(other.Accesses)
	if len(other.Accesses) > 0 {
		db.Regions = append(db.Regions, other)
	}

	for _, a := range db.Accesses {
		db.byValue[a.Value] = a
	}
	db.byDefine = byDefine
	for _, r := range db.Regions {
		if r == other {
			continue
		}
		for _, a := range r.Accesses {
			db.regionsByValue[a.Value] = append(db.regionsByValue[a.Value], r)
		}
	}

	return db
}

// unusedMapDirs returns folders of maps which are not included into the .dme of the code base.
// These are old maps with their own accesses and jobs (Torch etc.), they would only make a mess in the list.
func unusedMapDirs(rootDir string) map[string]bool {
	dmes, _ := filepath.Glob(filepath.Join(rootDir, "*.dme"))
	used := make(map[string]bool)
	for _, dme := range dmes {
		data, err := os.ReadFile(dme)
		if err != nil {
			continue
		}
		for _, m := range reDmeMapDir.FindAllSubmatch(data, -1) {
			used[strings.ToLower(string(m[1]))] = true
		}
	}
	if len(used) == 0 {
		return nil
	}
	res := make(map[string]bool)
	entries, _ := os.ReadDir(filepath.Join(rootDir, "maps"))
	for _, e := range entries {
		if e.IsDir() && !used[strings.ToLower(e.Name())] {
			res[filepath.Join(rootDir, "maps", e.Name())] = true
		}
	}
	return res
}

// procBlocks returns bodies of all procs with the provided name.
func procBlocks(src, procName string) []string {
	var blocks []string
	marker := "proc/" + procName + "("
	for {
		idx := strings.Index(src, marker)
		if idx < 0 {
			return blocks
		}
		src = src[idx+len(marker):]
		// The proc body ends with the next top-level declaration.
		end := len(src)
		for _, sep := range []string{"\n/", "\n#"} {
			if e := strings.Index(src, sep); e >= 0 && e < end {
				end = e
			}
		}
		blocks = append(blocks, src[:end])
	}
}

func sortAccesses(accesses []*Access) {
	sort.SliceStable(accesses, func(i, j int) bool {
		ni, ei := strconv.Atoi(accesses[i].Value)
		nj, ej := strconv.Atoi(accesses[j].Value)
		if ei == nil && ej == nil {
			return ni < nj
		}
		return accesses[i].Value < accesses[j].Value
	})
}

// ParseList parses a map variable value, like "list(12,13)" or "list(\"security\")", into values.
func ParseList(value string) []string {
	value = strings.TrimSpace(value)
	// newlist() on the map is an empty list.
	if value == "" || value == "null" || strings.HasPrefix(value, "newlist(") {
		return nil
	}
	if strings.HasPrefix(value, "list(") && strings.HasSuffix(value, ")") {
		value = value[len("list(") : len(value)-1]
	}
	var res []string
	for _, part := range splitTopLevel(value) {
		if part = strings.TrimSpace(part); part != "" {
			// Maps could use defines: the compiler replaces them with values.
			if db := current.Load(); db != nil && strings.HasPrefix(part, "ACCESS_") {
				if a, ok := db.byDefine[part]; ok {
					part = a.Value
				}
			}
			res = append(res, part)
		}
	}
	return res
}

// FormatList formats values to the map variable value.
func FormatList(values []string) string {
	sorted := make([]string, len(values))
	copy(sorted, values)
	sort.SliceStable(sorted, func(i, j int) bool {
		ni, ei := strconv.Atoi(sorted[i])
		nj, ej := strconv.Atoi(sorted[j])
		if ei == nil && ej == nil {
			return ni < nj
		}
		return sorted[i] < sorted[j]
	})
	return "list(" + strings.Join(sorted, ",") + ")"
}

// splitTopLevel splits by commas, which are not inside quotes or brackets.
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	inQuote := false
	last := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' && (i == 0 || s[i-1] != '\\'):
			inQuote = !inQuote
		case inQuote:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[last:i])
			last = i + 1
		}
	}
	return append(parts, s[last:])
}

var current atomic.Pointer[DB]

// Current returns the access db of the loaded environment. Could be nil while it's loading.
func Current() *DB {
	return current.Load()
}

// SetCurrent sets the access db of the loaded environment.
func SetCurrent(db *DB) {
	current.Store(db)
}

// IsGroup returns true for a nested list element, which means "any of" in Baystation (req_access = list(list(a, b))).
func IsGroup(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "list(")
}

// Flatten returns all accesses of the list, including members of nested "any of" groups.
func Flatten(values []string) []string {
	var res []string
	for _, v := range values {
		if IsGroup(v) {
			res = append(res, ParseList(v)...)
		} else {
			res = append(res, v)
		}
	}
	return res
}
