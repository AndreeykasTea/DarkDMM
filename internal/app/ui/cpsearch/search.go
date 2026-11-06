package cpsearch

import (
	"strconv"

	"sdmm/internal/app/ui/component"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/editor"

	"sdmm/internal/app/ui/shortcut"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

type App interface {
	CurrentEditor() *editor.Editor
	DoEditInstance(*dmminstance.Instance)
	ShowLayout(name string, focus bool)
}

type Search struct {
	component.Component

	app App

	shortcuts shortcut.Shortcuts

	prefabId string

	selectedResultIdx    int
	focusedResultIdx     int
	lastFocusedResultIdx int

	filterActive bool
	filterBound  util.Bounds

	resultsAll      []*dmminstance.Instance
	resultsFiltered []*dmminstance.Instance

	vars varSearch
}

func (s *Search) Init(app App) {
	s.app = app

	s.selectedResultIdx = -1
	s.focusedResultIdx = -1
	s.lastFocusedResultIdx = -1

	s.addShortcuts()

	s.AddOnFocused(func(focused bool) {
		s.shortcuts.SetVisible(focused)
	})
}

func (s *Search) Free() {
	s.resultsAll = s.resultsAll[:0]
	s.selectedResultIdx = -1
	s.focusedResultIdx = -1
	s.lastFocusedResultIdx = -1
	s.doResetFilter()
	log.Print("search free")
}

func (s *Search) Sync() {
	if s.vars.active {
		s.doVarSearch()
	} else {
		s.doSearch()
	}
}

func (s *Search) Search(prefabId uint64) {
	s.prefabId = strconv.FormatUint(prefabId, 10)
	s.doSearch()
}

func (s *Search) SearchByPath(path string) {
	s.prefabId = path
	s.doSearch()
}

func (s *Search) results() []*dmminstance.Instance {
	if !s.filterBound.IsEmpty() {
		return s.resultsFiltered
	}
	return s.resultsAll
}
