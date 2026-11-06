package dmicon

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"sdmm/internal/dmapi/dm"

	"github.com/rs/zerolog/log"
)

var Cache = &IconsCache{
	icons:   make(map[string]*Dmi),
	decoded: make(map[string]decodedDmi),
}

// IconsCache stores loaded icons.
// Icons could be decoded in the background with the Preload method, but OpenGL textures are created
// only in the main thread, when the icon is requested for the first time.
type IconsCache struct {
	mtx sync.Mutex

	rootDirPath string
	icons       map[string]*Dmi

	// decoded stores icons which were decoded in the background, but were not uploaded to the GPU yet.
	decoded map[string]decodedDmi
}

func (i *IconsCache) Free() {
	i.mtx.Lock()
	defer i.mtx.Unlock()

	for _, dmi := range i.icons {
		dmi.free() // nil-safe
	}
	log.Printf("cache free; [%d] icons disposed", len(i.icons))
	i.rootDirPath = ""
	i.icons = make(map[string]*Dmi)
	i.decoded = make(map[string]decodedDmi)
}

func (i *IconsCache) SetRootDirPath(rootDirPath string) {
	i.mtx.Lock()
	defer i.mtx.Unlock()
	i.rootDirPath = rootDirPath
	log.Print("cache root dir:", rootDirPath)
}

// Preload decodes provided icons in parallel. Could be called from any goroutine.
// Decoded icons will be uploaded to the GPU lazily, on the first access from the main thread.
func (i *IconsCache) Preload(icons []string) {
	start := time.Now()

	i.mtx.Lock()
	rootDirPath := i.rootDirPath
	toLoad := make([]string, 0, len(icons))
	seen := make(map[string]bool, len(icons))
	for _, icon := range icons {
		if len(icon) == 0 || seen[icon] {
			continue
		}
		seen[icon] = true
		if _, ok := i.icons[icon]; ok {
			continue
		}
		if _, ok := i.decoded[icon]; ok {
			continue
		}
		toLoad = append(toLoad, icon)
	}
	i.mtx.Unlock()

	if len(toLoad) == 0 {
		return
	}

	jobs := make(chan string)
	var wg sync.WaitGroup

	workers := max(1, runtime.NumCPU())
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for icon := range jobs {
				decoded := decode(rootDirPath + "/" + icon)
				i.mtx.Lock()
				// Environment could be changed while we were decoding.
				if i.rootDirPath == rootDirPath {
					if _, ok := i.icons[icon]; !ok {
						i.decoded[icon] = decoded
					}
				}
				i.mtx.Unlock()
			}
		}()
	}

	for _, icon := range toLoad {
		jobs <- icon
	}
	close(jobs)
	wg.Wait()

	log.Printf("preloaded [%d] icons in [%d] ms", len(toLoad), time.Since(start).Milliseconds())
}

// Get returns the icon. Must be called from the main thread.
func (i *IconsCache) Get(icon string) (*Dmi, error) {
	if len(icon) == 0 {
		return nil, errors.New("dmi icon is empty")
	}

	i.mtx.Lock()
	if dmi, ok := i.icons[icon]; ok {
		i.mtx.Unlock()
		if dmi == nil {
			return nil, fmt.Errorf("dmi [%s] is nil", icon)
		}
		return dmi, nil
	}

	decoded, isDecoded := i.decoded[icon]
	delete(i.decoded, icon)
	rootDirPath := i.rootDirPath
	i.mtx.Unlock()

	path := rootDirPath + "/" + icon
	if !isDecoded {
		decoded = decode(path)
	}

	dmi, err := fromDecoded(path, decoded)

	i.mtx.Lock()
	i.icons[icon] = dmi // Nil values are cached as well to not try to load broken icons over and over.
	i.mtx.Unlock()

	return dmi, err
}

func (i *IconsCache) GetState(icon, state string) (*State, error) {
	dmi, err := i.Get(icon)
	if err != nil {
		return nil, err
	}
	return dmi.State(state)
}

func (i *IconsCache) GetSpriteV(icon, state string, dir int) (*Sprite, error) {
	dmiState, err := i.GetState(icon, state)
	if err != nil {
		return nil, err
	}
	return dmiState.SpriteV(dir), nil
}

func (i *IconsCache) GetSprite(icon, state string) (*Sprite, error) {
	return i.GetSpriteV(icon, state, dm.DirDefault)
}

func (i *IconsCache) GetSpriteOrPlaceholder(icon, state string) *Sprite {
	return i.GetSpriteOrPlaceholderV(icon, state, dm.DirDefault)
}

func (i *IconsCache) GetSpriteOrPlaceholderV(icon, state string, dir int) *Sprite {
	if s, err := i.GetSpriteV(icon, state, dir); err == nil {
		return s
	}
	return SpritePlaceholder()
}
