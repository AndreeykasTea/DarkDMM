package cpsearch

import (
	"fmt"
	"regexp"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/imguiext/style"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

// Operators of the variables search.
const (
	varOpEqual = iota
	varOpNotEqual
	varOpContains
	varOpRegex
	varOpSet
	varOpNotSet
)

var varOpNames = []string{"равно", "не равно", "содержит", "regex", "изменена на карте", "не изменена"}

// varSearch is a state of the search by variables.
type varSearch struct {
	typeFilter string
	name       string
	op         int32
	value      string
	onlyEdited bool

	newValue string

	active bool // The last search was made by variables.
	err    string
}

func (s *Search) showVarSearch() {
	vs := &s.vars
	if !imgui.CollapsingHeader("Поиск по переменным") {
		return
	}

	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##var_search_type", "Тип (с подтипами), например /obj/machinery/door", &vs.typeFilter)

	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * .4)
	imgui.InputTextWithHint("##var_search_name", "Переменная", &vs.name)
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * .4)
	if imgui.BeginCombo("##var_search_op", varOpNames[vs.op]) {
		for idx, name := range varOpNames {
			if imgui.SelectableV(name, int32(idx) == vs.op, 0, imgui.Vec2{}) {
				vs.op = int32(idx)
			}
		}
		imgui.EndCombo()
	}
	if vs.op != varOpSet && vs.op != varOpNotSet {
		imgui.SameLine()
		imgui.SetNextItemWidth(-1)
		imgui.InputTextWithHint("##var_search_value", "Значение", &vs.value)
	}
	imgui.Checkbox("Только изменённые на карте##var_search_only_edited", &vs.onlyEdited)
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Не учитывать значения, которые объект получает из своего типа в коде")
	}
	imgui.SameLine()
	if imgui.Button("Найти##var_search_do") {
		s.doVarSearch()
	}
	if vs.err != "" {
		imgui.TextColored(style.ColorRedLighter, vs.err)
	}

	if vs.active && len(s.resultsAll) > 0 {
		imgui.Separator()
		imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * .6)
		imgui.InputTextWithHint("##var_search_new_value", "Новое значение (пусто - как в коде)", &vs.newValue)
		imgui.SameLine()
		if imgui.Button(fmt.Sprintf("Задать у всех (%d)##var_search_set_all", len(s.results()))) {
			s.doVarSetAll()
		}
		if imgui.IsItemHovered() {
			imgui.SetTooltip("Строки пишутся в кавычках: \"текст\". Пустое значение сбрасывает переменную к значению из кода.")
		}
	}
}

// varMatcher returns a function which checks the instance by the current variables search.
func (s *Search) varMatcher() (func(*dmminstance.Instance) bool, error) {
	vs := &s.vars
	name := strings.TrimSpace(vs.name)
	typeFilter := strings.TrimSpace(vs.typeFilter)
	if name == "" && typeFilter == "" {
		return nil, fmt.Errorf("укажите тип или переменную")
	}

	var re *regexp.Regexp
	if vs.op == varOpRegex {
		var err error
		if re, err = regexp.Compile(vs.value); err != nil {
			return nil, fmt.Errorf("ошибка в regex: %v", err)
		}
	}
	want := strings.TrimSpace(vs.value)

	return func(i *dmminstance.Instance) bool {
		p := i.Prefab()
		if typeFilter != "" && !strings.HasPrefix(p.Path(), typeFilter) {
			return false
		}
		if name == "" {
			return true
		}

		edited := isEdited(p.Vars(), name)
		value, exists := p.Vars().Value(name)
		if vs.onlyEdited && !edited && vs.op != varOpNotSet {
			return false
		}

		switch vs.op {
		case varOpSet:
			return edited
		case varOpNotSet:
			return exists && !edited
		}
		if !exists {
			return false
		}

		switch vs.op {
		case varOpEqual:
			return valueEquals(value, want)
		case varOpNotEqual:
			return !valueEquals(value, want)
		case varOpContains:
			return strings.Contains(strings.ToLower(value), strings.ToLower(want))
		case varOpRegex:
			return re.MatchString(value)
		}
		return false
	}, nil
}

// valueEquals compares values, the quotes of strings are optional for the user.
func valueEquals(value, want string) bool {
	value = strings.TrimSpace(value)
	if value == want {
		return true
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1:len(value)-1] == want
	}
	return strings.ReplaceAll(value, " ", "") == strings.ReplaceAll(want, " ", "")
}

func isEdited(vars *dmvars.Variables, name string) bool {
	for _, n := range vars.Iterate() {
		if n == name {
			return true
		}
	}
	return false
}

func (s *Search) doVarSearch() {
	ed := s.app.CurrentEditor()
	if ed == nil {
		return
	}
	match, err := s.varMatcher()
	s.vars.err = ""
	if err != nil {
		s.vars.err = err.Error()
		return
	}

	s.Free()
	s.vars.active = true
	for _, tile := range ed.Dmm().Tiles {
		for _, i := range tile.Instances() {
			if match(i) {
				s.resultsAll = append(s.resultsAll, i)
			}
		}
	}
	log.Printf("variables search: type [%s], var [%s], found: [%d]", s.vars.typeFilter, s.vars.name, len(s.resultsAll))
}

// doVarSetAll sets the variable to all found instances.
func (s *Search) doVarSetAll() {
	ed := s.app.CurrentEditor()
	name := strings.TrimSpace(s.vars.name)
	if ed == nil || name == "" {
		s.vars.err = "укажите переменную"
		return
	}
	newValue := strings.TrimSpace(s.vars.newValue)

	cache := make(map[uint64]*dmmprefab.Prefab) // Old prefab ID -> new prefab, to not create the same prefab many times.
	changed := 0
	for _, i := range s.results() {
		p := i.Prefab()
		if newPrefab, ok := cache[p.Id()]; ok {
			if newPrefab.Id() != p.Id() {
				i.SetPrefab(newPrefab)
				changed++
			}
			continue
		}

		var vars *dmvars.Variables
		initial := ""
		if obj, ok := ed.Environment().Objects[p.Path()]; ok {
			initial = obj.Vars.ValueV(name, "")
		}
		if newValue == "" || newValue == initial {
			vars = dmvars.Delete(p.Vars(), name)
		} else {
			vars = dmvars.Set(p.Vars(), name, newValue)
		}
		newPrefab := dmmap.PrefabStorage.Get(p.Path(), vars)
		cache[p.Id()] = newPrefab
		if newPrefab.Id() != p.Id() {
			i.SetPrefab(newPrefab)
			changed++
		}
	}

	if changed > 0 {
		ed.CommitChanges(fmt.Sprintf("Переменная %s у %d объектов", name, changed))
	}
	log.Printf("variable [%s] set to [%s] for [%d] instances", name, newValue, changed)
	s.doVarSearch()
}

// resultDescription returns a short text for the search result in the variables mode.
func (s *Search) resultDescription(i *dmminstance.Instance) string {
	if !s.vars.active {
		return ""
	}
	desc := dm.PathLast(i.Prefab().Path())
	if name := strings.TrimSpace(s.vars.name); name != "" {
		if v, ok := i.Prefab().Vars().Value(name); ok {
			desc += " " + name + " = " + v
		}
	}
	return desc
}
