package dm

import "strings"

// IsPath returns true if the orig path is the provided type or its subtype.
// Only whole parts of the path are compared: /obj/machinery/light_switch is not a /obj/machinery/light.
func IsPath(orig, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if !strings.HasPrefix(orig, path) {
		return false
	}
	return len(orig) == len(path) || orig[len(path)] == '/'
}

// IsPathBaseSame returns true if both provided paths has the same base.
func IsPathBaseSame(p1, p2 string) bool {
	return PathBase(p1) == PathBase(p2)
}

// PathWeight let us sort the content by the path weight. Basically: /obj->/turf->/area.
func PathWeight(p string) int {
	if IsPath(p, "/area") {
		return 3
	}
	if IsPath(p, "/turf") {
		return 2
	}
	return 1
}

// PathBase returns the base of the provided path.
// Example: /obj/item/weapon -> /obj.
func PathBase(p string) string {
	separatorIdx := strings.Index(p[1:], "/") + 1
	return p[:separatorIdx]
}

// PathLast returns the last part of the path (basically, a name of the type).
// Example: /obj/item/weapon -> weapon.
func PathLast(p string) string {
	return p[strings.LastIndex(p, "/")+1:]
}

// IsMovable returns true if the provided path is a "movable" type.
func IsMovable(path string) bool {
	return IsPath(path, "/obj") || IsPath(path, "/mob") || IsPath(path, "/atom/movable")
}
