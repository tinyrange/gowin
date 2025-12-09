package text

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	"github.com/tinyrange/gowin/internal/graphics"
)

//go:embed font.png
var defaultFontAtlas []byte

// Character order produced by internal/text/generate.py (Python's string.printable[:95]).
const fontCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ \t\n\r\v\f"

// The generator packs characters into a square-ish grid; 10 columns for the default 95 glyph set.
const fontColumns = 10

// Font holds per-glyph textures and sizing info to render text.
type Font struct {
	glyphs     map[rune]graphics.Texture
	cellWidth  float32
	cellHeight float32
	lineHeight float32
}

// Load builds textures for each glyph in the embedded atlas using the provided graphics backend.
func Load(gfx graphics.Window) (*Font, error) {
	atlas, err := png.Decode(bytes.NewReader(defaultFontAtlas))
	if err != nil {
		return nil, fmt.Errorf("decode font atlas: %w", err)
	}

	cols := fontColumns
	rows := (len(fontCharset) + cols - 1) / cols

	cellWidth := atlas.Bounds().Dx() / cols
	cellHeight := atlas.Bounds().Dy() / rows
	if cellWidth == 0 || cellHeight == 0 {
		return nil, fmt.Errorf("invalid cell size from atlas (%d x %d)", cellWidth, cellHeight)
	}

	glyphs := make(map[rune]graphics.Texture, len(fontCharset))

	for idx, r := range fontCharset {
		col := idx % cols
		row := idx / cols
		srcRect := image.Rect(col*cellWidth, row*cellHeight, (col+1)*cellWidth, (row+1)*cellHeight)

		sub := image.NewNRGBA(image.Rect(0, 0, cellWidth, cellHeight))
		draw.Draw(sub, sub.Bounds(), atlas, srcRect.Min, draw.Src)

		tex, err := gfx.NewTexture(sub)
		if err != nil {
			return nil, fmt.Errorf("create texture for rune %q: %w", r, err)
		}

		glyphs[r] = tex
	}

	return &Font{
		glyphs:     glyphs,
		cellWidth:  float32(cellWidth),
		cellHeight: float32(cellHeight),
		lineHeight: float32(cellHeight),
	}, nil
}

// RenderText draws the provided string at the given origin. Newlines advance the cursor to the next line.
func (f *Font) RenderText(frame graphics.Frame, text string, x, y float32) {
	if f == nil {
		return
	}

	cursorX := x
	cursorY := y

	for _, r := range text {
		switch r {
		case '\n':
			cursorX = x
			cursorY += f.lineHeight
			continue
		case '\r':
			continue
		case '\t':
			cursorX += f.cellWidth * 4
			continue
		}

		tex, ok := f.glyphs[r]
		if !ok {
			cursorX += f.cellWidth
			continue
		}

		frame.RenderQuad(cursorX, cursorY, f.cellWidth, f.cellHeight, tex)
		cursorX += f.cellWidth
	}
}
