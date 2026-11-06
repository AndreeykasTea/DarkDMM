// Package dmroute computes where a crew member with provided accesses can walk on the map.
// The access check is the same as the has_access proc of the code base:
// an empty access list opens the door for everyone, with check_one_access any access is enough,
// otherwise all of them are required. TG-like req_one_access is supported as well.
package dmroute

import (
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmaccess"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// Door is a door which requires accesses.
type Door struct {
	ReqAccess    []string
	ReqOneAccess []string
	// CheckOneAccess is the Paradise flag: any access of the ReqAccess is enough.
	CheckOneAccess bool
}

// Opens returns true if the door could be opened with provided accesses.
func (d *Door) Opens(accesses map[string]bool) bool {
	if len(d.ReqAccess) > 0 {
		if !d.CheckOneAccess {
			// Baystation has_access: every element is required, a nested list needs any of its members.
			for _, a := range d.ReqAccess {
				if dmaccess.IsGroup(a) {
					found := false
					for _, one := range dmaccess.ParseList(a) {
						if accesses[one] {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				} else if !accesses[a] {
					return false
				}
			}
		} else {
			found := false
			for _, a := range d.ReqAccess {
				if accesses[a] {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	if len(d.ReqOneAccess) > 0 {
		for _, a := range d.ReqOneAccess {
			if accesses[a] {
				return true
			}
		}
		return false
	}
	return true
}

type cell struct {
	blocked bool
	space   bool
	door    *Door
}

// Grid is a walkability map of a single Z-level.
type Grid struct {
	W, H  int
	Z     int
	cells []cell
	cache map[*dmmprefab.Prefab]prefabInfo
}

type prefabInfo struct {
	blocks bool
	space  bool
	door   *Door
}

func (g *Grid) idx(x, y int) int {
	return (y-1)*g.W + (x - 1)
}

func (g *Grid) inside(x, y int) bool {
	return x >= 1 && y >= 1 && x <= g.W && y <= g.H
}

// DoorAt returns the door on the tile or nil.
func (g *Grid) DoorAt(x, y int) *Door {
	if !g.inside(x, y) {
		return nil
	}
	return g.cells[g.idx(x, y)].door
}

// Build creates the grid for the Z-level of the map.
func Build(dmm *dmmap.Dmm, z int) *Grid {
	g := &Grid{W: dmm.MaxX, H: dmm.MaxY, Z: z, cells: make([]cell, dmm.MaxX*dmm.MaxY), cache: make(map[*dmmprefab.Prefab]prefabInfo)}
	for y := 1; y <= g.H; y++ {
		for x := 1; x <= g.W; x++ {
			c := &g.cells[g.idx(x, y)]
			for _, i := range dmm.GetTile(util.Point{X: x, Y: y, Z: z}).Instances() {
				info, ok := g.cache[i.Prefab()]
				if !ok {
					info = analyze(i.Prefab())
					g.cache[i.Prefab()] = info
				}
				if info.space {
					c.space = true
				}
				if info.door != nil {
					c.door = info.door
					// Multi tile airlocks are closed by fillers.
					for _, extra := range dmmap.DoorExtraTiles(dmm, util.Point{X: x, Y: y, Z: z}, i.Prefab()) {
						if g.inside(extra.X, extra.Y) {
							g.cells[g.idx(extra.X, extra.Y)].door = info.door
						}
					}
				} else if info.blocks {
					c.blocked = true
				}
			}
		}
	}
	return g
}

func boolVar(vars *dmvars.Variables, name string, def bool) bool {
	v, ok := vars.Value(name)
	if !ok || v == dmvars.NullValue {
		return def
	}
	switch strings.TrimSpace(v) {
	case "0", "FALSE":
		return false
	case "1", "TRUE":
		return true
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		return n != 0
	}
	return def
}

// Passable dense objects: they block only one side of the tile or could be climbed over.
var passableDense = []string{
	"/obj/structure/railing",
	"/obj/structure/plasticflaps",
	"/obj/machinery/door/window", // Windoors block only a side, the access is checked as for a door.
}

func analyze(p *dmmprefab.Prefab) prefabInfo {
	path := p.Path()
	vars := p.Vars()

	switch {
	case dm.IsPath(path, "/turf"):
		return prefabInfo{
			blocks: boolVar(vars, "density", false),
			space:  dm.IsPath(path, "/turf/space"),
		}
	case dm.IsPath(path, "/obj/machinery/door/blast"):
		// Baystation shutters and blast doors. They are mapped closed and open themselves by this flag.
		return prefabInfo{blocks: boolVar(vars, "begins_closed", true) && boolVar(vars, "density", true)}
	case dm.IsPath(path, "/obj/machinery/door/poddoor"), dm.IsPath(path, "/obj/machinery/door/firedoor"):
		// Shutters and blast doors are opened by buttons and alarms, not by an ID.
		return prefabInfo{blocks: boolVar(vars, "density", true)}
	case dm.IsPath(path, "/obj/machinery/door"):
		if boolVar(vars, "locked", false) {
			return prefabInfo{blocks: true} // Bolted.
		}
		return prefabInfo{door: &Door{
			ReqAccess:    dmaccess.ParseList(vars.ValueV("req_access", "")),
			ReqOneAccess: dmaccess.ParseList(vars.ValueV("req_one_access", "")),
			// Only Paradise has the flag (TRUE by default), Bay checks all accesses and nested "any of" lists.
			CheckOneAccess: boolVar(vars, "check_one_access", false),
		}}
	case dm.IsPath(path, "/obj/structure/window"):
		// Only full tile windows block the tile, directional ones are on the edge.
		dir := vars.IntV("dir", dm.DirSouth)
		diagonal := dir&(dir-1) != 0 // Baystation full tile windows have a diagonal dir.
		return prefabInfo{blocks: boolVar(vars, "fulltile", false) || strings.Contains(path, "full") || diagonal}
	case dm.IsPath(path, "/obj/effect/spawner/window"):
		return prefabInfo{blocks: true}
	case dm.IsPath(path, "/obj"):
		for _, p := range passableDense {
			if dm.IsPath(path, p) {
				return prefabInfo{}
			}
		}
		return prefabInfo{blocks: boolVar(vars, "density", false)}
	}
	return prefabInfo{}
}

// Result of the route computation.
type Result struct {
	// Dist is a number of steps to every tile from the start, -1 for unreachable tiles.
	Dist []int32
	// ClosedDoors are coordinates of doors which are next to the reachable area, but can't be opened.
	ClosedDoors []util.Point
	Reachable   int
}

// DistAt returns the distance to the tile or -1.
func (g *Grid) DistAt(res *Result, x, y int) int32 {
	if res == nil || !g.inside(x, y) {
		return -1
	}
	return res.Dist[g.idx(x, y)]
}

// Reach computes reachable tiles from the start.
func (g *Grid) Reach(start util.Point, accesses map[string]bool, allowSpace bool) *Result {
	res := &Result{Dist: make([]int32, len(g.cells))}
	for i := range res.Dist {
		res.Dist[i] = -1
	}
	if !g.inside(start.X, start.Y) {
		return res
	}

	closed := make(map[int]bool)
	passable := func(x, y int) bool {
		c := &g.cells[g.idx(x, y)]
		if c.blocked || (c.space && !allowSpace) {
			return false
		}
		if c.door != nil && !c.door.Opens(accesses) {
			closed[g.idx(x, y)] = true
			return false
		}
		return true
	}

	queue := []int{g.idx(start.X, start.Y)}
	res.Dist[queue[0]] = 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		res.Reachable++
		x, y := cur%g.W+1, cur/g.W+1
		for _, d := range dm.Cardinals {
			dx, dy := dm.DirOffset(d)
			nx, ny := x+dx, y+dy
			if !g.inside(nx, ny) {
				continue
			}
			ni := g.idx(nx, ny)
			if res.Dist[ni] >= 0 || !passable(nx, ny) {
				continue
			}
			res.Dist[ni] = res.Dist[cur] + 1
			queue = append(queue, ni)
		}
	}

	for i := range closed {
		res.ClosedDoors = append(res.ClosedDoors, util.Point{X: i%g.W + 1, Y: i/g.W + 1, Z: g.Z})
	}
	return res
}

// Path returns tiles of the shortest path from the start to the end (both inclusive) or nil if it's unreachable.
func (g *Grid) Path(res *Result, end util.Point) []util.Point {
	if g.DistAt(res, end.X, end.Y) < 0 {
		return nil
	}
	path := []util.Point{end}
	x, y := end.X, end.Y
	for d := res.Dist[g.idx(x, y)]; d > 0; d-- {
		for _, dir := range dm.Cardinals {
			dx, dy := dm.DirOffset(dir)
			if g.DistAt(res, x+dx, y+dy) == d-1 {
				x, y = x+dx, y+dy
				break
			}
		}
		path = append(path, util.Point{X: x, Y: y, Z: g.Z})
	}
	// Reverse to go from the start.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
