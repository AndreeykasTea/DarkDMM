package dmicon

import (
	"os"
	"path/filepath"
	"sdmm/third_party/sdmmparser"
	"testing"
)

func TestFrameAt(t *testing.T) {
	s := &State{Dirs: 1, Frames: 3}
	s.setDelays([]float32{1, 2, 3}) // Frames end at 1, 3 and 6 ticks.
	cases := map[float64]int{0: 0, 0.9: 0, 1: 1, 2.5: 1, 3: 2, 5.9: 2, 6: 0, 7.5: 1}
	for ticks, want := range cases {
		if got := s.FrameAt(ticks); got != want {
			t.Errorf("FrameAt(%v) = %d, want %d", ticks, got, want)
		}
	}
	single := &State{Dirs: 4, Frames: 1}
	single.setDelays(nil)
	if single.IsAnimated() || single.FrameAt(100) != 0 {
		t.Error("single frame state must not be animated")
	}
}

// Parses real icons of the code base, if DMICON_ROOT is set.
func TestRealDelays(t *testing.T) {
	root := os.Getenv("DMICON_ROOT")
	if root == "" {
		t.Skip("DMICON_ROOT is not set")
	}
	found := 0
	_ = filepath.Walk(filepath.Join(root, "icons"), func(path string, info os.FileInfo, err error) error {
		if err != nil || found >= 3 || filepath.Ext(path) != ".dmi" {
			return nil
		}
		meta, err := sdmmparser.ParseIconMetadata(path)
		if err != nil {
			return nil
		}
		for _, st := range meta.States {
			if st.Frames > 1 {
				if len(st.Delays) != st.Frames {
					t.Fatalf("%s [%s]: frames %d, delays %v", path, st.Name, st.Frames, st.Delays)
				}
				t.Logf("%s [%s]: delays %v", filepath.Base(path), st.Name, st.Delays)
				found++
				break
			}
		}
		return nil
	})
	if found == 0 {
		t.Fatal("no animated states found")
	}
}
