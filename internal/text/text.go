package text

import (
	_ "embed"

	"github.com/tinyrange/gowin/internal/graphics"
)

//go:embed RobotoMono-VariableFont_wght.ttf
var EMBEDDED_FONT []byte

type Renderer struct {
	stash *Stash
	font  int
}

func Load(win graphics.Window) (*Renderer, error) {
	gl, err := win.PlatformWindow().GL()
	if err != nil {
		return nil, err
	}

	stash := New(gl, 1024, 1024)
	stash.SetYInverted(true)
	fontIdx, err := stash.AddFontFromMemory(EMBEDDED_FONT)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		stash: stash,
		font:  fontIdx,
	}, nil
}

func (r *Renderer) RenderText(s string, x, y float32, size float64, color [4]float32) float32 {
	if r == nil || r.stash == nil {
		return x
	}

	r.stash.BeginDraw()
	next := r.stash.DrawText(r.font, size, float64(x), float64(y), s, color)
	r.stash.EndDraw()
	return float32(next)
}
