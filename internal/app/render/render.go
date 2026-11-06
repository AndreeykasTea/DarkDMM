package render

import (
	"time"

	"sdmm/internal/app/render/brush"
	"sdmm/internal/app/render/bucket"
	"sdmm/internal/app/render/bucket/level/chunk/unit"
	"sdmm/internal/app/render/lighting"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"

	"github.com/go-gl/gl/v3.3-core/gl"
)

type Render struct {
	Camera *Camera

	bucket *bucket.Bucket

	overlay       overlay
	unitProcessor unitProcessor

	// Lighting preview data.
	dmm      *dmmap.Dmm
	lighting *lighting.State
	// tileTint is an optional layer drawn over tiles (routes etc.).
	tileTint TileTint

	// Units highlights are drawn after the lighting, so they won't be darkened.
	pendingHighlights []unit.Unit
}

func New() *Render {
	brush.TryInit()
	return &Render{
		Camera:   newCamera(),
		bucket:   bucket.New(),
		lighting: lighting.NewState(),
	}
}

func (r *Render) SetUnitProcessor(processor unitProcessor) {
	r.unitProcessor = processor
}

func (r *Render) SetOverlay(state overlay) {
	r.overlay = state
}

func (r *Render) SetActiveLevel(dmm *dmmap.Dmm, activeLevel int) {
	r.Camera.Level = activeLevel
	if r.bucket.Level(activeLevel) == nil { // Ensure level exists
		r.UpdateBucket(dmm, activeLevel)
	}
}

// UpdateBucketV will update the bucket data by the provided level.
func (r *Render) UpdateBucketV(dmm *dmmap.Dmm, level int, tilesToUpdate []util.Point) {
	r.dmm = dmm
	r.bucket.UpdateLevel(dmm, level, tilesToUpdate)
	r.lighting.Invalidate()
}

// UpdateBucket will ensure that the bucket has data by the provided level.
func (r *Render) UpdateBucket(dmm *dmmap.Dmm, level int) {
	r.UpdateBucketV(dmm, level, nil)
}

// InvalidateLighting forces the lighting to be recomputed.
func (r *Render) InvalidateLighting() {
	r.lighting.Invalidate()
}

// LightingSourcesCount returns the number of light sources on the current level or -1 if lighting is not computed.
func (r *Render) LightingSourcesCount() int {
	if res := r.lighting.Result(r.dmm, r.Camera.Level); res != nil {
		return res.SourcesCount
	}
	return -1
}

// LightingUnpoweredCount returns the number of lamps on the current level, which have no power.
func (r *Render) LightingUnpoweredCount() int {
	if res := r.lighting.Result(r.dmm, r.Camera.Level); res != nil {
		return res.UnpoweredCount
	}
	return 0
}

// Animations are played from the editor start. Ticks are 1/10 of a second, like in BYOND.
var (
	animationStart = time.Now()
	animationTicks float64
)

func (r *Render) Draw(width, height float32) {
	animationTicks = time.Since(animationStart).Seconds() * 10
	r.prepare()
	r.draw(width, height)
	r.cleanup()
}

// Initialize OpenGL state.
func (r *Render) prepare() {
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.BlendEquation(gl.FUNC_ADD)
	gl.ActiveTexture(gl.TEXTURE0)
}

func (r *Render) draw(width, height float32) {
	viewBounds := r.viewportBounds(width, height)
	r.batchBucketUnits(viewBounds)
	r.batchLighting(viewBounds)
	r.batchTileTint(viewBounds)
	r.batchPendingHighlights()
	//r.batchChunksVisuals()
	r.batchOverlayAreasBorders()
	r.batchOverlayAreas()
	brush.Draw(width, height, r.Camera.ShiftX, r.Camera.ShiftY, r.Camera.Scale)
}

// Clean OpenGL state after rendering.
func (r *Render) cleanup() {
	gl.Disable(gl.BLEND)
}

func (r *Render) viewportBounds(width, height float32) util.Bounds {
	// Get transformed bounds of the map, so we can ignore out of bounds units.
	w := width / r.Camera.Scale
	h := height / r.Camera.Scale

	x1 := -r.Camera.ShiftX
	y1 := -r.Camera.ShiftY
	x2 := x1 + w
	y2 := y1 + h

	return util.Bounds{
		X1: x1,
		Y1: y1,
		X2: x2,
		Y2: y2,
	}
}

// ComputeLightingNow computes the lighting of the current level synchronously.
func (r *Render) ComputeLightingNow() {
	r.lighting.ComputeNow(r.dmm, r.Camera.Level)
}
