package dm

const (
	DirNorth = 1
	DirSouth = 2
	DirEast  = 4
	DirWest  = 8

	DirNortheast = 5
	DirNorthwest = 9
	DirSoutheast = 6
	DirSouthwest = 10

	DirDefault = DirSouth
)

// Cardinals are the four main directions.
var Cardinals = []int{DirNorth, DirSouth, DirEast, DirWest}

// DirTurnCW rotates the direction (cardinal or diagonal) by 90 degrees clockwise.
// It's the same as the BYOND turn(dir, -90).
func DirTurnCW(dir int) int {
	return mapDirBits(dir, func(d int) int {
		switch d {
		case DirNorth:
			return DirEast
		case DirEast:
			return DirSouth
		case DirSouth:
			return DirWest
		case DirWest:
			return DirNorth
		}
		return d
	})
}

// DirTurnCCW rotates the direction by 90 degrees counterclockwise (BYOND turn(dir, 90)).
func DirTurnCCW(dir int) int {
	return DirTurnCW(DirTurnCW(DirTurnCW(dir)))
}

// DirReverse returns the opposite direction.
func DirReverse(dir int) int {
	return DirTurnCW(DirTurnCW(dir))
}

// DirFlipX mirrors the direction horizontally (east <-> west).
func DirFlipX(dir int) int {
	return mapDirBits(dir, func(d int) int {
		switch d {
		case DirEast:
			return DirWest
		case DirWest:
			return DirEast
		}
		return d
	})
}

// DirFlipY mirrors the direction vertically (north <-> south).
func DirFlipY(dir int) int {
	return mapDirBits(dir, func(d int) int {
		switch d {
		case DirNorth:
			return DirSouth
		case DirSouth:
			return DirNorth
		}
		return d
	})
}

// mapDirBits applies the transformation to every cardinal bit of the direction.
// Values out of the 1..15 range (like 0 or UP/DOWN) are returned unchanged.
func mapDirBits(dir int, f func(int) int) int {
	if dir <= 0 || dir > 15 {
		return dir
	}
	res := 0
	for _, d := range Cardinals {
		if dir&d != 0 {
			res |= f(d)
		}
	}
	return res
}

// DirOffset returns the coordinate offset of the direction.
func DirOffset(dir int) (dx, dy int) {
	if dir&DirNorth != 0 {
		dy++
	}
	if dir&DirSouth != 0 {
		dy--
	}
	if dir&DirEast != 0 {
		dx++
	}
	if dir&DirWest != 0 {
		dx--
	}
	return dx, dy
}

// DirBetween returns the cardinal direction from one tile to the adjacent one, or 0 if they are not adjacent.
func DirBetween(fromX, fromY, toX, toY int) int {
	switch {
	case toX == fromX && toY == fromY+1:
		return DirNorth
	case toX == fromX && toY == fromY-1:
		return DirSouth
	case toY == fromY && toX == fromX+1:
		return DirEast
	case toY == fromY && toX == fromX-1:
		return DirWest
	}
	return 0
}

// DirName returns the lowercase name of the direction, as it's used in type paths ("north", "southwest").
func DirName(dir int) string {
	switch dir {
	case DirNorth:
		return "north"
	case DirSouth:
		return "south"
	case DirEast:
		return "east"
	case DirWest:
		return "west"
	case DirNortheast:
		return "northeast"
	case DirNorthwest:
		return "northwest"
	case DirSoutheast:
		return "southeast"
	case DirSouthwest:
		return "southwest"
	}
	return ""
}

// DirByName is the reverse of the DirName.
func DirByName(name string) int {
	for _, d := range []int{DirNorth, DirSouth, DirEast, DirWest, DirNortheast, DirNorthwest, DirSoutheast, DirSouthwest} {
		if DirName(d) == name {
			return d
		}
	}
	return 0
}

// FixDir returns the real direction for the value of the dir variable. BYOND treats 0 as the south:
// Baystation disposal pipes have "dir = 0" in the code, and pipes without a dir on the map are looking south.
func FixDir(dir int) int {
	if dir == 0 {
		return DirSouth
	}
	return dir
}
