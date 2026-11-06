package dmlint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// This file is a port of the TG "maplint" tool (tools/maplint in the code base).
// Rules are read from the code base, so the editor reports exactly what the CI reports.

// MaplintRules is a set of lints loaded from the code base.
type MaplintRules struct {
	Lints  []*maplintLint
	Errors []string // Problems with rule files themselves.
}

type maplintLint struct {
	file  string
	help  string
	rules []maplintTypedRules
}

type maplintTypedRules struct {
	typepath typepathExtra
	rules    *maplintRules
}

type typepathExtra struct {
	path     string
	exact    bool
	wildcard bool
}

func newTypepathExtra(s string) typepathExtra {
	if s == "*" {
		return typepathExtra{wildcard: true}
	}
	if strings.HasPrefix(s, "=") {
		return typepathExtra{path: s[1:], exact: true}
	}
	return typepathExtra{path: s}
}

func (t typepathExtra) matches(path string) bool {
	switch {
	case t.wildcard:
		return true
	case t.exact:
		return t.path == path
	}
	return path == t.path || strings.HasPrefix(path, t.path+"/")
}

type atomNeighbor struct {
	typepath  *typepathExtra
	pattern   *regexp.Regexp
	identical bool
	name      string
}

func (n atomNeighbor) matches(identified, neighbor *dmminstance.Instance) bool {
	if n.identical {
		return identified.Prefab().Path() == neighbor.Prefab().Path() && sameVarEdits(identified.Prefab(), neighbor.Prefab())
	}
	if n.typepath != nil && n.typepath.matches(neighbor.Prefab().Path()) {
		return true
	}
	return n.pattern != nil && n.pattern.MatchString(neighbor.Prefab().Path())
}

type choices struct {
	values  []any // string or float64
	pattern *regexp.Regexp
}

func (c *choices) contains(v constant) bool {
	if c.pattern != nil {
		return c.pattern.MatchString(v.str())
	}
	for _, value := range c.values {
		switch value := value.(type) {
		case float64:
			if v.isNum && v.num == value {
				return true
			}
		case string:
			if !v.isNum && v.s == value {
				return true
			}
		}
	}
	return false
}

func (c *choices) String() string {
	if c.pattern != nil {
		return c.pattern.String()
	}
	var parts []string
	for _, v := range c.values {
		parts = append(parts, fmt.Sprint(v))
	}
	return strings.Join(parts, ", ")
}

type bannedVariable struct {
	name        string
	allow, deny *choices
}

type condition struct {
	all, isGroup bool
	children     []condition

	kind, varName, value string
	re                   *regexp.Regexp
}

var (
	reCondNotSet   = regexp.MustCompile(`^(.+) is not set$`)
	reCondSet      = regexp.MustCompile(`^(.+) is set$`)
	reCondNotEqual = regexp.MustCompile(`^(.+) is not '(.+)'$`)
	reCondEqual    = regexp.MustCompile(`^(.+) is '(.+)'$`)
	reCondLike     = regexp.MustCompile(`^(.+) like '(.+)'$`)
)

func (c condition) met(p *dmmprefab.Prefab) bool {
	if c.isGroup {
		for _, child := range c.children {
			m := child.met(p)
			if c.all && !m {
				return false
			}
			if !c.all && m {
				return true
			}
		}
		return c.all
	}

	value, set := varEdit(p, c.varName)
	switch c.kind {
	case "set":
		return set
	case "not_set":
		return !set
	case "equal":
		return set && strings.TrimSpace(value.str()) == strings.TrimSpace(c.value)
	case "not_equal":
		return !set || strings.TrimSpace(value.str()) != strings.TrimSpace(c.value)
	case "like":
		return set && c.re.MatchString(value.str())
	}
	return false
}

type maplintRules struct {
	banned            bool
	bannedNeighbors   []atomNeighbor
	requiredNeighbors []atomNeighbor
	ignored           []typepathExtra
	bannedAllVars     bool
	bannedVariables   []bannedVariable
	when              *condition
	skipFiles         []string
	skipPatterns      []*regexp.Regexp
}

// constant is a parsed value of the variable, like the maplint does it.
type constant struct {
	s     string
	num   float64
	isNum bool
}

func (c constant) str() string {
	if c.isNum {
		if c.num == float64(int64(c.num)) {
			return strconv.FormatInt(int64(c.num), 10)
		}
		return strconv.FormatFloat(c.num, 'f', -1, 64)
	}
	return c.s
}

func parseConstant(raw string) constant {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		if s, err := strconv.Unquote(raw); err == nil {
			return constant{s: s}
		}
		return constant{s: raw[1 : len(raw)-1]}
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return constant{num: n, isNum: true}
	}
	return constant{s: raw}
}

// varEdit returns the value of the variable edited on the map.
func varEdit(p *dmmprefab.Prefab, name string) (constant, bool) {
	for _, n := range p.Vars().Iterate() {
		if n == name {
			v, _ := p.Vars().Value(name)
			return parseConstant(v), true
		}
	}
	return constant{}, false
}

func sameVarEdits(a, b *dmmprefab.Prefab) bool {
	if a.Id() == b.Id() {
		return true
	}
	an, bn := a.Vars().Iterate(), b.Vars().Iterate()
	if len(an) != len(bn) {
		return false
	}
	for _, name := range an {
		av, _ := a.Vars().Value(name)
		bv, ok := varEdit(b, name)
		if !ok || parseConstant(av) != bv {
			return false
		}
	}
	return true
}

// LoadMaplint loads lints from the "tools/maplint/lints" folder of the code base.
// Returns nil if the code base doesn't use the maplint.
func LoadMaplint(rootDir string) *MaplintRules {
	dir := filepath.Join(rootDir, "tools", "maplint", "lints")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	rules := &MaplintRules{}
	var names []string
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); !e.IsDir() && (ext == ".yml" || ext == ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		lint, err := parseLint(name, data)
		if err != nil {
			rules.Errors = append(rules.Errors, fmt.Sprintf("%s: %v", name, err))
			log.Printf("unable to parse maplint rule [%s]: %v", name, err)
			continue
		}
		rules.Lints = append(rules.Lints, lint)
	}

	log.Printf("maplint rules loaded: [%d], errors: [%d]", len(rules.Lints), len(rules.Errors))
	return rules
}

func parseLint(file string, data []byte) (*maplintLint, error) {
	// PyYAML (used by the maplint) allows duplicated keys, the last one wins. yaml.v3 doesn't, so nodes are converted manually.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	root, _ := nodeToAny(&doc).(map[string]any)
	if root == nil {
		root = map[string]any{}
	}
	lint := &maplintLint{file: file}
	if help, ok := root["help"].(string); ok {
		lint.help = help
	}
	var keys []string
	for k := range root {
		if k != "help" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, typepath := range keys {
		body, _ := root[typepath].(map[string]any)
		r, err := parseRules(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", typepath, err)
		}
		lint.rules = append(lint.rules, maplintTypedRules{newTypepathExtra(typepath), r})
	}
	return lint, nil
}

// compilePython compiles a Python regular expression. Only the syntax common with Go is supported.
func compilePython(pattern string, anchored bool) (*regexp.Regexp, error) {
	if anchored && !strings.HasPrefix(pattern, "^") {
		pattern = "^(?:" + pattern + ")" // re.match() is anchored at the start.
	}
	return regexp.Compile(pattern)
}

func parseNeighbors(data any) ([]atomNeighbor, error) {
	var res []atomNeighbor
	add := func(key string, props map[string]any) error {
		n := atomNeighbor{name: key}
		if strings.ToUpper(key) != key {
			t := newTypepathExtra(key)
			n.typepath = &t
		}
		if v, ok := props["identical"].(bool); ok {
			n.identical = v
		}
		if p, ok := props["pattern"].(string); ok {
			re, err := compilePython(p, true)
			if err != nil {
				return err
			}
			n.pattern = re
			n.name = p
		}
		res = append(res, n)
		return nil
	}
	switch data := data.(type) {
	case []any:
		for _, item := range data {
			if s, ok := item.(string); ok {
				if err := add(s, nil); err != nil {
					return nil, err
				}
			}
		}
	case map[string]any:
		for key, props := range data {
			m, _ := props.(map[string]any)
			if err := add(key, m); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

func parseChoices(data any) (*choices, error) {
	switch data := data.(type) {
	case []any:
		c := &choices{}
		for _, item := range data {
			switch item := item.(type) {
			case int:
				c.values = append(c.values, float64(item))
			case float64:
				c.values = append(c.values, item)
			case string:
				c.values = append(c.values, item)
			}
		}
		return c, nil
	case map[string]any:
		if p, ok := data["pattern"].(string); ok {
			re, err := compilePython(p, true)
			if err != nil {
				return nil, err
			}
			return &choices{pattern: re}, nil
		}
	}
	return nil, nil
}

func parseCondition(data any) (condition, error) {
	switch data := data.(type) {
	case string:
		c := condition{}
		switch {
		case reCondNotSet.MatchString(data):
			c.kind, c.varName = "not_set", reCondNotSet.FindStringSubmatch(data)[1]
		case reCondSet.MatchString(data):
			c.kind, c.varName = "set", reCondSet.FindStringSubmatch(data)[1]
		case reCondNotEqual.MatchString(data):
			m := reCondNotEqual.FindStringSubmatch(data)
			c.kind, c.varName, c.value = "not_equal", m[1], m[2]
		case reCondEqual.MatchString(data):
			m := reCondEqual.FindStringSubmatch(data)
			c.kind, c.varName, c.value = "equal", m[1], m[2]
		case reCondLike.MatchString(data):
			m := reCondLike.FindStringSubmatch(data)
			re, err := compilePython(m[2], true)
			if err != nil {
				return c, err
			}
			c.kind, c.varName, c.re = "like", m[1], re
		default:
			return c, fmt.Errorf("unknown condition: %s", data)
		}
		return c, nil
	case map[string]any:
		for key, list := range data {
			items, _ := list.([]any)
			g := condition{isGroup: true, all: key == "all"}
			for _, item := range items {
				child, err := parseCondition(item)
				if err != nil {
					return g, err
				}
				g.children = append(g.children, child)
			}
			return g, nil
		}
	}
	return condition{}, fmt.Errorf("invalid condition")
}

func parseRules(data map[string]any) (*maplintRules, error) {
	r := &maplintRules{}
	var err error

	if v, ok := data["banned"].(bool); ok {
		r.banned = v
	}
	if r.bannedNeighbors, err = parseNeighbors(data["banned_neighbors"]); err != nil {
		return nil, err
	}
	if r.requiredNeighbors, err = parseNeighbors(data["required_neighbors"]); err != nil {
		return nil, err
	}
	if list, ok := data["ignore"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				r.ignored = append(r.ignored, newTypepathExtra(s))
			}
		}
	}

	switch vars := data["banned_variables"].(type) {
	case bool:
		r.bannedAllVars = vars
	case []any:
		for _, item := range vars {
			if s, ok := item.(string); ok {
				r.bannedVariables = append(r.bannedVariables, bannedVariable{name: s})
			}
		}
	case map[string]any:
		for name, props := range vars {
			bv := bannedVariable{name: name}
			if m, ok := props.(map[string]any); ok {
				if bv.allow, err = parseChoices(m["allow"]); err != nil {
					return nil, err
				}
				if bv.deny, err = parseChoices(m["deny"]); err != nil {
					return nil, err
				}
			}
			r.bannedVariables = append(r.bannedVariables, bv)
		}
	}

	if list, ok := data["when"].([]any); ok {
		g := condition{isGroup: true, all: true}
		for _, item := range list {
			c, err := parseCondition(item)
			if err != nil {
				return nil, err
			}
			g.children = append(g.children, c)
		}
		r.when = &g
	}

	if list, ok := data["skip_files"].([]any); ok {
		for _, item := range list {
			switch item := item.(type) {
			case string:
				r.skipFiles = append(r.skipFiles, item)
			case map[string]any:
				if p, ok := item["pattern"].(string); ok {
					re, err := compilePython(p, false)
					if err != nil {
						return nil, err
					}
					r.skipPatterns = append(r.skipPatterns, re)
				}
			}
		}
	}

	return r, nil
}

// run checks a single object on the tile. Returns error messages.
func (r *maplintRules) run(identified *dmminstance.Instance, contents []*dmminstance.Instance, mapFile string) []string {
	for _, s := range r.skipFiles {
		if strings.Contains(mapFile, s) {
			return nil
		}
	}
	for _, re := range r.skipPatterns {
		if re.MatchString(mapFile) {
			return nil
		}
	}
	if r.when != nil && !r.when.met(identified.Prefab()) {
		return nil
	}

	path := identified.Prefab().Path()
	var res []string

	if r.banned {
		res = append(res, fmt.Sprintf("тип %s запрещён", path))
	}

	others := make([]*dmminstance.Instance, 0, len(contents))
	for _, c := range contents {
		if c != identified {
			others = append(others, c)
		}
	}

	for _, n := range r.bannedNeighbors {
		ignored := false
		for _, other := range others {
			for _, ign := range r.ignored {
				if ign.matches(other.Prefab().Path()) {
					ignored = true
				}
			}
		}
		if ignored {
			continue
		}
		for _, other := range others {
			if n.matches(identified, other) {
				res = append(res, fmt.Sprintf("%s на одном тайле с запрещённым %s", path, other.Prefab().Path()))
			}
		}
	}

	for _, n := range r.requiredNeighbors {
		found := false
		for _, other := range others {
			if n.matches(identified, other) {
				found = true
				break
			}
		}
		if !found {
			res = append(res, fmt.Sprintf("у %s нет обязательного соседа %s", path, n.name))
		}
	}

	if r.bannedAllVars {
		if identified.Prefab().Vars().Len() > 0 {
			res = append(res, fmt.Sprintf("у %s не должно быть изменённых переменных", path))
		}
	} else {
		for _, bv := range r.bannedVariables {
			value, set := varEdit(identified.Prefab(), bv.name)
			if !set {
				continue
			}
			switch {
			case bv.allow != nil:
				if bv.allow.contains(value) {
					continue
				}
				res = append(res, fmt.Sprintf("%s.%s = %s: допустимо только %s", path, bv.name, value.str(), bv.allow))
			case bv.deny != nil:
				if !bv.deny.contains(value) {
					continue
				}
				res = append(res, fmt.Sprintf("%s.%s = %s: недопустимое значение", path, bv.name, value.str()))
			default:
				res = append(res, fmt.Sprintf("%s: переменную %s нельзя менять на карте", path, bv.name))
			}
		}
	}

	return res
}

// nodeToAny converts the YAML node to Go values. For duplicated keys the last value is used, like in PyYAML.
func nodeToAny(n *yaml.Node) any {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) > 0 {
			return nodeToAny(n.Content[0])
		}
		return nil
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = nodeToAny(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		list := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			list = append(list, nodeToAny(c))
		}
		return list
	case yaml.AliasNode:
		return nodeToAny(n.Alias)
	case yaml.ScalarNode:
		var v any
		if err := n.Decode(&v); err != nil {
			return n.Value
		}
		return v
	}
	return nil
}
