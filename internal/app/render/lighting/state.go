package lighting

import (
	"sync"
	"time"

	"sdmm/internal/dmapi/dmmap"

	"github.com/rs/zerolog/log"
)

// Settings of the lighting preview. Stored in the editor preferences.
type Settings struct {
	Enabled bool
	// Ambient is a minimal brightness of the dark tiles in percents. Helps to see the map in the dark.
	Ambient int
	// Intensity is a multiplier for all light sources in percents.
	Intensity int
	// ShowSources draws markers on light sources.
	ShowSources bool
	// IgnorePower shows all lamps as working, even if their area has no APC.
	IgnorePower bool
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:     false,
		Ambient:     15,
		Intensity:   100,
		ShowSources: false,
	}
}

// Current is the active settings. It's replaced with a pointer to the preferences on the app start.
var Current = func() *Settings { s := DefaultSettings(); return &s }()

// recomputeThrottle limits how often the lighting is recomputed while the user is editing the map.
const recomputeThrottle = 120 * time.Millisecond

// State is a lighting state of a single map renderer.
// All methods must be called from the main thread, the heavy computation is done in the background.
type State struct {
	extractor *Extractor

	dirty     bool
	level     int
	lastStart time.Time

	mtx       sync.Mutex
	computing bool
	pending   *Result
	pendingLv int

	result      *Result
	resultLevel int

	sources   []Source
	unpowered []Source

	ignorePower bool
}

func NewState() *State {
	return &State{extractor: NewExtractor(), dirty: true}
}

// Invalidate marks the lighting as outdated. It will be recomputed on the next access.
func (s *State) Invalidate() {
	s.dirty = true
}

// Result returns the latest computed lighting for the level.
// If the lighting is outdated, it will start a recomputation in the background and return the previous result.
func (s *State) Result(dmm *dmmap.Dmm, level int) *Result {
	if dmm == nil {
		return nil
	}

	s.mtx.Lock()
	if s.pending != nil {
		s.result, s.resultLevel = s.pending, s.pendingLv
		s.pending = nil
	}
	computing := s.computing
	s.mtx.Unlock()

	if level != s.level {
		s.level = level
		s.dirty = true
	}
	if Current.IgnorePower != s.ignorePower {
		s.ignorePower = Current.IgnorePower
		s.dirty = true
	}

	if s.dirty && !computing && time.Since(s.lastStart) >= recomputeThrottle {
		s.dirty = false
		s.lastStart = time.Now()
		s.start(dmm, level)
	}

	// Drop the result for the different level, so we won't show wrong lighting.
	if s.result != nil && (s.resultLevel != level || s.result.W != dmm.MaxX || s.result.H != dmm.MaxY) {
		return nil
	}
	return s.result
}

func (s *State) start(dmm *dmmap.Dmm, level int) {
	start := time.Now()
	// Data extraction touches the map, so it's done in the main thread. It's fast thanks to the prefab cache.
	grid, sources := s.extractor.Build(dmm, level)
	s.sources, s.unpowered = sources, grid.Unpowered
	extractTime := time.Since(start)

	s.mtx.Lock()
	s.computing = true
	s.mtx.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("lighting computation panic: %v", r)
			}
			s.mtx.Lock()
			s.computing = false
			s.mtx.Unlock()
		}()

		computeStart := time.Now()
		res := Compute(grid, sources)

		s.mtx.Lock()
		s.pending, s.pendingLv = res, level
		s.mtx.Unlock()

		log.Printf("lighting computed: level [%d], sources [%d], extract [%d] ms, compute [%d] ms",
			level, len(sources), extractTime.Milliseconds(), time.Since(computeStart).Milliseconds())
	}()
}

// Sources returns light sources found during the last lighting computation.
func (s *State) Sources() []Source {
	return s.sources
}

// Unpowered returns light sources, which don't shine because their area has no power.
func (s *State) Unpowered() []Source {
	return s.unpowered
}

// ComputeNow computes the lighting synchronously. Used for screenshots, where there is no next frame to wait for.
func (s *State) ComputeNow(dmm *dmmap.Dmm, level int) {
	if dmm == nil {
		return
	}
	grid, sources := s.extractor.Build(dmm, level)
	res := Compute(grid, sources)

	s.mtx.Lock()
	s.pending = nil
	s.mtx.Unlock()

	s.sources, s.unpowered = sources, grid.Unpowered
	s.result, s.resultLevel = res, level
	s.level = level
	s.dirty = false
	s.lastStart = time.Now()
}
