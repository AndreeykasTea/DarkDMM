package dmicon

import (
	"os"
	"sdmm/third_party/sdmmparser"
	"testing"
)

func TestPipeIconDirs(t *testing.T) {
	root := os.Getenv("DMICON_ROOT")
	if root == "" {
		t.Skip()
	}
	meta, err := sdmmparser.ParseIconMetadata(root + "/icons/obj/pipes_and_stuff/atmospherics/atmos/pipes.dmi")
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range meta.States {
		switch st.Name {
		case "intact", "intact-supply", "intact-scrubbers", "":
			t.Logf("state %q: dirs %d frames %d", st.Name, st.Dirs, st.Frames)
		}
	}
	t.Logf("total states: %d", len(meta.States))
}
