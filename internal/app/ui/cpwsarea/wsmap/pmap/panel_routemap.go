package pmap

import (
	"fmt"
	"strings"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmroute"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/util"

	"github.com/SpaiR/imgui-go"
)

// routeView stores the computed route of the map pane.
type routeView struct {
	grid        *dmroute.Grid
	gridChanges int
	gridLevel   int

	version int
	res     *dmroute.Result
	path    []util.Point
	onPath  map[util.Point]bool
	closed  map[util.Point]bool

	filter string
}

// processRouteMap recomputes the route when the map or the settings are changed and sets the tint layer.
func (p *PaneMap) processRouteMap() {
	render := p.canvas.Render()
	if !tools.IsSelected(tools.TNRouteMap) {
		render.SetTileTint(nil)
		return
	}

	rv := &p.routeView
	s := tools.RouteMap

	// The grid depends only on the map, so it's rebuilt only after map changes.
	if rv.grid == nil || rv.gridChanges != p.editor.Changes() || rv.gridLevel != p.activeLevel || rv.grid.W != p.dmm.MaxX || rv.grid.H != p.dmm.MaxY {
		rv.grid = dmroute.Build(p.dmm, p.activeLevel)
		rv.gridChanges = p.editor.Changes()
		rv.gridLevel = p.activeLevel
		rv.version = -1
	}

	if rv.version != s.Version {
		rv.version = s.Version
		rv.res, rv.path = nil, nil
		rv.onPath = make(map[util.Point]bool)
		rv.closed = make(map[util.Point]bool)
		if s.Start != nil && s.Start.Z == p.activeLevel {
			rv.res = rv.grid.Reach(*s.Start, s.Accesses, s.AllowSpace)
			for _, c := range rv.res.ClosedDoors {
				rv.closed[util.Point{X: c.X, Y: c.Y}] = true
			}
			if s.End != nil && s.End.Z == p.activeLevel {
				rv.path = rv.grid.Path(rv.res, *s.End)
				for _, c := range rv.path {
					rv.onPath[util.Point{X: c.X, Y: c.Y}] = true
				}
			}
		}
	}

	if rv.res == nil {
		render.SetTileTint(nil)
		return
	}

	res, grid := rv.res, rv.grid
	start, end := s.Start, s.End
	render.SetTileTint(func(x, y int) (float32, float32, float32, float32, bool) {
		key := util.Point{X: x, Y: y}
		switch {
		case start != nil && start.X == x && start.Y == y:
			return .2, .5, 1, .7, true
		case end != nil && end.X == x && end.Y == y:
			return 1, .4, .9, .7, true
		case rv.onPath[key]:
			return 1, .85, .1, .55, true
		case rv.closed[key]:
			return 1, .15, .15, .55, true
		case grid.DistAt(res, x, y) >= 0:
			return .2, 1, .3, .18, true
		}
		return 0, 0, 0, 0, false
	})
}

func (p *PaneMap) showRouteMapPanel() {
	s := tools.RouteMap
	rv := &p.routeView
	db := dmaccess.Current()

	imgui.Dummy(imgui.Vec2{X: window.PointSize() * 300})
	imgui.Text("Маршруты и доступы")
	imgui.Separator()
	imgui.TextWrapped("Клик — откуда, Alt+клик — куда. Зелёным — куда можно дойти, красным — двери, которые не открыть, жёлтым — путь.")

	// Presets.
	imgui.SetNextItemWidth(-1)
	if imgui.BeginCombo("##route_job", fmt.Sprintf("Доступы: %d (выбрать должность)", len(s.Accesses))) {
		if imgui.Selectable("Без доступа (ассистент)") {
			s.Accesses = make(map[string]bool)
			s.Changed()
		}
		if db != nil {
			if imgui.Selectable("Все доступы станции") {
				s.Accesses = make(map[string]bool)
				for _, r := range db.Regions {
					if r.Define == "" {
						continue // Not a station department.
					}
					for _, a := range r.Accesses {
						s.Accesses[a.Value] = true
					}
				}
				s.Changed()
			}
			imgui.Separator()
			for _, job := range db.Jobs {
				if imgui.Selectable(job.Name + "##job_" + job.Name) {
					s.Accesses = make(map[string]bool)
					for _, a := range job.Accesses {
						s.Accesses[a] = true
					}
					s.Changed()
				}
			}
		}
		imgui.EndCombo()
	}

	if imgui.Checkbox("Можно через космос##route_space", &s.AllowSpace) {
		s.Changed()
	}

	// Result.
	imgui.Separator()
	switch {
	case s.Start == nil:
		imgui.TextDisabled("Кликните по карте, чтобы выбрать начальную точку")
	case rv.res == nil:
		imgui.TextDisabled("Начальная точка на другом Z-уровне")
	default:
		imgui.Text(fmt.Sprintf("Доступно тайлов: %d", rv.res.Reachable))
		if s.End != nil {
			if rv.path != nil {
				imgui.TextColored(style.ColorGold, fmt.Sprintf("Путь: %d шагов", len(rv.path)-1))
			} else {
				imgui.TextColored(style.ColorRedLighter, "Цель недостижима с этими доступами")
			}
		}
		p.showClosedDoors(db)
	}

	// Manual accesses selection.
	if db != nil && imgui.CollapsingHeader("Выбрать доступы вручную") {
		imgui.SetNextItemWidth(-1)
		imgui.InputTextWithHint("##route_access_filter", "Поиск доступа", &rv.filter)
		filter := strings.ToLower(rv.filter)
		imgui.BeginChildV("route_accesses", imgui.Vec2{Y: window.PointSize() * 220}, true, 0)
		for _, region := range db.Regions {
			if filter == "" && !imgui.TreeNode(region.Name+"##route_region_"+region.Name) {
				continue
			}
			for _, a := range region.Accesses {
				if filter != "" && !strings.Contains(strings.ToLower(a.Name()), filter) {
					continue
				}
				checked := s.Accesses[a.Value]
				if imgui.Checkbox(a.Name()+"##route_access_"+a.Value, &checked) {
					if checked {
						s.Accesses[a.Value] = true
					} else {
						delete(s.Accesses, a.Value)
					}
					s.Changed()
				}
			}
			if filter == "" {
				imgui.TreePop()
			}
		}
		imgui.EndChild()
	}
}

// showClosedDoors lists doors which stop the crew member, with accesses they need.
func (p *PaneMap) showClosedDoors(db *dmaccess.DB) {
	rv := &p.routeView
	if len(rv.res.ClosedDoors) == 0 {
		return
	}
	if !imgui.CollapsingHeader(fmt.Sprintf("Закрытые двери на границе: %d###route_closed", len(rv.res.ClosedDoors))) {
		return
	}
	for idx, c := range rv.res.ClosedDoors {
		if idx >= 50 {
			imgui.TextDisabled(fmt.Sprintf("...и ещё %d", len(rv.res.ClosedDoors)-idx))
			break
		}
		if imgui.SmallButton(fmt.Sprintf("%d,%d##route_door_%d", c.X, c.Y, idx)) {
			p.editor.FocusCameraOnPosition(c)
		}
		imgui.SameLine()
		door := rv.grid.DoorAt(c.X, c.Y)
		if door == nil {
			continue
		}
		imgui.TextWrapped("нужно: " + describeDoorAccess(db, door))
	}
}

// describeDoorAccess returns a human-readable access rule of the door.
func describeDoorAccess(db *dmaccess.DB, door *dmroute.Door) string {
	var parts []string
	join := " и "
	if door.CheckOneAccess {
		join = " или "
	}
	for _, v := range door.ReqAccess {
		if dmaccess.IsGroup(v) {
			var group []string
			for _, one := range dmaccess.ParseList(v) {
				group = append(group, accessName(db, one))
			}
			parts = append(parts, "("+strings.Join(group, " или ")+")")
		} else {
			parts = append(parts, accessName(db, v))
		}
	}
	res := strings.Join(parts, join)
	if len(door.ReqOneAccess) > 0 {
		var one []string
		for _, v := range door.ReqOneAccess {
			one = append(one, accessName(db, v))
		}
		if res != "" {
			res += " и "
		}
		res += "(" + strings.Join(one, " или ") + ")"
	}
	return res
}
