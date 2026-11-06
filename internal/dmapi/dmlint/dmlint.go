// Package dmlint checks the map for problems. By default it reports only what the CI of the code base reports:
// maplint rules (tools/maplint), text checks of the check_grep.py, non TGM format,
// and things which break the map loading (unknown variables, broken values).
// Additional heuristic checks (broken cables and pipes, APC) are optional, since the CI doesn't care about them.
package dmlint

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "Ошибка"
	case SeverityWarning:
		return "Предупреждение"
	}
	return "Заметка"
}

type Category string

const (
	CatMaplint    Category = "maplint (CI)"
	CatText       Category = "Текст: check_grep (CI)"
	CatFormat     Category = "Формат карты (CI)"
	CatGrep       Category = "Проверки CI (run-test.sh)"
	CatUnknownVar Category = "Неизвестные переменные"
	CatBadValue   Category = "Ошибки в значениях"
	CatDuplicate  Category = "Дубли объектов (доп.)"
	CatTurfArea   Category = "Турфы и зоны (доп.)"
	CatCable      Category = "Кабели (доп.)"
	CatPipe       Category = "Трубы (доп.)"
	CatDisposal   Category = "Мусоропровод (доп.)"
	CatAPC        Category = "APC (доп.)"
	CatSaveFormat Category = "Формат сохранения"

	CatTestCable    Category = "Тест карты: кабели"
	CatTestDisposal Category = "Тест карты: мусоропровод"
	CatTestAccess   Category = "Тест карты: доступы"
	CatTestDoor     Category = "Тест карты: двери"
	CatTestLadder   Category = "Тест карты: лестницы"
	CatTestCamera   Category = "Тест карты: камеры"
	CatTestAPC      Category = "Тест карты: APC"
	CatTestArea     Category = "Тест карты: зоны"
	CatTestCryo     Category = "Тест карты: криокапсулы"
	CatTestLandmark Category = "Тест карты: метки"
)

// Issue is a single found problem.
type Issue struct {
	Severity Severity
	Category Category
	Coord    util.Point
	Message  string
	Help     string

	// Fix fixes the problem by modifying the map. Nil if the problem can't be fixed automatically.
	Fix func() bool
}

// Options of the check.
type Options struct {
	// Maplint rules of the code base. Could be nil.
	Maplint *MaplintRules
	// MapFile is a path of the map relative to the code base root with forward slashes ("_maps/...").
	MapFile string
	// SavesAsTGM is true when the map will be saved in the TGM format.
	SavesAsTGM bool
	// RequireTGM enables the check of the save format (the CI rejects non TGM maps).
	RequireTGM bool
	// TextChecks enables check_grep.py text checks.
	TextChecks bool
	// Greps are grep checks of the CI applied to map lines.
	Greps []GrepRule
	// Extra enables heuristic checks, which are not done by the CI.
	Extra bool
	// UnitTests enables ports of map unit tests of Baystation code bases.
	UnitTests bool
	// RewritesFormat is true when the save will change the format of the map file, so every line of the file will change.
	RewritesFormat bool
}

// RewriteFormatMessage warns that the save will rewrite the whole file.
const RewriteFormatMessage = "формат сохранения отличается от формата файла: сохранение перепишет всю карту и даст огромный дифф в git (поставьте формат Initial в настройках)"

var rePathInValue = regexp.MustCompile(`(?:^|[\s(,=])(/[a-zA-Z_][\w/]*)`)

// Check checks the whole map.
func Check(env *dmenv.Dme, dmm *dmmap.Dmm, opts Options) []Issue {
	c := &checker{env: env, dmm: dmm, opts: opts, prefabIssues: make(map[uint64][]prefabIssue)}
	c.run()
	sort.SliceStable(c.issues, func(i, j int) bool {
		if c.issues[i].Severity != c.issues[j].Severity {
			return c.issues[i].Severity < c.issues[j].Severity
		}
		return c.issues[i].Category < c.issues[j].Category
	})
	return c.issues
}

type prefabIssue struct {
	severity Severity
	category Category
	message  string
	fixed    func(p *dmmprefab.Prefab) *dmmprefab.Prefab
}

type checker struct {
	env  *dmenv.Dme
	dmm  *dmmap.Dmm
	opts Options

	issues       []Issue
	prefabIssues map[uint64][]prefabIssue
	checked      map[uint64]bool
}

func (c *checker) add(issue Issue) {
	c.issues = append(c.issues, issue)
}

// IsTGMFile returns true if the map file is in the TGM format.
func IsTGMFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	return strings.Contains(string(buf[:n]), "//MAP CONVERTED BY dmm2tgm.py")
}

func (c *checker) run() {
	c.checked = make(map[uint64]bool)

	if c.opts.RequireTGM && !c.opts.SavesAsTGM {
		c.add(Issue{Severity: SeverityError, Category: CatFormat, Coord: util.Point{X: 1, Y: 1, Z: 1},
			Message: "карта будет сохранена не в TGM, CI это не пропустит (выберите формат TGM в настройках)"})
	}

	if c.opts.RewritesFormat {
		c.add(Issue{Severity: SeverityWarning, Category: CatSaveFormat, Coord: util.Point{X: 1, Y: 1, Z: 1},
			Message: RewriteFormatMessage})
	}

	apcs := make(map[string][]util.Point)
	areaTiles := make(map[string]util.Point)

	for _, tile := range c.dmm.Tiles {
		tile := tile
		contents := tile.Instances()
		var turfs, areas []*dmminstance.Instance
		seen := make(map[uint64]int)

		for _, inst := range contents {
			inst := inst
			p := inst.Prefab()
			path := p.Path()

			switch {
			case dm.IsPath(path, "/turf"):
				turfs = append(turfs, inst)
			case dm.IsPath(path, "/area"):
				areas = append(areas, inst)
				if _, ok := areaTiles[path]; !ok {
					areaTiles[path] = tile.Coord
				}
			}

			// Problems of the prefab itself are checked only once and reported for every instance.
			for _, pi := range c.prefabProblems(p) {
				pi := pi
				issue := Issue{Severity: pi.severity, Category: pi.category, Coord: tile.Coord, Message: pi.message}
				if pi.fixed != nil {
					issue.Fix = func() bool {
						newPrefab := pi.fixed(inst.Prefab())
						if newPrefab == nil || newPrefab.Id() == inst.Prefab().Id() {
							return false
						}
						inst.SetPrefab(newPrefab)
						return true
					}
				}
				c.add(issue)
			}

			c.runMaplint(tile, inst, contents)

			if c.opts.Extra && c.opts.Maplint == nil && isDuplicateSensitive(path) {
				seen[p.Id()]++
				if seen[p.Id()] == 2 {
					c.add(Issue{
						Severity: SeverityWarning,
						Category: CatDuplicate,
						Coord:    tile.Coord,
						Message:  fmt.Sprintf("два одинаковых объекта на тайле: %s", path),
						Fix:      func() bool { return removeDuplicates(tile, p.Id()) },
					})
				}
			}

			if dm.IsPath(path, "/obj/machinery/power/apc") {
				if area := areaOf(tile); area != "" {
					apcs[area] = append(apcs[area], tile.Coord)
				}
			}
		}

		// The maplint has its own rules for stacked turfs and areas.
		if c.opts.Extra && c.opts.Maplint == nil {
			if len(turfs) > 1 {
				c.add(Issue{Severity: SeverityError, Category: CatTurfArea, Coord: tile.Coord,
					Message: fmt.Sprintf("на тайле %d турфа, игра оставит только один", len(turfs)),
					Fix:     func() bool { return keepLast(tile, "/turf") }})
			}
			if len(areas) > 1 {
				c.add(Issue{Severity: SeverityError, Category: CatTurfArea, Coord: tile.Coord,
					Message: fmt.Sprintf("на тайле %d зоны", len(areas)),
					Fix:     func() bool { return keepLast(tile, "/area") }})
			}
		}

		if c.opts.Extra {
			c.checkNetworks(tile)
		}
	}

	if c.opts.UnitTests {
		c.runUnitTests()
	}

	if !c.opts.Extra {
		return
	}
	for area, coords := range apcs {
		if len(coords) > 1 {
			for _, coord := range coords {
				c.add(Issue{Severity: SeverityWarning, Category: CatAPC, Coord: coord,
					Message: fmt.Sprintf("в зоне %s несколько APC (%d)", area, len(coords))})
			}
		}
	}
	for area, coord := range areaTiles {
		if _, ok := apcs[area]; ok || !needsAPC(c.env, area) {
			continue
		}
		c.add(Issue{Severity: SeverityInfo, Category: CatAPC, Coord: coord,
			Message: fmt.Sprintf("в зоне %s нет APC", area)})
	}
}

func (c *checker) runMaplint(tile *dmmap.Tile, inst *dmminstance.Instance, contents dmmap.Instances) {
	if c.opts.Maplint == nil {
		return
	}
	for _, lint := range c.opts.Maplint.Lints {
		for _, tr := range lint.rules {
			if !tr.typepath.matches(inst.Prefab().Path()) {
				continue
			}
			for _, msg := range tr.rules.run(inst, contents, c.opts.MapFile) {
				c.add(Issue{Severity: SeverityError, Category: CatMaplint, Coord: tile.Coord,
					Message: msg + " [" + lint.file + "]", Help: lint.help})
			}
		}
	}
}

func (c *checker) prefabProblems(p *dmmprefab.Prefab) []prefabIssue {
	if c.checked[p.Id()] {
		return c.prefabIssues[p.Id()]
	}
	c.checked[p.Id()] = true

	var res []prefabIssue
	obj := c.env.Objects[p.Path()]

	for _, name := range p.Vars().Iterate() {
		name := name
		value, _ := p.Vars().Value(name)

		// Unknown variables cause runtime errors during the map loading.
		if obj != nil {
			if _, ok := obj.Vars.Value(name); !ok {
				res = append(res, prefabIssue{SeverityError, CatUnknownVar,
					fmt.Sprintf("у типа %s нет переменной %s", p.Path(), name),
					func(p *dmmprefab.Prefab) *dmmprefab.Prefab {
						return dmmap.PrefabStorage.Get(p.Path(), dmvars.Delete(p.Vars(), name))
					}})
				continue
			}
		}

		if c.opts.TextChecks {
			res = append(res, c.textIssues(name, value, p)...)
		}

		// The CI greps the map file, every variable is a separate line "\tname = value" in TGM.
		line := "\t" + name + " = " + value
		for _, g := range c.opts.Greps {
			if g.Pattern.MatchString(line) {
				res = append(res, prefabIssue{SeverityError, CatGrep,
					fmt.Sprintf("%s.%s: %s", p.Path(), name, g.Name),
					func(p *dmmprefab.Prefab) *dmmprefab.Prefab {
						return dmmap.PrefabStorage.Get(p.Path(), dmvars.Delete(p.Vars(), name))
					}})
			}
		}

		if msg := badValue(c.env, value); msg != "" {
			// A missing type becomes null with a runtime error, broken syntax breaks the loading of the map.
			severity := SeverityError
			if strings.HasPrefix(msg, "несуществующий тип") {
				severity = SeverityWarning
			}
			res = append(res, prefabIssue{severity, CatBadValue,
				fmt.Sprintf("%s.%s: %s", p.Path(), name, msg), nil})
		}
	}

	c.prefabIssues[p.Id()] = res
	return res
}

// ---------------------------------------------------------------------------------------------------------------------
// Text checks, a port of the tools/ci/check_grep.py checks which are applied to maps.

type textIssue struct {
	message string
	fix     func(string) string
}

var (
	reNanotrasenCamelEn = regexp.MustCompile(`NanoTrasen`)
	reNanotrasenCamelRu = regexp.MustCompile(`НаноТрейзен`)
	reNanotrasenLowerRu = regexp.MustCompile(`нанотрейзен`)
	reHTMLUppercase     = regexp.MustCompile(`</?[A-Z][A-Z0-9]*\b[^>]*/?>`)
	reHTMLTagName       = regexp.MustCompile(`^(</?)([A-Z][A-Z0-9]*)`)
)

const (
	longDash = '—'
	enDash   = '–'
)

func isCyrillic(r rune) bool {
	r = unicode.ToLower(r)
	return (r >= 'а' && r <= 'я') || r == 'ё'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// badDashes returns indexes (in runes) of " - " and " – " which must be replaced with the long dash.
// The same as HYPHEN_USAGE_RE and EN_DASH_USAGE_RE of the check_grep.py.
func badDashes(value string) (hyphens, enDashes []int) {
	runes := []rune(value)
	for i := 2; i+2 < len(runes); i++ {
		if runes[i-1] != ' ' || runes[i+1] != ' ' || (runes[i] != '-' && runes[i] != enDash) {
			continue
		}
		prev, next := runes[i-2], runes[i+2]
		if (isCyrillic(prev) && (isCyrillic(next) || isDigit(next))) || (isDigit(prev) && isCyrillic(next)) {
			if runes[i] == '-' {
				hyphens = append(hyphens, i)
			} else {
				enDashes = append(enDashes, i)
			}
		}
	}
	return
}

// FixDashes replaces hyphens and en dashes between russian words with the long dash, like the CI wants.
func FixDashes(value string) string {
	hyphens, enDashes := badDashes(value)
	runes := []rune(value)
	for _, i := range append(hyphens, enDashes...) {
		runes[i] = longDash
	}
	return string(runes)
}

func textProblems(value string) []textIssue {
	var res []textIssue

	if hyphens, enDashes := badDashes(value); len(hyphens) > 0 || len(enDashes) > 0 {
		msg := "дефис вместо длинного тире (—) в русском тексте"
		if len(hyphens) == 0 {
			msg = "короткое тире (–) вместо длинного (—) в русском тексте"
		}
		res = append(res, textIssue{msg, FixDashes})
	}

	if reNanotrasenCamelEn.MatchString(value) {
		res = append(res, textIssue{"\"NanoTrasen\" надо писать как \"Nanotrasen\"", func(v string) string {
			return reNanotrasenCamelEn.ReplaceAllString(v, "Nanotrasen")
		}})
	}
	if reNanotrasenCamelRu.MatchString(value) {
		res = append(res, textIssue{"\"НаноТрейзен\" надо писать как \"Нанотрейзен\"", func(v string) string {
			return reNanotrasenCamelRu.ReplaceAllString(v, "Нанотрейзен")
		}})
	}
	if reNanotrasenLowerRu.MatchString(value) && !strings.Contains(value, "UNLINT") {
		res = append(res, textIssue{"\"нанотрейзен\" надо писать с большой буквы", func(v string) string {
			return reNanotrasenLowerRu.ReplaceAllString(v, "Нанотрейзен")
		}})
	}
	if msg := nanotrasenQuotes(value); msg != "" {
		res = append(res, textIssue{msg, nil})
	}

	if m := reHTMLUppercase.FindString(value); m != "" {
		res = append(res, textIssue{fmt.Sprintf("HTML тег %s должен быть строчными буквами", m), func(v string) string {
			return reHTMLUppercase.ReplaceAllStringFunc(v, func(tag string) string {
				return reHTMLTagName.ReplaceAllStringFunc(tag, strings.ToLower)
			})
		}})
	}

	return res
}

// nanotrasenQuotes is a port of the NANOTRASEN_QUOTES_RU check: the word must be in escaped quotes.
func nanotrasenQuotes(value string) string {
	const msg = "\"Нанотрейзен\" должно быть в экранированных кавычках \\\"Нанотрейзен\\\""
	word := []rune("Нанотрейзен")
	runes := []rune(value)
	for i := 2; i+len(word)+2 <= len(runes); i++ {
		if string(runes[i:i+len(word)]) != string(word) {
			continue
		}
		before := string(runes[i-2 : i])
		after := string(runes[i+len(word) : i+len(word)+2])
		if before != `\"` && after != `\"` {
			return msg
		}
		if runes[i-1] != '"' && runes[i+len(word)] != '"' {
			return msg
		}
	}
	return ""
}

// badValue checks the syntax of the value: quotes and brackets must be balanced, types must exist.
func badValue(env *dmenv.Dme, value string) string {
	depth := 0
	inQuote := false
	var outside strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch == '\\' && inQuote:
			i++ // Skip the escaped character.
		case ch == '"':
			inQuote = !inQuote
		case inQuote:
		case ch == '(':
			depth++
		case ch == ')':
			depth--
			if depth < 0 {
				return "лишняя закрывающая скобка"
			}
		}
		if !inQuote && ch != '"' {
			outside.WriteByte(ch)
		}
	}
	if inQuote {
		return "незакрытая кавычка"
	}
	if depth != 0 {
		return "незакрытая скобка"
	}

	// Type paths outside of strings must exist, otherwise the map can't be loaded.
	for _, m := range rePathInValue.FindAllStringSubmatch(outside.String(), -1) {
		path := strings.TrimRight(m[1], "/")
		if _, ok := env.Objects[path]; !ok && !isBuiltinPath(path) {
			return "несуществующий тип " + path
		}
	}
	return ""
}

func isBuiltinPath(path string) bool {
	switch path {
	case "/datum", "/atom", "/atom/movable", "/obj", "/mob", "/turf", "/area", "/image", "/icon", "/sound", "/list", "/savefile", "/client", "/world", "/matrix", "/regex", "/mutable_appearance":
		return true
	}
	return false
}

// isDuplicateSensitive returns true for objects which must not be stacked on the same tile.
func isDuplicateSensitive(path string) bool {
	for _, p := range []string{"/obj/machinery", "/obj/structure"} {
		if dm.IsPath(path, p) {
			return true
		}
	}
	return false
}

func removeDuplicates(tile *dmmap.Tile, prefabId uint64) bool {
	found := false
	removed := false
	for _, inst := range tile.Instances() {
		if inst.Prefab().Id() != prefabId {
			continue
		}
		if found {
			tile.InstancesRemoveByInstance(inst)
			removed = true
		}
		found = true
	}
	return removed
}

// keepLast removes all instances of the path type except the last one (the one which the game will use).
func keepLast(tile *dmmap.Tile, path string) bool {
	var list []*dmminstance.Instance
	for _, inst := range tile.Instances() {
		if dm.IsPath(inst.Prefab().Path(), path) {
			list = append(list, inst)
		}
	}
	if len(list) < 2 {
		return false
	}
	for _, inst := range list[:len(list)-1] {
		tile.InstancesRemoveByInstance(inst)
	}
	return true
}

func areaOf(tile *dmmap.Tile) string {
	for _, inst := range tile.Instances() {
		if dm.IsPath(inst.Prefab().Path(), "/area") {
			return inst.Prefab().Path()
		}
	}
	return ""
}

func needsAPC(env *dmenv.Dme, area string) bool {
	if !dm.IsPath(area, "/area/station") {
		return false
	}
	obj, ok := env.Objects[area]
	if !ok {
		return false
	}
	return obj.Vars.ValueV("requires_power", "1") != "0" && obj.Vars.ValueV("requires_power", "1") != "FALSE"
}

// textIssues returns check_grep.py text problems of the variable.
func (c *checker) textIssues(name, value string, p *dmmprefab.Prefab) []prefabIssue {
	var res []prefabIssue
	for _, ti := range textProblems(value) {
		ti := ti
		issue := prefabIssue{SeverityError, CatText, fmt.Sprintf("%s.%s: %s", p.Path(), name, ti.message), nil}
		if ti.fix != nil {
			issue.fixed = func(p *dmmprefab.Prefab) *dmmprefab.Prefab {
				v, _ := p.Vars().Value(name)
				return dmmap.PrefabStorage.Get(p.Path(), dmvars.Set(p.Vars(), name, ti.fix(v)))
			}
		}
		res = append(res, issue)
	}
	return res
}
