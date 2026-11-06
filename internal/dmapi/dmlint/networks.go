package dmlint

import (
	"fmt"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmnet"
	"sdmm/internal/util"
)

// Networks checks are optional heuristics, the CI does not check them.

func (c *checker) neighbour(coord util.Point, dir int) *dmmap.Tile {
	dx, dy := dm.DirOffset(dir)
	n := util.Point{X: coord.X + dx, Y: coord.Y + dy, Z: coord.Z}
	if !c.dmm.HasTile(n) {
		return nil
	}
	return c.dmm.GetTile(n)
}

func (c *checker) checkNetworks(tile *dmmap.Tile) {
	for _, inst := range tile.Instances() {
		p := inst.Prefab()
		path := p.Path()

		switch {
		case dm.IsPath(path, "/obj/structure/cable"):
			piece, ok := dmnet.ParseCableState(p.Vars().TextV("icon_state", ""))
			if !ok {
				continue
			}
			layer := p.Vars().ValueV("cable_layer", "")
			for _, d := range []int{piece.D1, piece.D2} {
				if d == 0 || !isCardinal(d) {
					continue
				}
				if !c.cableConnected(tile, d, layer) {
					c.add(Issue{Severity: SeverityWarning, Category: CatCable, Coord: tile.Coord,
						Message: fmt.Sprintf("кабель %s обрывается в сторону %s", piece.State(), dm.DirName(d))})
				}
			}

		case dm.IsPath(path, "/obj/machinery/atmospherics/pipe"):
			dirs := pipeDirs(path, p.Vars().IntV("dir", dm.DirSouth))
			for _, d := range dm.Cardinals {
				if dirs&d != 0 && !c.atmosConnected(tile, d) {
					c.add(Issue{Severity: SeverityWarning, Category: CatPipe, Coord: tile.Coord,
						Message: fmt.Sprintf("открытый конец трубы в сторону %s", dm.DirName(d))})
				}
			}

		case dm.IsPath(path, "/obj/structure/disposalpipe"):
			dirs, _ := dmnet.DisposalDirs(path, p.Vars().IntV("dir", dm.DirSouth), p.Vars().TextV("icon_state", ""))
			if dm.IsPath(path, "/obj/structure/disposalpipe/trunk") && !hasDisposalUnit(tile) {
				c.add(Issue{Severity: SeverityWarning, Category: CatDisposal, Coord: tile.Coord,
					Message: "trunk без мусорки или выхода на тайле"})
			}
			for _, d := range dm.Cardinals {
				if dirs&d != 0 && !c.disposalConnected(tile, d) {
					c.add(Issue{Severity: SeverityWarning, Category: CatDisposal, Coord: tile.Coord,
						Message: fmt.Sprintf("мусоропровод обрывается в сторону %s", dm.DirName(d))})
				}
			}
		}
	}
}

func isCardinal(d int) bool {
	return d == dm.DirNorth || d == dm.DirSouth || d == dm.DirEast || d == dm.DirWest
}

func (c *checker) cableConnected(tile *dmmap.Tile, d int, layer string) bool {
	n := c.neighbour(tile.Coord, d)
	if n == nil {
		return false
	}
	back := dm.DirReverse(d)
	for _, inst := range n.Instances() {
		p := inst.Prefab()
		if !dm.IsPath(p.Path(), "/obj/structure/cable") {
			continue
		}
		if p.Vars().ValueV("cable_layer", "") != layer {
			continue
		}
		piece, ok := dmnet.ParseCableState(p.Vars().TextV("icon_state", ""))
		if !ok {
			return true // Unknown cable kind (multiz hubs etc.), consider connected.
		}
		if piece.D1 == back || piece.D2 == back {
			return true
		}
	}
	return false
}

func pipeDirs(path string, dir int) int {
	for _, kind := range []string{"manifold4w", "manifold", "simple", "cap"} {
		if strings.Contains(path, "/pipe/"+kind) {
			return dmnet.PipeDirs(kind, dir)
		}
	}
	return 0 // Special pipes (multiz, heat exchangers junctions etc.) are not checked.
}

func (c *checker) atmosConnected(tile *dmmap.Tile, d int) bool {
	n := c.neighbour(tile.Coord, d)
	if n == nil {
		return false
	}
	back := dm.DirReverse(d)
	for _, inst := range n.Instances() {
		path := inst.Prefab().Path()
		if !dm.IsPath(path, "/obj/machinery/atmospherics") {
			continue
		}
		if !dm.IsPath(path, "/obj/machinery/atmospherics/pipe") {
			return true // Devices (vents, pumps etc.) have their own connection rules.
		}
		dirs := pipeDirs(path, inst.Prefab().Vars().IntV("dir", dm.DirSouth))
		if dirs == 0 || dirs&back != 0 {
			return true
		}
	}
	return false
}

func (c *checker) disposalConnected(tile *dmmap.Tile, d int) bool {
	n := c.neighbour(tile.Coord, d)
	if n == nil {
		return false
	}
	back := dm.DirReverse(d)
	for _, inst := range n.Instances() {
		path := inst.Prefab().Path()
		if !dm.IsPath(path, "/obj/structure/disposalpipe") {
			continue
		}
		dirs, _ := dmnet.DisposalDirs(path, inst.Prefab().Vars().IntV("dir", dm.DirSouth), inst.Prefab().Vars().TextV("icon_state", ""))
		if dirs == 0 || dirs&back != 0 {
			return true
		}
	}
	return false
}

func hasDisposalUnit(tile *dmmap.Tile) bool {
	for _, inst := range tile.Instances() {
		path := inst.Prefab().Path()
		if dm.IsPath(path, "/obj/machinery/disposal") || dm.IsPath(path, "/obj/structure/disposaloutlet") {
			return true
		}
	}
	return false
}

// Summary returns the number of issues by severity.
func Summary(issues []Issue) (errors, warnings, infos int) {
	for _, i := range issues {
		switch i.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		default:
			infos++
		}
	}
	return
}
