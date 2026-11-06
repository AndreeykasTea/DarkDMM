package rsc

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/png"

	xdraw "golang.org/x/image/draw"

	"github.com/rs/zerolog/log"
)

var (
	//go:embed png/editor_texture_atlas.png
	editorSpriteAtlas []byte
	//go:embed png/editor_icon.png
	editorIcon []byte
)

// EditorTextureAtlas returns a sprite atlas with all textures used by the editor.
func EditorTextureAtlas() TextureAtlas {
	return TextureAtlas{
		Width:  64,
		Height: 32,
		data:   editorSpriteAtlas,
	}
}

// EditorIcon returns the application icon. The size is taken from the image, so the icon could be replaced by any PNG.
func EditorIcon() TextureAtlas {
	w, h := 1000, 1000
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(editorIcon)); err == nil {
		w, h = cfg.Width, cfg.Height
	}
	return TextureAtlas{
		Width:  w,
		Height: h,
		data:   editorIcon,
	}
}

// EditorIconSizes returns the application icon scaled to sizes used by the OS (taskbar, title bar).
func EditorIconSizes() []image.Image {
	src := EditorIcon().RGBA()
	var res []image.Image
	for _, size := range []int{256, 64, 48, 32, 16} {
		dst := image.NewNRGBA(image.Rect(0, 0, size, size))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
		res = append(res, dst)
	}
	return res
}

type TextureAtlas struct {
	Width  int
	Height int
	data   []byte
}

func (a TextureAtlas) RGBA() *image.NRGBA {
	res, _, err := image.Decode(bytes.NewReader(a.data))
	if err != nil {
		log.Panic().Msg("unable to decode texture atlas!")
	}
	img := image.NewNRGBA(image.Rect(0, 0, a.Width, a.Height))
	draw.Draw(img, img.Bounds(), res, image.Point{}, draw.Src)
	return img
}
