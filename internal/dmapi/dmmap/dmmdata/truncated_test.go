package dmmdata

import (
	"os"
	"path/filepath"
	"testing"
)

// A truncated or broken map must give an error, not a panic.
func TestTruncatedMap(t *testing.T) {
	src := os.Getenv("DMMDATA_MAP")
	if src == "" {
		t.Skip()
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, size := range []int{0, 1, 10, 100, 1000, 5000, len(data) / 3, len(data) / 2, len(data) - 10} {
		path := filepath.Join(dir, "broken.dmm")
		_ = os.WriteFile(path, data[:size], 0o644)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on a map truncated to %d bytes: %v", size, r)
				}
			}()
			if _, err := New(path); err == nil {
				t.Errorf("a map truncated to %d bytes must give an error", size)
			}
		}()
	}
}
