package text

import (
	"io/ioutil"
	"math"
	"unicode/utf8"
	"unsafe"

	glpkg "github.com/tinyrange/gowin/internal/gl"
	"github.com/tinyrange/gowin/internal/third_party/truetype"
)

const (
	HASH_LUT_SIZE uint = 256
	MAX_ROWS      int  = 128
	VERT_COUNT         = 6 * 128
	VERT_STRIDE        = 16
)

var idx int = 0

const (
	TTFONT_FILE int = iota + 1
	TTFONT_MEM
	BMFONT
)

type Stash struct {
	gl glpkg.OpenGL

	tw         int
	th         int
	itw        float64
	ith        float64
	emptyData  []byte
	ttTextures []*Texture
	bmTextures []*Texture
	fonts      []*Font
	drawing    bool
	yInverted  bool
}

type Font struct {
	idx       int
	fType     int
	font      *truetype.FontInfo
	data      []byte
	glyphs    []*Glyph
	lut       [HASH_LUT_SIZE]int
	ascender  float64
	descender float64
	lineh     float64
}

type Row struct {
	x, y, h int16
}

type Texture struct {
	id     uint32
	rows   []*Row
	verts  [VERT_COUNT * 4]float32
	color  [4]float32
	nverts int
}

type Glyph struct {
	codepoint int
	size      int16
	texture   *Texture
	x0        int
	y0        int
	x1        int
	y1        int
	xadv      float64
	xoff      float64
	yoff      float64
	next      int
}

type Quad struct {
	x0, y0, s0, t0 float32
	x1, y1, s1, t1 float32
}

func hashint(a uint) uint {
	a += ^(a << 15)
	a ^= (a >> 10)
	a += (a << 3)
	a ^= (a >> 6)
	a += ^(a << 11)
	a ^= (a >> 16)
	return a
}

func New(gl glpkg.OpenGL, cachew, cacheh int) *Stash {
	stash := &Stash{}

	stash.gl = gl

	// Create data for clearing the textures
	stash.emptyData = make([]byte, cachew*cacheh)

	// Create first texture for the cache
	stash.tw = cachew
	stash.th = cacheh
	stash.itw = 1 / float64(cachew)
	stash.ith = 1 / float64(cacheh)
	gl.Enable(glpkg.Texture2D)
	stash.ttTextures = make([]*Texture, 1)
	stash.ttTextures[0] = &Texture{}
	gl.GenTextures(1, &stash.ttTextures[0].id)
	gl.BindTexture(glpkg.Texture2D, stash.ttTextures[0].id)
	gl.TexImage2D(glpkg.Texture2D, 0, glpkg.Alpha, int32(cachew), int32(cacheh),
		0, glpkg.Alpha, glpkg.UnsignedByte, unsafe.Pointer(&stash.emptyData[0]))
	gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMinFilter, glpkg.Nearest)
	gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMagFilter, glpkg.Nearest)
	gl.TexParameteri(glpkg.Texture2D, glpkg.TextureWrapS, glpkg.ClampToEdge)
	gl.TexParameteri(glpkg.Texture2D, glpkg.TextureWrapT, glpkg.ClampToEdge)
	gl.Disable(glpkg.Texture2D)

	return stash
}

func (s *Stash) AddFontFromMemory(buffer []byte) (int, error) {
	fnt := &Font{}

	// Init hash lookup.
	for i := 0; i < int(HASH_LUT_SIZE); i++ {
		fnt.lut[i] = -1
	}

	fnt.data = buffer

	// Init truetype
	var err error
	fnt.font, err = truetype.InitFont(fnt.data, 0)
	if err != nil {
		return 0, err
	}

	// Store normalized line height. The real line height is calculated
	// by multiplying the lineh by font size.
	ascent, descent, lineGap := fnt.font.GetFontVMetrics()
	fh := float64(ascent - descent)
	fnt.ascender = float64(ascent) / fh
	fnt.descender = float64(descent) / fh
	fnt.lineh = (fh + float64(lineGap)) / fh

	fnt.idx = idx
	fnt.fType = TTFONT_MEM
	s.fonts = append([]*Font{fnt}, s.fonts...)

	idx++
	return idx - 1, nil
}

func (s *Stash) AddFont(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	idx, err := s.AddFontFromMemory(data)
	if err != nil {
		return 0, err
	}
	s.fonts[0].fType = TTFONT_FILE

	return idx, nil
}

func (s *Stash) GetGlyph(fnt *Font, codepoint int, isize int16) *Glyph {
	size := float64(isize) / 10

	// Find code point and size.
	h := hashint(uint(codepoint)) & (HASH_LUT_SIZE - 1)
	for i := fnt.lut[h]; i != -1; i = fnt.glyphs[i].next {
		if fnt.glyphs[i].codepoint == codepoint && (fnt.fType == BMFONT || fnt.glyphs[i].size == isize) {
			return fnt.glyphs[i]
		}
	}
	// Could not find glyph.

	// For bitmap fonts: ignore this glyph.
	if fnt.fType == BMFONT {
		return nil
	}

	// For truetype fonts: create this glyph.
	scale := fnt.font.ScaleForPixelHeight(size)
	g := fnt.font.FindGlyphIndex(codepoint)
	if g == 0 {
		// glyph not found
		return nil
	}
	advance, _ := fnt.font.GetGlyphHMetrics(g)
	x0, y0, x1, y1 := fnt.font.GetGlyphBitmapBox(g, scale, scale)
	gw := x1 - x0
	gh := y1 - y0

	// Check if glyph is larger than maximum texture size
	if gw >= s.tw || gh >= s.th {
		return nil
	}

	// Find texture and row where the glyph can be fit.
	rh := (int16(gh) + 7) & ^7
	var tt int
	texture := s.ttTextures[tt]
	var br *Row
	for br == nil {
		for i := range texture.rows {
			if texture.rows[i].h == rh && int(texture.rows[i].x)+gw+1 <= s.tw {
				br = texture.rows[i]
			}
		}

		// If no row is found, there are 3 possibilities:
		//  - add new row
		//  - try next texture
		//  - create new texture
		if br == nil {
			var py int16
			// Check that there is enough space.
			if len(texture.rows) > 0 {
				py = texture.rows[len(texture.rows)-1].y + texture.rows[len(texture.rows)-1].h + 1
				if int(py+rh) > s.th {
					if tt < len(s.ttTextures)-1 {
						tt++
						texture = s.ttTextures[tt]
					} else {
						// Create new texture
						s.gl.Enable(glpkg.Texture2D)
						texture = &Texture{}
						s.gl.GenTextures(1, &texture.id)
						s.gl.BindTexture(glpkg.Texture2D, texture.id)
						s.gl.TexImage2D(glpkg.Texture2D, 0, glpkg.Alpha,
							int32(s.tw), int32(s.th), 0,
							glpkg.Alpha, glpkg.UnsignedByte,
							unsafe.Pointer(&s.emptyData[0]))
						s.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMinFilter, glpkg.Nearest)
						s.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMagFilter, glpkg.Nearest)
						s.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureWrapS, glpkg.ClampToEdge)
						s.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureWrapT, glpkg.ClampToEdge)
						s.gl.Disable(glpkg.Texture2D)
						s.ttTextures = append(s.ttTextures, texture)
					}
					continue
				}
			}
			// Init and add row
			br = &Row{
				x: 0,
				y: py,
				h: rh,
			}
			texture.rows = append(texture.rows, br)
		}
	}

	// Init glyph.
	glyph := &Glyph{
		codepoint: codepoint,
		size:      isize,
		texture:   texture,
		x0:        int(br.x),
		y0:        int(br.y),
		x1:        int(br.x) + gw,
		y1:        int(br.y) + gh,
		xadv:      scale * float64(advance),
		xoff:      float64(x0),
		yoff:      float64(y0),
		next:      0,
	}
	fnt.glyphs = append(fnt.glyphs, glyph)

	// Advance row location.
	br.x += int16(gw) + 1

	// Insert char to hash lookup.
	glyph.next = fnt.lut[h]
	fnt.lut[h] = len(fnt.glyphs) - 1

	// Rasterize
	bmp := make([]byte, gw*gh)
	bmp = fnt.font.MakeGlyphBitmap(bmp, gw, gh, gw, scale, scale, g)
	if len(bmp) > 0 {
		s.gl.Enable(glpkg.Texture2D)
		// Update texture
		s.gl.BindTexture(glpkg.Texture2D, texture.id)
		s.gl.PixelStorei(glpkg.UnpackAlignment, 1)
		s.gl.TexSubImage2D(glpkg.Texture2D, 0, int32(glyph.x0), int32(glyph.y0),
			int32(gw), int32(gh), glpkg.Alpha, glpkg.UnsignedByte,
			unsafe.Pointer(&bmp[0]))
		s.gl.Disable(glpkg.Texture2D)
	}

	return glyph
}

func (s *Stash) SetYInverted(inverted bool) {
	s.yInverted = inverted
}

func (s *Stash) GetQuad(fnt *Font, glyph *Glyph, isize int16, x, y float64) (float64, float64, *Quad) {
	q := &Quad{}
	scale := float64(1)

	if fnt.fType == BMFONT {
		scale = float64(isize) / float64(glyph.size*10)
	}

	rx := math.Floor(x + scale*glyph.xoff)
	ry := math.Floor(y - scale*glyph.yoff)

	q.x0 = float32(rx)
	q.y0 = float32(ry)
	q.x1 = float32(float64(rx) + scale*float64(glyph.x1-glyph.x0))
	q.y1 = float32(float64(ry) - scale*float64(glyph.y1-glyph.y0))

	q.s0 = float32(float64(glyph.x0) * s.itw)
	q.t0 = float32(float64(glyph.y0) * s.ith)
	q.s1 = float32(float64(glyph.x1) * s.itw)
	q.t1 = float32(float64(glyph.y1) * s.ith)

	if s.yInverted {
		yOffset := float32(2 * y)
		q.y0 = yOffset - q.y0
		q.y1 = yOffset - q.y1
	}

	x += scale * glyph.xadv

	return x, y, q
}

func (s *Stash) FlushDraw() {
	i := 0
	texture := s.ttTextures[i]
	tt := true
	for {
		if texture.nverts > 0 {
			s.gl.Enable(glpkg.Texture2D)
			s.gl.BindTexture(glpkg.Texture2D, texture.id)
			for k := 0; k < texture.nverts; k++ {
				s.gl.Begin(glpkg.Quads)
				s.gl.Color4fv(&texture.color[0])
				s.gl.TexCoord2f(texture.verts[k*4+2], texture.verts[k*4+3])
				s.gl.Vertex2f(texture.verts[k*4+0], texture.verts[k*4+1])
				k++
				s.gl.Color4fv(&texture.color[0])
				s.gl.TexCoord2f(texture.verts[k*4+2], texture.verts[k*4+3])
				s.gl.Vertex2f(texture.verts[k*4+0], texture.verts[k*4+1])
				k++
				s.gl.Color4fv(&texture.color[0])
				s.gl.TexCoord2f(texture.verts[k*4+2], texture.verts[k*4+3])
				s.gl.Vertex2f(texture.verts[k*4+0], texture.verts[k*4+1])
				k++
				s.gl.Color4fv(&texture.color[0])
				s.gl.TexCoord2f(texture.verts[k*4+2], texture.verts[k*4+3])
				s.gl.Vertex2f(texture.verts[k*4+0], texture.verts[k*4+1])
				s.gl.End()
			}
			s.gl.Disable(glpkg.Texture2D)
			texture.nverts = 0
		}
		if tt {
			if i < len(s.ttTextures)-1 {
				i++
				texture = s.ttTextures[i]
			} else {
				i = 0
				if len(s.bmTextures) > 0 {
					texture = s.bmTextures[i]
					tt = false
				} else {
					break
				}
			}
		} else {
			if i < len(s.bmTextures)-1 {
				i++
				texture = s.bmTextures[i]
			} else {
				break
			}
		}
	}
}

func (s *Stash) BeginDraw() {
	if s.drawing {
		s.FlushDraw()
	}
	s.drawing = true
}

func (s *Stash) EndDraw() {
	if !s.drawing {
		return
	}
	s.FlushDraw()
	s.drawing = false
}

func (s *Stash) GetFontByIdx(idx int) *Font {
	for _, f := range s.fonts {
		if f.idx == idx {
			return f
		}
	}
	return nil
}

func (stash *Stash) GetAdvance(idx int, size float64, s string) float64 {
	isize := int16(size * 10)

	var fnt *Font
	for _, f := range stash.fonts {
		if f.idx == idx {
			fnt = f
			break
		}
	}
	if fnt == nil {
		return 0
	}
	if fnt.fType != BMFONT && len(fnt.data) == 0 {
		return 0
	}

	x := float64(0)

	b := []byte(s)
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		glyph := stash.GetGlyph(fnt, int(r), isize)
		if glyph == nil {
			b = b[size:]
			continue
		}
		x, _, _ = stash.GetQuad(fnt, glyph, isize, x, 0)
		b = b[size:]
	}

	return x
}

func (stash *Stash) DrawText(idx int, size, x, y float64, s string, color [4]float32) (nextX float64) {
	isize := int16(size * 10)

	var fnt *Font
	for _, f := range stash.fonts {
		if f.idx == idx {
			fnt = f
			break
		}
	}
	if fnt == nil {
		return 0
	}
	if fnt.fType != BMFONT && len(fnt.data) == 0 {
		return 0
	}

	var q *Quad

	b := []byte(s)
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		glyph := stash.GetGlyph(fnt, int(r), isize)
		if glyph == nil {
			b = b[size:]
			continue
		}
		texture := glyph.texture
		texture.color = color
		if texture.nverts*4 >= VERT_COUNT {
			stash.FlushDraw()
		}

		x, y, q = stash.GetQuad(fnt, glyph, isize, x, y)

		texture.verts[texture.nverts*4+0] = q.x0
		texture.verts[texture.nverts*4+1] = q.y0
		texture.verts[texture.nverts*4+2] = q.s0
		texture.verts[texture.nverts*4+3] = q.t0
		texture.nverts++
		texture.verts[texture.nverts*4+0] = q.x1
		texture.verts[texture.nverts*4+1] = q.y0
		texture.verts[texture.nverts*4+2] = q.s1
		texture.verts[texture.nverts*4+3] = q.t0
		texture.nverts++
		texture.verts[texture.nverts*4+0] = q.x1
		texture.verts[texture.nverts*4+1] = q.y1
		texture.verts[texture.nverts*4+2] = q.s1
		texture.verts[texture.nverts*4+3] = q.t1
		texture.nverts++
		texture.verts[texture.nverts*4+0] = q.x0
		texture.verts[texture.nverts*4+1] = q.y1
		texture.verts[texture.nverts*4+2] = q.s0
		texture.verts[texture.nverts*4+3] = q.t1
		texture.nverts++
		b = b[size:]
	}

	return x
}

func (s *Stash) VMetrics(idx int, size float64) (float64, float64, float64) {
	var fnt *Font
	for _, f := range s.fonts {
		if f.idx == idx {
			fnt = f
			break
		}
	}
	if fnt == nil {
		return 0, 0, 0
	}
	if fnt.fType != BMFONT && len(fnt.data) == 0 {
		return 0, 0, 0
	}
	return fnt.ascender * size, fnt.descender * size, fnt.lineh * size
}
