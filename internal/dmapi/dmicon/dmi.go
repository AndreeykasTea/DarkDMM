package dmicon

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"sdmm/third_party/sdmmparser"

	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/platform"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/rs/zerolog/log"
)

type Dmi struct {
	IconWidth     int
	IconHeight    int
	TextureWidth  int
	TextureHeight int
	Cols, Rows    int
	Texture       uint32
	States        map[string]*State

	// alpha stores only an alpha channel of the icon image.
	// It's used for a pixel perfect hit detection and takes x4 less memory than a full image.
	alpha []uint8
}

func (d *Dmi) free() {
	if d == nil || d.Texture == 0 {
		return
	}
	texture := d.Texture
	d.Texture = 0
	window.RunLater(func() {
		gl.DeleteTextures(1, &texture)
	})
}

// AlphaAt returns an alpha value of the icon pixel in the provided position.
func (d *Dmi) AlphaAt(x, y int) uint8 {
	if x < 0 || y < 0 || x >= d.TextureWidth || y >= d.TextureHeight {
		return 0
	}
	idx := y*d.TextureWidth + x
	if idx >= len(d.alpha) {
		return 0
	}
	return d.alpha[idx]
}

func (d *Dmi) State(state string) (*State, error) {
	if dmiState, ok := d.States[state]; ok {
		return dmiState, nil
	}
	if dmiState, ok := d.States[""]; ok {
		return dmiState, nil
	}
	return nil, fmt.Errorf("no dmi state by name [%s]", state)
}

// decodedDmi is a result of the icon file decoding. It doesn't touch OpenGL, so can be created in any goroutine.
type decodedDmi struct {
	metadata *sdmmparser.IconMetadata
	rgba     *image.NRGBA
	err      error
}

func decode(path string) decodedDmi {
	iconMetadata, err := sdmmparser.ParseIconMetadata(path)
	if err != nil {
		log.Printf("unable to parse icon metadata [%s]: %s", path, err)
		return decodedDmi{err: err}
	}
	if iconMetadata.Width <= 0 || iconMetadata.Height <= 0 {
		return decodedDmi{err: fmt.Errorf("invalid icon size [%s]", path)}
	}

	rgba, err := loadRgbaImage(path)
	if err != nil {
		log.Printf("unable to load rgba image [%s]: %s", path, err)
		return decodedDmi{err: err}
	}

	return decodedDmi{metadata: iconMetadata, rgba: rgba}
}

// New creates a new Dmi from the file. Must be called in the main thread, since it creates OpenGL texture.
func New(path string) (*Dmi, error) {
	return fromDecoded(path, decode(path))
}

func fromDecoded(path string, decoded decodedDmi) (*Dmi, error) {
	if decoded.err != nil {
		return nil, decoded.err
	}

	iconMetadata := decoded.metadata
	rgba := decoded.rgba

	width := rgba.Bounds().Dx()
	height := rgba.Bounds().Dy()

	alpha := make([]uint8, width*height)
	for i := range alpha {
		alpha[i] = rgba.Pix[i*4+3]
	}

	dmi := &Dmi{
		IconWidth:     iconMetadata.Width,
		IconHeight:    iconMetadata.Height,
		TextureWidth:  width,
		TextureHeight: height,
		Cols:          max(1, width/iconMetadata.Width),
		Rows:          max(1, height/iconMetadata.Height),
		Texture:       platform.CreateTexture(rgba),
		States:        make(map[string]*State, len(iconMetadata.States)),
		alpha:         alpha,
	}

	spriteIdx := 0
	maxSprites := dmi.Cols * dmi.Rows

	for _, state := range iconMetadata.States {
		dmiState := &State{
			Dirs:   max(1, state.Dirs),
			Frames: max(1, state.Frames),
		}
		dmiState.setDelays(state.Delays)

		for i := 0; i < dmiState.Dirs*dmiState.Frames; i++ {
			// Broken icons could declare more sprites than the image actually has.
			idx := spriteIdx
			if idx >= maxSprites {
				idx = maxSprites - 1
			}
			dmiState.Sprites = append(dmiState.Sprites, newDmiSprite(dmi, idx))
			spriteIdx += 1
		}

		// The first state with the name has a priority (BYOND behaviour).
		if _, ok := dmi.States[state.Name]; !ok {
			dmi.States[state.Name] = dmiState
		}
	}

	log.Printf("created: [%s]", path)

	return dmi, nil
}

func loadRgbaImage(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("unable to open image file [%s]: %s", path, err)
		return nil, err
	}
	defer f.Close()

	imgOs, _, err := image.Decode(f)
	if err != nil {
		log.Printf("unable to decode image file [%s]: %s", path, err)
		return nil, err
	}

	// Avoid the extra copy when the image is already in the required format.
	if nrgba, ok := imgOs.(*image.NRGBA); ok && nrgba.Rect.Min == (image.Point{}) && nrgba.Stride == nrgba.Rect.Dx()*4 {
		return nrgba, nil
	}

	b := imgOs.Bounds()
	rgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), imgOs, b.Min, draw.Src)
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return nil, fmt.Errorf("unable to convert image to NRGBA")
	}

	return rgba, nil
}

type State struct {
	Dirs, Frames int
	Sprites      []*Sprite

	// frameEnds stores the end time of every frame in ticks, the last value is the animation duration.
	frameEnds []float32
}

func (s *State) setDelays(delays []float32) {
	if s.Frames <= 1 {
		return
	}
	var total float32
	s.frameEnds = make([]float32, s.Frames)
	for i := 0; i < s.Frames; i++ {
		delay := float32(1)
		if i < len(delays) && delays[i] > 0 {
			delay = delays[i]
		}
		total += delay
		s.frameEnds[i] = total
	}
}

// IsAnimated returns true if the state has more than one frame.
func (s *State) IsAnimated() bool {
	return len(s.frameEnds) > 1
}

// FrameAt returns the frame index for the time in ticks (1/10 of a second).
func (s *State) FrameAt(ticks float64) int {
	if len(s.frameEnds) <= 1 {
		return 0
	}
	total := float64(s.frameEnds[len(s.frameEnds)-1])
	t := float32(ticks - total*float64(int(ticks/total)))
	for i, end := range s.frameEnds {
		if t < end {
			return i
		}
	}
	return len(s.frameEnds) - 1
}

func (s State) Sprite() *Sprite {
	return s.SpriteV(dm.DirDefault)
}

func (s State) SpriteV(dir int) *Sprite {
	return s.SpriteByFrame(dir, 0)
}

func (s State) SpriteByFrame(dir, frame int) *Sprite {
	if len(s.Sprites) == 0 {
		return SpritePlaceholder()
	}
	idx := s.dir2idx(dir) + (frame%s.Frames)*s.Dirs
	if idx < 0 || idx >= len(s.Sprites) {
		idx = 0
	}
	return s.Sprites[idx]
}

func (s State) dir2idx(dir int) int {
	if s.Dirs == 1 || dir < dm.DirNorth || dir > dm.DirSouthwest {
		return 0
	}

	idx := 0
	switch dir {
	case dm.DirSouth:
		idx = 0
	case dm.DirNorth:
		idx = 1
	case dm.DirEast:
		idx = 2
	case dm.DirWest:
		idx = 3
	case dm.DirSoutheast:
		idx = 4
	case dm.DirSouthwest:
		idx = 5
	case dm.DirNortheast:
		idx = 6
	case dm.DirNorthwest:
		idx = 7
	}

	// Diagonal dirs on the 4-dir state should fall back to the first dir instead of the next frame.
	if idx < s.Dirs {
		return idx
	}
	return 0
}

type Sprite struct {
	dmi            *Dmi
	X1, Y1, X2, Y2 int
	U1, V1, U2, V2 float32
}

func (s *Sprite) Dmi() *Dmi {
	return s.dmi
}

// IsOpaqueAt returns true if the pixel of the icon atlas in the provided position is not fully transparent.
func (s Sprite) IsOpaqueAt(x, y int) bool {
	return s.dmi.AlphaAt(x, y) != 0
}

func (s Sprite) Texture() uint32 {
	return s.dmi.Texture
}

func (s Sprite) TextureWidth() int {
	return s.dmi.TextureWidth
}

func (s Sprite) TextureHeight() int {
	return s.dmi.TextureHeight
}

func (s Sprite) IconWidth() int {
	return s.dmi.IconWidth
}

func (s Sprite) IconHeight() int {
	return s.dmi.IconHeight
}

func newDmiSprite(dmi *Dmi, idx int) *Sprite {
	const uvMargin = .000001
	x := idx % dmi.Cols
	y := idx / dmi.Cols
	return &Sprite{
		dmi: dmi,
		X1:  x * dmi.IconWidth,
		Y1:  y * dmi.IconHeight,
		X2:  (x + 1) * dmi.IconWidth,
		Y2:  (y + 1) * dmi.IconHeight,
		U1:  float32(x)/float32(dmi.Cols) + uvMargin,
		V1:  float32(y)/float32(dmi.Rows) + uvMargin,
		U2:  float32(x+1)/float32(dmi.Cols) - uvMargin,
		V2:  float32(y+1)/float32(dmi.Rows) - uvMargin,
	}
}
