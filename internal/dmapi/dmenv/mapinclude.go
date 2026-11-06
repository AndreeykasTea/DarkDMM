package dmenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// Some Baystation forks (Foundation-19) choose the map right before the compilation: the .dme includes an empty
// maps/_map_include.dm and a script writes the voted map there. So the .dme itself has no types from map files
// (areas of the map as well), but a mapper needs types of all maps at once. To have them the environment is parsed
// from a temporary copy of the .dme, where maps of the rotation are added in place of that file.
// The rotation is maps with an included maps/NAME/NAME_define.dm file and an existing maps/NAME/NAME.dm one.
var (
	reMapIncludeLine = regexp.MustCompile(`(?im)^[ \t]*#include[ \t]+"maps[\\/]_map_include\.dm"[ \t]*\r?$`)
	reMapDefineLine  = regexp.MustCompile(`(?im)^[ \t]*#include[ \t]+"maps[\\/]([^\\/"]+)[\\/]([^\\/"]+)_define\.dm"`)
	reRealInclude    = regexp.MustCompile(`(?m)^[ \t]*#include[ \t]+"`)
)

// prepareMapInclude returns a path of the file to parse and a function to remove temporary data.
func prepareMapInclude(dmePath string) (string, func()) {
	noop := func() {}

	data, err := os.ReadFile(dmePath)
	if err != nil {
		return dmePath, noop
	}

	maps := rotationMaps(dmePath, data)
	if len(maps) == 0 {
		return dmePath, noop
	}

	var includes strings.Builder
	for _, name := range maps {
		_, _ = fmt.Fprintf(&includes, "#include \"maps\\%s\\%s.dm\"\n", name, name)
	}
	content := reMapIncludeLine.ReplaceAllLiteralString(string(data), includes.String())

	// The file must be next to the original one, since all includes are relative to it.
	tmpPath := filepath.Join(filepath.Dir(dmePath), ".~dmm_env_"+filepath.Base(dmePath))
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		log.Printf("unable to write a temporary .dme [%s]: %v", tmpPath, err)
		return dmePath, noop
	}

	log.Printf("the map is chosen by a script, parsing the environment with maps: %v", maps)
	return tmpPath, func() {
		if err := os.Remove(tmpPath); err != nil {
			log.Printf("unable to remove a temporary .dme [%s]: %v", tmpPath, err)
		}
	}
}

func rotationMaps(dmePath string, dme []byte) []string {
	if !reMapIncludeLine.Match(dme) {
		return nil
	}

	root := filepath.Dir(dmePath)

	// A real map is already written to the file: there is nothing to do.
	if include, err := os.ReadFile(filepath.Join(root, "maps", "_map_include.dm")); err == nil {
		if reRealInclude.Match(include) {
			return nil
		}
	}

	lowerDme := strings.ToLower(strings.ReplaceAll(string(dme), "/", "\\"))

	var maps []string
	seen := make(map[string]bool)
	for _, m := range reMapDefineLine.FindAllSubmatch(dme, -1) {
		dir, name := string(m[1]), string(m[2])
		if !strings.EqualFold(dir, name) || seen[name] {
			continue
		}
		seen[name] = true
		if strings.Contains(lowerDme, strings.ToLower(`"maps\`+dir+`\`+name+`.dm"`)) {
			continue // Already included.
		}
		if _, err := os.Stat(filepath.Join(root, "maps", dir, name+".dm")); err == nil {
			maps = append(maps, name)
		}
	}
	return maps
}
