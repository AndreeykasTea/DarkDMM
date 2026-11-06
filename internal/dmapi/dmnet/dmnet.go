// Package dmnet knows how to build networks of power cables, atmospheric pipes and disposal pipes on the map.
// Given a tile and directions it must be connected to, it places pieces with proper icon states and dirs,
// merging them with already existing pieces of the same network (T-junctions, crossings etc.).
//
// Rules are taken from the code base (Paradise, TG-like):
//   - cable: separate pieces with icon_state "d1-d2", pieces on the same tile are connected
//     when they share a direction, "0-d" is a knot which connects machines on the tile;
//   - pipe: simple (straight with N/S or E/W dir, bent with a diagonal dir), manifold (dir is the side
//     without a connection), manifold4w and cap;
//   - disposal: segment (same dir rules as simple pipe), junction (dir, clockwise and reverse),
//     trunk (connects disposal units).
package dmnet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
)

type Kind int

const (
	// KindLine just places the object on every tile of the route.
	KindLine Kind = iota
	KindCable
	// KindSmartCable is a cable which computes connections by itself (no icon_state on the map).
	KindSmartCable
	KindPipe
	KindDisposal
)

func (k Kind) String() string {
	switch k {
	case KindCable:
		return "Кабель"
	case KindSmartCable:
		return "Кабель (авто)"
	case KindPipe:
		return "Труба"
	case KindDisposal:
		return "Мусоропровод"
	}
	return "Линия объектов"
}

const allCardinals = dm.DirNorth | dm.DirSouth | dm.DirEast | dm.DirWest

var rePipe = regexp.MustCompile(`^(.*/pipe)/(simple|manifold4w|manifold|cap)(/.*)?$`)

// Network describes what is being built.
type Network struct {
	Kind Kind

	env  *dmenv.Dme
	base *dmmprefab.Prefab

	// Pipes: paths of pieces of the same family (e.g. hidden/supply).
	simplePath, manifoldPath, manifold4wPath, capPath string

	// UseCaps places caps on open pipe ends instead of straight pipes.
	UseCaps bool

	// bayDisposals is true for Baystation disposals: corners are "pipe-c" with a cardinal dir.
	bayDisposals bool
	// cableDirs writes d1 and d2 variables of cables, like Baystation maps do.
	cableDirs bool
}

// Detect returns the network for the selected prefab.
func Detect(env *dmenv.Dme, prefab *dmmprefab.Prefab) *Network {
	n := &Network{Kind: KindLine, env: env, base: prefab, UseCaps: true}
	path := prefab.Path()

	switch {
	case dm.IsPath(path, "/obj/structure/cable"):
		if obj, ok := env.Objects[path]; ok {
			if _, hasD1 := obj.Vars.Value("d1"); hasD1 {
				n.Kind = KindCable
				// Baystation maps write d1 and d2 with the icon state, the Paradise maplint bans them.
				_, ownD1 := ownValue(prefab.Vars(), "d1")
				_, statErr := os.Stat(filepath.Join(env.RootDir, "tools", "maplint", "lints", "cable.yaml"))
				n.cableDirs = ownD1 || statErr != nil
				return n
			}
		}
		n.Kind = KindSmartCable
	case dm.IsPath(path, "/obj/structure/disposalpipe"):
		n.Kind = KindDisposal
		_, n.bayDisposals = env.Objects["/obj/structure/disposalpipe/segment/bent"]
	default:
		if m := rePipe.FindStringSubmatch(path); m != nil {
			n.Kind = KindPipe
			prefix, suffix := m[1], m[3]
			connectTypes := prefab.Vars().ValueV("connect_types", "")
			n.simplePath = pipeFamilyPath(env, prefix, "simple", suffix, connectTypes)
			n.manifoldPath = pipeFamilyPath(env, prefix, "manifold", suffix, connectTypes)
			n.manifold4wPath = pipeFamilyPath(env, prefix, "manifold4w", suffix, connectTypes)
			n.capPath = pipeFamilyPath(env, prefix, "cap", suffix, connectTypes)
			if n.simplePath == "" {
				n.simplePath = path
			}
		}
	}
	return n
}

// pipeFamilyPath finds a pipe piece of the kind for the family. Not every color has all pieces in the code
// (e.g. there is "simple/hidden/blue", but no "manifold/hidden/blue"), so parent types are checked as well.
// Only pieces with the same connect_types are used, otherwise they would be a part of another network.
func pipeFamilyPath(env *dmenv.Dme, prefix, kind, suffix, connectTypes string) string {
	for s := suffix; ; {
		path := prefix + "/" + kind + s
		if obj, ok := env.Objects[path]; ok && obj.Vars.ValueV("connect_types", "") == connectTypes {
			return path
		}
		if s == "" {
			return ""
		}
		s = s[:strings.LastIndex(s, "/")]
	}
}

// Description returns a short description of the network for UI.
func (n *Network) Description() string {
	return fmt.Sprintf("%s: %s", n.Kind, n.base.Path())
}

// Connect makes the tile connected to provided directions (a bit mask of cardinals).
// Returns true if the tile was modified and a warning, if something can't be done.
func (n *Network) Connect(tile *dmmap.Tile, dirs int) (changed bool, warning string) {
	dirs &= allCardinals
	switch n.Kind {
	case KindCable:
		return n.connectCable(tile, dirs)
	case KindSmartCable:
		return n.ensureOne(tile, func(i *dmminstance.Instance) bool { return i.Prefab().Path() == n.base.Path() }), ""
	case KindPipe:
		return n.connectPipe(tile, dirs)
	case KindDisposal:
		return n.connectDisposal(tile, dirs)
	}
	return n.ensureOne(tile, func(i *dmminstance.Instance) bool { return i.Prefab().Id() == n.base.Id() }), ""
}

func (n *Network) ensureOne(tile *dmmap.Tile, same func(*dmminstance.Instance) bool) bool {
	for _, i := range tile.Instances() {
		if same(i) {
			return false
		}
	}
	tile.InstancesAdd(n.base)
	return true
}

// prefab returns a prefab of the path with base vars (color etc.) and provided overrides.
// Variables equal to the initial value of the type are not stored.
func (n *Network) prefab(path string, overrides map[string]string) *dmmprefab.Prefab {
	var vars *dmvars.Variables
	obj := n.env.Objects[path]
	if path == n.base.Path() {
		vars = n.base.Vars()
	} else {
		vars = dmvars.FromParent(obj.Vars)
		// Keep the color of the selected piece (cables and pipes of different colors).
		// The color could be declared in the type of the selected piece, while the other piece is a parent type.
		if col, ok := n.base.Vars().Value("color"); ok && col != obj.Vars.ValueV("color", "") {
			vars = dmvars.Set(vars, "color", col)
		}
	}
	for name, value := range overrides {
		initial := ""
		if obj != nil {
			initial = obj.Vars.ValueV(name, "")
		}
		// dir = 0 of the type is the same as the south.
		if name == "dir" && initial == "0" {
			initial = strconv.Itoa(dm.DirSouth)
		}
		if obj != nil && initial == value {
			if _, own := ownValue(vars, name); own {
				vars = dmvars.Delete(vars, name)
			}
			continue
		}
		vars = dmvars.Set(vars, name, value)
	}
	return dmmap.PrefabStorage.Get(path, vars)
}

// ownValue returns the value of the variable only if it's declared in the prefab itself.
func ownValue(vars *dmvars.Variables, name string) (string, bool) {
	for _, n := range vars.Iterate() {
		if n == name {
			return vars.Value(name)
		}
	}
	return "", false
}

func dirOf(p *dmmprefab.Prefab) int {
	return dm.FixDir(p.Vars().IntV("dir", dm.DirSouth))
}

func bitCount(v int) int {
	c := 0
	for _, d := range dm.Cardinals {
		if v&d != 0 {
			c++
		}
	}
	return c
}

func firstBit(v int) int {
	for _, d := range []int{dm.DirNorth, dm.DirSouth, dm.DirEast, dm.DirWest} {
		if v&d != 0 {
			return d
		}
	}
	return 0
}

// ---------------------------------------------------------------------------------------------------------------------
// Cables

// CablePiece is a single cable: d1 is 0 for knots.
type CablePiece struct{ D1, D2 int }

// ParseCableState parses "1-2" icon state.
func ParseCableState(state string) (CablePiece, bool) {
	parts := strings.SplitN(state, "-", 2)
	if len(parts) != 2 {
		return CablePiece{}, false
	}
	d1, err1 := strconv.Atoi(parts[0])
	d2, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return CablePiece{}, false
	}
	if d1 > d2 {
		d1, d2 = d2, d1
	}
	return CablePiece{d1, d2}, true
}

func (c CablePiece) State() string {
	d1, d2 := c.D1, c.D2
	if d1 > d2 {
		d1, d2 = d2, d1
	}
	return fmt.Sprintf("%d-%d", d1, d2)
}

// PlanCable returns cable pieces to add, so the tile with existing pieces gets connected to all dirs.
func PlanCable(existing []CablePiece, dirs int) []CablePiece {
	existingDirs := 0
	for _, p := range existing {
		existingDirs |= p.D1 | p.D2
	}
	targets := dirs &^ existingDirs
	if targets == 0 {
		return nil
	}

	var res []CablePiece
	if existingDirs != 0 {
		// Pieces on the same tile are connected if they share a direction.
		anchor := firstBit(existingDirs)
		for _, p := range existing {
			if p.D1 == 0 {
				anchor = 0 // Knots are connected through the center, so new knots connect to them and machines.
				break
			}
		}
		for _, d := range dm.Cardinals {
			if targets&d != 0 {
				res = append(res, CablePiece{anchor, d})
			}
		}
		return res
	}

	if bitCount(dirs) == 1 {
		// A dead end: the knot connects machines placed on the tile (APC, SMES etc.).
		return []CablePiece{{0, dirs}}
	}

	anchor := firstBit(dirs)
	for _, d := range dm.Cardinals {
		if d != anchor && dirs&d != 0 {
			res = append(res, CablePiece{anchor, d})
		}
	}
	return res
}

func (n *Network) connectCable(tile *dmmap.Tile, dirs int) (bool, string) {
	var existing []CablePiece
	for _, i := range tile.Instances() {
		if i.Prefab().Path() != n.base.Path() { // Cables of other layers/colors are separate networks.
			continue
		}
		if piece, ok := ParseCableState(i.Prefab().Vars().TextV("icon_state", "")); ok {
			existing = append(existing, piece)
		}
	}
	plan := PlanCable(existing, dirs)
	for _, piece := range plan {
		overrides := map[string]string{"icon_state": strconv.Quote(piece.State())}
		if n.cableDirs {
			d1, d2 := piece.D1, piece.D2
			if d1 > d2 {
				d1, d2 = d2, d1
			}
			overrides["d1"], overrides["d2"] = strconv.Itoa(d1), strconv.Itoa(d2)
		}
		tile.InstancesAdd(n.prefab(n.base.Path(), overrides))
	}
	return len(plan) > 0, ""
}

// ---------------------------------------------------------------------------------------------------------------------
// Pipes

// PipeDirs returns connections of the pipe piece.
func PipeDirs(kind string, dir int) int {
	switch kind {
	case "simple":
		switch dir {
		case dm.DirNorth, dm.DirSouth:
			return dm.DirNorth | dm.DirSouth
		case dm.DirEast, dm.DirWest:
			return dm.DirEast | dm.DirWest
		}
		return dir & allCardinals
	case "manifold":
		return allCardinals &^ dir
	case "manifold4w":
		return allCardinals
	case "cap":
		return dir & allCardinals
	}
	return 0
}

// PipePiece is a chosen pipe piece: one of simple, manifold, manifold4w, cap.
type PipePiece struct {
	Kind string
	Dir  int
}

// PlanPipe returns a pipe piece for the provided connections.
func PlanPipe(dirs int, useCap bool) PipePiece {
	switch bitCount(dirs) {
	case 1:
		if useCap {
			return PipePiece{"cap", dirs}
		}
		return PipePiece{"simple", dirs}
	case 2:
		switch dirs {
		case dm.DirNorth | dm.DirSouth:
			return PipePiece{"simple", dm.DirSouth}
		case dm.DirEast | dm.DirWest:
			return PipePiece{"simple", dm.DirEast}
		}
		return PipePiece{"simple", dirs} // A bend with a diagonal dir.
	case 3:
		return PipePiece{"manifold", allCardinals &^ dirs}
	case 4:
		return PipePiece{"manifold4w", dm.DirSouth}
	}
	return PipePiece{}
}

func (n *Network) pipeKindOf(path string) string {
	switch path {
	case n.simplePath:
		return "simple"
	case n.manifoldPath:
		return "manifold"
	case n.manifold4wPath:
		return "manifold4w"
	case n.capPath:
		return "cap"
	}
	return ""
}

func (n *Network) pipePath(kind string) string {
	switch kind {
	case "simple":
		return n.simplePath
	case "manifold":
		return n.manifoldPath
	case "manifold4w":
		return n.manifold4wPath
	case "cap":
		return n.capPath
	}
	return ""
}

func (n *Network) connectPipe(tile *dmmap.Tile, dirs int) (bool, string) {
	current := 0
	var old []*dmminstance.Instance
	for _, i := range tile.Instances() {
		if kind := n.pipeKindOf(i.Prefab().Path()); kind != "" {
			current |= PipeDirs(kind, dirOf(i.Prefab()))
			old = append(old, i)
		}
	}

	// A dead end on a tile with an atmospheric device: the device is rotated to the pipe instead.
	if current == 0 && bitCount(dirs) == 1 {
		for _, i := range tile.Instances() {
			path := i.Prefab().Path()
			if !dm.IsPath(path, "/obj/machinery/atmospherics") || dm.IsPath(path, "/obj/machinery/atmospherics/pipe") {
				continue
			}
			if dm.IsPath(path, "/obj/machinery/atmospherics/unary") {
				if dirOf(i.Prefab()) != dirs {
					i.SetPrefab(n.prefabWithDir(i.Prefab(), dirs))
					return true, ""
				}
				return false, ""
			}
			return false, fmt.Sprintf("на тайле %s стоит устройство %s, труба не поставлена", tile.Coord, path)
		}
	}

	want := current | dirs
	if want == current && len(old) == 1 {
		return false, ""
	}

	// Caps are replaced when the pipe continues.
	piece := PlanPipe(want, n.UseCaps && n.capPath != "")
	path := n.pipePath(piece.Kind)
	if path == "" {
		return false, fmt.Sprintf("нет типа %s для этой трубы (тайл %s)", piece.Kind, tile.Coord)
	}

	for _, i := range old {
		tile.InstancesRemoveByInstance(i)
	}
	tile.InstancesAdd(n.prefab(path, map[string]string{"dir": strconv.Itoa(piece.Dir)}))
	return true, ""
}

func (n *Network) prefabWithDir(p *dmmprefab.Prefab, dir int) *dmmprefab.Prefab {
	vars := p.Vars()
	if obj, ok := n.env.Objects[p.Path()]; ok && dm.FixDir(obj.Vars.IntV("dir", dm.DirSouth)) == dir {
		vars = dmvars.Delete(vars, "dir")
	} else {
		vars = dmvars.Set(vars, "dir", strconv.Itoa(dir))
	}
	return dmmap.PrefabStorage.Get(p.Path(), vars)
}

// ---------------------------------------------------------------------------------------------------------------------
// Disposals

const (
	disposalSegment  = "/obj/structure/disposalpipe/segment"
	disposalJunction = "/obj/structure/disposalpipe/junction"
	disposalTrunk    = "/obj/structure/disposalpipe/trunk"

	// Baystation corner: a cardinal dir with this icon state connects dir and clockwise(dir).
	disposalCornerState = "pipe-c"
)

// DisposalDirs returns connections of the disposal pipe piece.
// Junctions are recognized by their icon state, it's the same in Paradise and Baystation:
// "pipe-j1" connects dir, clockwise(dir) and reverse(dir), "pipe-j2" is mirrored, "pipe-y" is a Y junction.
// The second value is false for special pieces, which must not be replaced (sorting junctions etc.).
func DisposalDirs(path string, dir int, iconState string) (int, bool) {
	dir = dm.FixDir(dir)
	switch {
	case dm.IsPath(path, disposalSegment):
		if iconState == disposalCornerState {
			return dir | dm.DirTurnCW(dir), true
		}
		return PipeDirs("simple", dir), true
	case dm.IsPath(path, disposalTrunk):
		return dir & allCardinals, true
	case dm.IsPath(path, "/obj/structure/disposalpipe/up"), dm.IsPath(path, "/obj/structure/disposalpipe/down"):
		// The second end goes to another level.
		return dir & allCardinals, false
	case dm.IsPath(path, "/obj/structure/disposalpipe/tagger"):
		return PipeDirs("simple", dir), false
	case dm.IsPath(path, disposalJunction), dm.IsPath(path, "/obj/structure/disposalpipe/sortjunction"),
		dm.IsPath(path, "/obj/structure/disposalpipe/diversion_junction"):
		replaceable := path == disposalJunction && iconState != "pipe-j2" && iconState != "pipe-y"
		switch iconState {
		case "pipe-j2", "pipe-j2s":
			return dir | dm.DirTurnCCW(dir) | dm.DirReverse(dir), replaceable
		case "pipe-y":
			return dir | dm.DirTurnCW(dir) | dm.DirTurnCCW(dir), replaceable
		}
		if strings.HasSuffix(path, "/reversed") || strings.HasSuffix(path, "/mirrored") || strings.HasSuffix(path, "/flipped") {
			return dir | dm.DirTurnCCW(dir) | dm.DirReverse(dir), false
		}
		if strings.HasSuffix(path, "/yjunction") {
			return dir | dm.DirTurnCW(dir) | dm.DirTurnCCW(dir), false
		}
		return dir | dm.DirTurnCW(dir) | dm.DirReverse(dir), replaceable
	}
	return 0, false
}

// DisposalPiece is a chosen disposal piece.
type DisposalPiece struct {
	Path string
	Dir  int
	// IconState is set for Baystation corners ("pipe-c"), empty means the initial state of the type.
	IconState string
}

// PlanDisposal returns a disposal piece for provided connections.
// bayCorners selects the Baystation corner style (cardinal dir + "pipe-c") instead of the diagonal dir.
func PlanDisposal(dirs int, hasUnit, bayCorners bool) (DisposalPiece, bool) {
	switch bitCount(dirs) {
	case 1:
		if hasUnit {
			return DisposalPiece{Path: disposalTrunk, Dir: dirs}, true
		}
		if bayCorners {
			// Unit tests of Baystation require straight pipes to have only the south or the east dir.
			return DisposalPiece{Path: disposalSegment, Dir: PlanPipe(dirs|dm.DirReverse(dirs), false).Dir}, true
		}
		return DisposalPiece{Path: disposalSegment, Dir: dirs}, true
	case 2:
		p := PlanPipe(dirs, false)
		if bayCorners && p.Dir != dm.DirSouth && p.Dir != dm.DirEast {
			// A bend: find the dir whose clockwise neighbour is the second connection.
			for _, d := range dm.Cardinals {
				if d|dm.DirTurnCW(d) == dirs {
					return DisposalPiece{Path: disposalSegment, Dir: d, IconState: disposalCornerState}, true
				}
			}
		}
		return DisposalPiece{Path: disposalSegment, Dir: p.Dir}, true
	case 3:
		// junction connections are: dir, clockwise(dir) and reverse(dir), so the missing one is counterclockwise.
		missing := allCardinals &^ dirs
		return DisposalPiece{Path: disposalJunction, Dir: dm.DirTurnCW(missing)}, true
	}
	return DisposalPiece{}, false
}

func isDisposalUnit(path string) bool {
	return dm.IsPath(path, "/obj/machinery/disposal") || dm.IsPath(path, "/obj/structure/disposaloutlet")
}

func (n *Network) connectDisposal(tile *dmmap.Tile, dirs int) (bool, string) {
	current := 0
	var old []*dmminstance.Instance
	hasUnit := false
	for _, i := range tile.Instances() {
		path := i.Prefab().Path()
		if isDisposalUnit(path) {
			hasUnit = true
		}
		if !dm.IsPath(path, "/obj/structure/disposalpipe") {
			continue
		}
		d, replaceable := DisposalDirs(path, dirOf(i.Prefab()), i.Prefab().Vars().TextV("icon_state", ""))
		if !replaceable {
			if d&dirs == dirs {
				return false, ""
			}
			return false, fmt.Sprintf("особый узел мусоропровода на %s не изменён: %s", tile.Coord, path)
		}
		current |= d
		old = append(old, i)
	}

	want := current | dirs
	if want == current && len(old) == 1 {
		return false, ""
	}

	piece, ok := PlanDisposal(want, hasUnit, n.bayDisposals)
	if !ok {
		return false, fmt.Sprintf("у мусоропровода не бывает 4-сторонних соединений (тайл %s)", tile.Coord)
	}
	obj, exists := n.env.Objects[piece.Path]
	if !exists {
		return false, "в окружении нет типа " + piece.Path
	}

	overrides := map[string]string{"dir": strconv.Itoa(piece.Dir)}
	if piece.IconState != "" {
		overrides["icon_state"] = strconv.Quote(piece.IconState)
	} else if n.bayDisposals {
		// A straight piece after the corner: back to the initial icon state of the type.
		overrides["icon_state"] = obj.Vars.ValueV("icon_state", "")
	}

	for _, i := range old {
		tile.InstancesRemoveByInstance(i)
	}
	tile.InstancesAdd(n.prefab(piece.Path, overrides))
	return true, ""
}
