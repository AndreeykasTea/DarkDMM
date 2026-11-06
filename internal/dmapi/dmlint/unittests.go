package dmlint

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// Ports of map unit tests of Baystation code bases (code/unit_tests/map_tests.dm and area_tests.dm).
// They are run on a compiled map, here they are checked right in the editor.
// Tests which depend on the runtime state are not ported: closets capacity, spawn points, air and shuttles.
//
// All of them are warnings: a code base could not run these tests at all (the Foundation-19 CI doesn't),
// so maps have old problems, and the editor must not annoy with them on every save.

const (
	dirUp   = 16
	dirDown = 32

	noAPCFlag = 1 // NO_APC

	testCablePath    = "/obj/structure/cable"
	testSegmentPath  = "/obj/structure/disposalpipe/segment"
	testDisposalPath = "/obj/structure/disposalpipe"
)

var (
	reLevelNumber = regexp.MustCompile(`\d+`)
	rePathValue   = regexp.MustCompile(`(/[\w/]+)\s*=\s*(\d+)`)
	rePathItem    = regexp.MustCompile(`/[\w/]+`)
)

// mapTestEnv is the data for tests, which is collected once per a check.
type mapTestEnv struct {
	datum *dmvars.Variables

	// hasAbove is z_levels of the game: the level has a connected level above it.
	hasAbove map[int]bool

	stationLevels map[int]bool
	playerLevels  map[int]bool

	accessIDs map[string]bool

	areas map[uint64]*testAreaData
}

func (c *checker) runUnitTests() {
	t := &mapTestEnv{hasAbove: make(map[int]bool)}

	name := strings.TrimSuffix(path.Base(c.opts.MapFile), ".dmm")
	// Tests about a whole station have no sense for parts of maps and event maps: they have no map datum.
	if obj, ok := c.env.Objects["/datum/map/"+name]; ok {
		t.datum = obj.Vars
		t.stationLevels = parseLevels(obj.Vars.ValueV("station_levels", ""))
		t.playerLevels = parseLevels(obj.Vars.ValueV("player_levels", ""))
	}

	t.accessIDs = make(map[string]bool)
	for p, obj := range c.env.Objects {
		if dm.IsPath(p, "/datum/access") {
			if id, ok := obj.Vars.Value("id"); ok && id != dmvars.NullValue {
				t.accessIDs[id] = true
			}
		}
	}

	for _, tile := range c.dmm.Tiles {
		for _, inst := range tile.Instances() {
			if dm.IsPath(inst.Prefab().Path(), "/obj/effect/landmark/map_data") {
				height := inst.Prefab().Vars().IntV("height", 1)
				for z := tile.Coord.Z - height + 1; z < tile.Coord.Z; z++ {
					t.hasAbove[z] = true
				}
			}
		}
	}

	t.areas = c.collectAreas()

	c.testTiles(t)
	if t.datum != nil {
		c.testAPCs(t)
		c.testAreasCoherent(t)
		c.testLandmarks(t)
		c.testCryopods(t)
		c.testLadders(t)
	}
	c.testAreasPure(t)
	c.testCameras(t)
}

func parseLevels(value string) map[int]bool {
	res := make(map[int]bool)
	for _, s := range reLevelNumber.FindAllString(value, -1) {
		n, _ := strconv.Atoi(s)
		res[n] = true
	}
	return res
}

func (c *checker) addTest(t *mapTestEnv, category Category, coord util.Point, message string, fix func() bool) {
	c.add(Issue{Severity: SeverityWarning, Category: category, Coord: coord, Message: message, Fix: fix})
}

func dirOf(p *dmmprefab.Prefab) int {
	return dm.FixDir(p.Vars().IntV("dir", dm.DirSouth))
}

func (c *checker) tileAt(coord util.Point) *dmmap.Tile {
	if !c.dmm.HasTile(coord) {
		return nil
	}
	return c.dmm.GetTile(coord)
}

func (c *checker) step(coord util.Point, dir int) *dmmap.Tile {
	dx, dy := dm.DirOffset(dir)
	return c.tileAt(util.Point{X: coord.X + dx, Y: coord.Y + dy, Z: coord.Z})
}

// testTiles runs tests of single objects.
func (c *checker) testTiles(t *mapTestEnv) {
	for _, tile := range c.dmm.Tiles {
		tile := tile
		var turf string
		instances := tile.Instances()
		for _, inst := range instances {
			if p := inst.Prefab().Path(); dm.IsPath(p, "/turf") {
				turf = p
			}
		}

		seenCables := make(map[string]bool)
		breakerbox := false
		for _, inst := range instances {
			if dm.IsPath(inst.Prefab().Path(), "/obj/machinery/power/breakerbox/activated") {
				breakerbox = true
			}
		}

		for _, inst := range instances {
			inst := inst
			p := inst.Prefab()
			objPath := p.Path()

			switch {
			case dm.IsPath(objPath, testCablePath):
				// The cable reads d1 and d2 from the icon state in its New, map variables mean nothing.
				state := p.Vars().TextV("icon_state", "")
				d1, d2, ok := cableEnds(state)
				if !ok {
					c.addTest(t, CatTestCable, tile.Coord, fmt.Sprintf("у кабеля неверный icon_state \"%s\"", state), nil)
					continue
				}

				// MAP: Cable Dir And Icon State Test. The icon has states only with a sorted order,
				// except vertical cables: they are "16-0" and "32-0".
				if d1 > d2 && d1 != dirUp && d1 != dirDown {
					fixed := fmt.Sprintf("%d-%d", d2, d1)
					c.addTest(t, CatTestCable, tile.Coord, fmt.Sprintf("у кабеля icon_state \"%s\", такого состояния нет в иконке (нужно \"%s\")", state, fixed), func() bool {
						cur := inst.Prefab()
						inst.SetPrefab(dmmap.PrefabStorage.Get(cur.Path(), dmvars.Set(cur.Vars(), "icon_state", strconv.Quote(fixed))))
						return true
					})
				}

				// MAP: Cable Overlap Test.
				if seenCables[state] && !breakerbox {
					c.addTest(t, CatTestCable, tile.Coord, fmt.Sprintf("два кабеля %s лежат друг на друге", state), func() bool {
						tile.InstancesRemoveByInstance(inst)
						return true
					})
				}
				seenCables[state] = true

				// MAP: Station wires shall be connected.
				if t.stationLevels[tile.Coord.Z] && d1 != dirUp && d2 != dirUp && d1 != dirDown && d2 != dirDown {
					for _, d := range []int{d1, d2} {
						if d != 0 && !c.cableReaches(tile.Coord, d) {
							c.addTest(t, CatTestCable, tile.Coord, fmt.Sprintf("кабель %s не подключён в сторону %s", state, dm.DirName(d)), nil)
						}
					}
				}

			case dm.IsPath(objPath, testDisposalPath):
				if dm.IsPath(objPath, testSegmentPath) {
					c.testDisposalSegment(t, tile, inst)
				}

			case dm.IsPath(objPath, "/obj/machinery/power/terminal"):
				// MAP: Station power terminals shall be wired.
				if t.stationLevels[tile.Coord.Z] && !hasKnot(instances) {
					c.addTest(t, CatTestCable, tile.Coord, "к терминалу не подведён кабель (нужен узел 0-N на этом тайле)", nil)
				}

			case dm.IsPath(objPath, "/obj/machinery/door"):
				// MAP: Doors shall be on appropriate turfs.
				if badDoorTurf(turf) {
					c.addTest(t, CatTestDoor, tile.Coord, fmt.Sprintf("дверь %s стоит на %s", objPath, turf), nil)
				}
				for _, extra := range dmmap.DoorExtraTiles(c.dmm, tile.Coord, p) {
					extraTile := c.tileAt(extra)
					if extraTile == nil {
						continue
					}
					for _, i := range extraTile.Instances() {
						if ep := i.Prefab().Path(); dm.IsPath(ep, "/turf") && badDoorTurf(ep) {
							c.addTest(t, CatTestDoor, extra, fmt.Sprintf("вторая половина двери %s стоит на %s", objPath, ep), nil)
						}
					}
				}
			}

			// MAP: every obj shall have valid access strings in req_access.
			if dm.IsPath(objPath, "/obj") {
				for _, bad := range invalidAccess(p, t.accessIDs) {
					c.addTest(t, CatTestAccess, tile.Coord, fmt.Sprintf("%s: несуществующий доступ %s", objPath, bad), nil)
				}
			}
		}
	}
}

func badDoorTurf(turf string) bool {
	return dm.IsPath(turf, "/turf/simulated/open") || dm.IsPath(turf, "/turf/space")
}

// cableEnds is the New proc of the cable.
func cableEnds(state string) (d1, d2 int, ok bool) {
	parts := strings.SplitN(state, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	d1, err1 := strconv.Atoi(parts[0])
	d2, err2 := strconv.Atoi(parts[1])
	return d1, d2, err1 == nil && err2 == nil
}

func hasKnot(instances dmmap.Instances) bool {
	for _, inst := range instances {
		if p := inst.Prefab(); dm.IsPath(p.Path(), testCablePath) {
			if d1, d2, ok := cableEnds(p.Vars().TextV("icon_state", "")); ok && d1 == 0 && d2 > 0 {
				return true
			}
		}
	}
	return false
}

func (c *checker) cableReaches(coord util.Point, dir int) bool {
	target := c.step(coord, dir)
	if target == nil {
		return false
	}
	back := dm.DirReverse(dir)
	for _, inst := range target.Instances() {
		if p := inst.Prefab(); dm.IsPath(p.Path(), testCablePath) {
			if d1, d2, ok := cableEnds(p.Vars().TextV("icon_state", "")); ok && (d1 == back || d2 == back) {
				return true
			}
		}
	}
	return false
}

func invalidAccess(p *dmmprefab.Prefab, ids map[string]bool) []string {
	value, ok := p.Vars().Value("req_access")
	if !ok || value == dmvars.NullValue || len(ids) == 0 {
		return nil
	}
	var res []string
	for _, access := range dmaccess.Flatten(dmaccess.ParseList(value)) {
		// Defines without quotes are checked by the compiler.
		if strings.HasPrefix(access, "\"") {
			if !ids[access] {
				res = append(res, access)
			}
		} else if _, err := strconv.Atoi(access); err == nil {
			res = append(res, access) // A non-string is never allowed.
		}
	}
	return res
}

// Desired dirs of neighbours for straight pipes: for a curved neighbour in the main and in the reversed direction.
var straightDesired = map[int][2][]int{
	dm.DirSouth: {{dm.DirNorth, dm.DirWest}, {dm.DirSouth, dm.DirEast}},
	dm.DirEast:  {{dm.DirSouth, dm.DirWest}, {dm.DirNorth, dm.DirEast}},
}

type curvedNeighbour struct {
	straight int
	curved   []int
}

// Desired dirs of neighbours for curved pipes in the main and in the curved direction.
var curvedDesired = map[int][2]curvedNeighbour{
	dm.DirNorth: {{dm.DirSouth, []int{dm.DirSouth, dm.DirEast}}, {dm.DirEast, []int{dm.DirSouth, dm.DirWest}}},
	dm.DirEast:  {{dm.DirEast, []int{dm.DirSouth, dm.DirWest}}, {dm.DirSouth, []int{dm.DirNorth, dm.DirWest}}},
	dm.DirSouth: {{dm.DirSouth, []int{dm.DirNorth, dm.DirWest}}, {dm.DirEast, []int{dm.DirNorth, dm.DirEast}}},
	dm.DirWest:  {{dm.DirEast, []int{dm.DirNorth, dm.DirEast}}, {dm.DirSouth, []int{dm.DirSouth, dm.DirEast}}},
}

// MAP: Disposal segments shall connect with other disposal pipes.
func (c *checker) testDisposalSegment(t *mapTestEnv, tile *dmmap.Tile, inst *dmminstance.Instance) {
	p := inst.Prefab()
	dir := dirOf(p)

	if p.Vars().TextV("icon_state", "") == "pipe-s" {
		desired, ok := straightDesired[dir]
		if !ok {
			// Not a failure of the test by itself, but neighbours of this pipe will fail.
			fixed := dm.DirSouth
			if dir == dm.DirWest {
				fixed = dm.DirEast
			}
			c.add(Issue{Severity: SeverityWarning, Category: CatTestDisposal, Coord: tile.Coord,
				Message: fmt.Sprintf("у прямой трубы dir должен быть только south или east, сейчас %s", dm.DirName(dir)),
				Fix: func() bool {
					cur := inst.Prefab()
					vars := dmvars.Delete(cur.Vars(), "dir")
					if fixed != dm.DirSouth {
						vars = dmvars.Set(vars, "dir", strconv.Itoa(fixed))
					}
					inst.SetPrefab(dmmap.PrefabStorage.Get(cur.Path(), vars))
					return true
				}})
			return
		}
		if !c.disposalMatches(c.step(tile.Coord, dir), dir, desired[0]) || !c.disposalMatches(c.step(tile.Coord, dm.DirReverse(dir)), dir, desired[1]) {
			c.addTest(t, CatTestDisposal, tile.Coord, "прямая труба не соединена с соседними", nil)
		}
		return
	}

	desired, ok := curvedDesired[dir]
	if !ok {
		c.addTest(t, CatTestDisposal, tile.Coord, fmt.Sprintf("у угла трубы неверный dir %d", dir), nil)
		return
	}
	main, rev := desired[0], desired[1]
	if !c.disposalMatches(c.step(tile.Coord, dir), main.straight, main.curved) || !c.disposalMatches(c.step(tile.Coord, dm.DirTurnCW(dir)), rev.straight, rev.curved) {
		c.addTest(t, CatTestDisposal, tile.Coord, "угол трубы не соединён с соседними", nil)
	}
}

// disposalMatches is the turf_contains_matching_disposal_pipe proc of the test.
func (c *checker) disposalMatches(tile *dmmap.Tile, straightDir int, curvedDirs []int) bool {
	if tile == nil {
		return false
	}
	for _, inst := range tile.Instances() {
		p := inst.Prefab()
		if !dm.IsPath(p.Path(), testDisposalPath) {
			continue
		}
		if p.Path() != testSegmentPath {
			return true
		}
		dir := dirOf(p)
		if p.Vars().TextV("icon_state", "") == "pipe-s" {
			if dir == straightDir {
				return true
			}
		} else {
			for _, d := range curvedDirs {
				if d == dir {
					return true
				}
			}
		}
	}
	return false
}

// testAreaData is collected for areas of the map. An area is identified by its prefab: var-edited areas are separate objects.
type testAreaData struct {
	path   string
	prefab *dmmprefab.Prefab
	first  util.Point
	minZ   int
	apc    *util.Point
}

func (c *checker) collectAreas() map[uint64]*testAreaData {
	areas := make(map[uint64]*testAreaData)
	for _, tile := range c.dmm.Tiles {
		var data *testAreaData
		for _, inst := range tile.Instances() {
			if p := inst.Prefab(); dm.IsPath(p.Path(), "/area") {
				d, ok := areas[p.Id()]
				if !ok {
					d = &testAreaData{path: p.Path(), prefab: p, first: tile.Coord, minZ: tile.Coord.Z}
					areas[p.Id()] = d
				}
				if tile.Coord.Z < d.minZ {
					d.minZ = tile.Coord.Z
				}
				data = d
			}
		}
		if data == nil {
			continue
		}
		for _, inst := range tile.Instances() {
			if dm.IsPath(inst.Prefab().Path(), "/obj/machinery/power/apc") {
				coord := tile.Coord
				data.apc = &coord
			}
		}
	}
	return areas
}

func tileAreaId(tile *dmmap.Tile) (uint64, bool) {
	for _, inst := range tile.Instances() {
		if p := inst.Prefab(); dm.IsPath(p.Path(), "/area") {
			return p.Id(), true
		}
	}
	return 0, false
}

// MAP: Area Test APC.
func (c *checker) testAPCs(t *mapTestEnv) {
	exempt := rePathValue.FindAllStringSubmatch(t.datum.ValueV("apc_test_exempt_areas", ""), -1)

	flagsOf := func(area string) int {
		// Deeper types come last.
		for i := len(exempt) - 1; i >= 0; i-- {
			if dm.IsPath(area, exempt[i][1]) {
				flags, _ := strconv.Atoi(exempt[i][2])
				return flags
			}
		}
		return 0
	}

	for _, area := range t.areas {
		if !t.playerLevels[area.minZ] {
			continue
		}
		noAPC := flagsOf(area.path)&noAPCFlag != 0
		// The test wants an APC even there, but lamps of such an area work without it.
		if v := area.prefab.Vars().ValueV("requires_power", "1"); area.apc == nil && (v == "0" || v == "FALSE") {
			continue
		}
		if area.apc == nil && !noAPC {
			c.addTest(t, CatTestAPC, area.first, fmt.Sprintf("в зоне %s нет APC", area.path), nil)
		} else if area.apc != nil && noAPC {
			c.addTest(t, CatTestAPC, *area.apc, fmt.Sprintf("в зоне %s не должно быть APC (apc_test_exempt_areas)", area.path), nil)
		}
	}
}

// AREA: Areas shall be coherent.
func (c *checker) testAreasCoherent(t *mapTestEnv) {
	exact := make(map[string]bool)
	for _, p := range rePathItem.FindAllString(t.datum.ValueV("area_coherency_test_exempt_areas", ""), -1) {
		exact[p] = true
	}
	roots := rePathItem.FindAllString(t.datum.ValueV("area_coherency_test_exempted_root_areas", ""), -1)
	expected := make(map[string]int)
	for _, m := range rePathValue.FindAllStringSubmatch(t.datum.ValueV("area_coherency_test_subarea_count", ""), -1) {
		expected[m[1]], _ = strconv.Atoi(m[2])
	}

	w, h, depth := c.dmm.MaxX, c.dmm.MaxY, c.dmm.MaxZ
	index := func(x, y, z int) int { return (z-1)*w*h + (y-1)*w + (x - 1) }
	areaIds := make([]uint64, w*h*depth)
	for _, tile := range c.dmm.Tiles {
		if id, ok := tileAreaId(tile); ok {
			areaIds[index(tile.Coord.X, tile.Coord.Y, tile.Coord.Z)] = id
		}
	}

	visited := make([]bool, len(areaIds))
	parts := make(map[uint64]int)
	second := make(map[uint64]util.Point)
	var stack []util.Point

	for z := 1; z <= depth; z++ {
		for y := 1; y <= h; y++ {
			for x := 1; x <= w; x++ {
				start := index(x, y, z)
				if visited[start] {
					continue
				}
				visited[start] = true
				id := areaIds[start]
				parts[id]++
				if parts[id] == 2 {
					second[id] = util.Point{X: x, Y: y, Z: z}
				}

				stack = append(stack[:0], util.Point{X: x, Y: y, Z: z})
				for len(stack) > 0 {
					cur := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					visit := func(nx, ny, nz int) {
						if nx < 1 || ny < 1 || nz < 1 || nx > w || ny > h || nz > depth {
							return
						}
						i := index(nx, ny, nz)
						if !visited[i] && areaIds[i] == id {
							visited[i] = true
							stack = append(stack, util.Point{X: nx, Y: ny, Z: nz})
						}
					}
					visit(cur.X+1, cur.Y, cur.Z)
					visit(cur.X-1, cur.Y, cur.Z)
					visit(cur.X, cur.Y+1, cur.Z)
					visit(cur.X, cur.Y-1, cur.Z)
					// MULTIZAS is defined: levels are a part of the same space.
					if t.hasAbove[cur.Z] {
						visit(cur.X, cur.Y, cur.Z+1)
					}
					if t.hasAbove[cur.Z-1] {
						visit(cur.X, cur.Y, cur.Z-1)
					}
				}
			}
		}
	}

	for id, area := range t.areas {
		if exact[area.path] {
			continue
		}
		isRoot := false
		for _, root := range roots {
			isRoot = isRoot || dm.IsPath(area.path, root)
		}
		if isRoot {
			continue
		}
		want := expected[area.path]
		if want == 0 {
			want = 1
		}
		if parts[id] != want {
			coord, ok := second[id]
			if !ok {
				coord = area.first
			}
			c.addTest(t, CatTestArea, coord, fmt.Sprintf("зона %s разорвана на %d частей (ожидается %d)", area.path, parts[id], want), nil)
		}
	}
}

// AREA: Areas shall be pure.
func (c *checker) testAreasPure(t *mapTestEnv) {
	exempt := make(map[string]bool)
	if t.datum != nil {
		for _, p := range rePathItem.FindAllString(t.datum.ValueV("area_purity_test_exempt_areas", ""), -1) {
			exempt[p] = true
		}
	}
	for _, area := range t.areas {
		if exempt[area.path] {
			continue
		}
		obj, ok := c.env.Objects[area.path]
		if !ok {
			continue
		}
		if area.prefab.Vars().ValueV("name", "") != obj.Vars.ValueV("name", "") {
			c.addTest(t, CatTestArea, area.first, fmt.Sprintf("у зоны %s на карте изменено имя", area.path), nil)
		}
	}
}

// MAP: Landmark Check.
func (c *checker) testLandmarks(t *mapTestEnv) {
	safe, space := 0, 0
	for _, tile := range c.dmm.Tiles {
		for _, inst := range tile.Instances() {
			switch p := inst.Prefab().Path(); {
			case dm.IsPath(p, "/obj/effect/landmark/test/safe_turf"):
				safe++
			case dm.IsPath(p, "/obj/effect/landmark/test/space_turf"):
				space++
			}
		}
	}
	// These marks are used only by unit tests, the round works without them.
	coord := util.Point{X: 1, Y: 1, Z: 1}
	if safe != 1 {
		c.add(Issue{Severity: SeverityInfo, Category: CatTestLandmark, Coord: coord,
			Message: fmt.Sprintf("меток /obj/effect/landmark/test/safe_turf на карте %d, юнит-тесту нужна ровно одна", safe)})
	}
	if space != 1 {
		c.add(Issue{Severity: SeverityInfo, Category: CatTestLandmark, Coord: coord,
			Message: fmt.Sprintf("меток /obj/effect/landmark/test/space_turf на карте %d, юнит-тесту нужна ровно одна", space)})
	}
}

// MAP: Cryopod Validity Check.
func (c *checker) testCryopods(t *mapTestEnv) {
	type place struct {
		coord util.Point
		area  string
	}
	var pods, computers []place
	podAreas, computerAreas := make(map[string]bool), make(map[string]bool)

	for _, tile := range c.dmm.Tiles {
		area := areaOf(tile)
		for _, inst := range tile.Instances() {
			switch p := inst.Prefab().Path(); {
			case dm.IsPath(p, "/obj/machinery/cryopod"):
				pods = append(pods, place{tile.Coord, area})
				podAreas[area] = true
			case dm.IsPath(p, "/obj/machinery/computer/cryopod"):
				computers = append(computers, place{tile.Coord, area})
				computerAreas[area] = true
			}
		}
	}
	for _, pod := range pods {
		if !computerAreas[pod.area] {
			c.addTest(t, CatTestCryo, pod.coord, fmt.Sprintf("в зоне %s есть криокапсула, но нет её консоли", pod.area), nil)
		}
	}
	for _, computer := range computers {
		if !podAreas[computer.area] {
			c.addTest(t, CatTestCryo, computer.coord, fmt.Sprintf("в зоне %s есть консоль криокапсул, но нет капсул", computer.area), nil)
		}
	}
}

// MAP: Camera unique c_tag check. Cameras without a c_tag get a unique name in the game by themselves.
func (c *checker) testCameras(t *mapTestEnv) {
	first := make(map[string]util.Point)
	for _, tile := range c.dmm.Tiles {
		for _, inst := range tile.Instances() {
			p := inst.Prefab()
			if !dm.IsPath(p.Path(), "/obj/machinery/camera") {
				continue
			}
			tag := p.Vars().ValueV("c_tag", dmvars.NullValue)
			if tag == dmvars.NullValue || tag == `""` {
				continue
			}
			if coord, ok := first[tag]; ok {
				c.addTest(t, CatTestCamera, tile.Coord, fmt.Sprintf("c_tag %s уже есть у камеры на %s", tag, coord), nil)
			} else {
				first[tag] = tile.Coord
			}
		}
	}
}

// MAP: Ladder Check.
func (c *checker) testLadders(t *mapTestEnv) {
	exempt := rePathItem.FindAllString(t.datum.ValueV("ladder_check_exempt_ladders", ""), -1)

	ladderOn := func(coord util.Point) *dmmprefab.Prefab {
		tile := c.tileAt(coord)
		if tile == nil {
			return nil
		}
		for _, inst := range tile.Instances() {
			if dm.IsPath(inst.Prefab().Path(), "/obj/structure/ladder") {
				return inst.Prefab()
			}
		}
		return nil
	}

	check := func(coord util.Point, connected bool, dz, otherDir int, side string) {
		if !connected {
			c.addTest(t, CatTestLadder, coord, fmt.Sprintf("лестница ведёт %s, но там нет уровня", side), nil)
			return
		}
		other := ladderOn(util.Point{X: coord.X, Y: coord.Y, Z: coord.Z + dz})
		if other == nil {
			c.addTest(t, CatTestLadder, coord, fmt.Sprintf("лестница ведёт %s, но там нет лестницы", side), nil)
			return
		}
		if other.Vars().IntV("allowed_directions", dirDown)&otherDir == 0 {
			c.addTest(t, CatTestLadder, coord, fmt.Sprintf("лестница %s (%s) не соединена с этой", side, other.Path()), nil)
		}
	}

	for _, tile := range c.dmm.Tiles {
		for _, inst := range tile.Instances() {
			p := inst.Prefab()
			if !dm.IsPath(p.Path(), "/obj/structure/ladder") {
				continue
			}
			skip := false
			for _, e := range exempt {
				skip = skip || dm.IsPath(p.Path(), e)
			}
			if skip {
				continue
			}
			allowed := p.Vars().IntV("allowed_directions", dirDown)
			coord := tile.Coord
			if allowed&dirUp != 0 {
				check(coord, t.hasAbove[coord.Z], 1, dirDown, "вверх")
			}
			if allowed&dirDown != 0 {
				check(coord, t.hasAbove[coord.Z-1], -1, dirUp, "вниз")
				openTurf := false
				for _, i := range tile.Instances() {
					openTurf = openTurf || dm.IsPath(i.Prefab().Path(), "/turf/simulated/open")
				}
				if !openTurf {
					c.addTest(t, CatTestLadder, coord, "лестница вниз стоит не на open space", nil)
				}
			}
		}
	}
}
