package dm

import "testing"

func TestDirTransforms(t *testing.T) {
	cases := []struct{ in, cw, flipX int }{
		{DirNorth, DirEast, DirNorth},
		{DirEast, DirSouth, DirWest},
		{DirNortheast, DirSoutheast, DirNorthwest},
		{DirSouthwest, DirNorthwest, DirSoutheast},
		{DirNorth | DirSouth, DirEast | DirWest, DirNorth | DirSouth},
		{0, 0, 0},
		{16, 16, 16},
	}
	for _, c := range cases {
		if got := DirTurnCW(c.in); got != c.cw {
			t.Errorf("cw(%d) = %d, want %d", c.in, got, c.cw)
		}
		if got := DirFlipX(c.in); got != c.flipX {
			t.Errorf("flipX(%d) = %d, want %d", c.in, got, c.flipX)
		}
		if got := DirTurnCCW(DirTurnCW(c.in)); got != c.in {
			t.Errorf("ccw(cw(%d)) = %d", c.in, got)
		}
	}
	if DirReverse(DirNorthwest) != DirSoutheast {
		t.Error("reverse")
	}
}
