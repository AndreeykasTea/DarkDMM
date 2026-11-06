package dmenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareMapInclude(t *testing.T) {
	root := t.TempDir()
	dmePath := filepath.Join(root, "x.dme")
	dme := "#include \"code\\a.dm\"\r\n#include \"maps\\_map_include.dm\"\r\n#include \"maps\\a\\a_define.dm\"\r\n#include \"maps\\b\\b_define.dm\"\r\n#include \"maps\\c\\c_define.dm\"\r\n"
	writeFile(t, dmePath, dme)
	writeFile(t, filepath.Join(root, "maps", "_map_include.dm"), "/* dummy\n #include in a comment is not at the line start */\n")
	writeFile(t, filepath.Join(root, "maps", "a", "a.dm"), "")
	writeFile(t, filepath.Join(root, "maps", "b", "b.dm"), "")
	// There is no maps/c/c.dm.

	path, cleanup := prepareMapInclude(dmePath)
	if path == dmePath {
		t.Fatal("a temporary file is expected")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`#include "maps\a\a.dm"`, `#include "maps\b\b.dm"`, `#include "code\a.dm"`} {
		if !strings.Contains(text, want) {
			t.Errorf("no %s in\n%s", want, text)
		}
	}
	if strings.Contains(text, "_map_include") || strings.Contains(text, `maps\c\c.dm`) {
		t.Errorf("wrong content:\n%s", text)
	}
	cleanup()
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the temporary file must be removed")
	}
	if got, _ := os.ReadFile(dmePath); string(got) != dme {
		t.Fatal("the original file must stay the same")
	}

	// The script has written the map: nothing to do.
	writeFile(t, filepath.Join(root, "maps", "_map_include.dm"), "#include \"maps\\a\\a.dm\"\n")
	if path, _ := prepareMapInclude(dmePath); path != dmePath {
		t.Fatal("the real map include must be used as is")
	}

	// A usual code base.
	plain := filepath.Join(root, "plain.dme")
	writeFile(t, plain, "#include \"code\\a.dm\"\n")
	if path, _ := prepareMapInclude(plain); path != plain {
		t.Fatal("a usual .dme must be used as is")
	}
}
