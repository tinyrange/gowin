package graphics

import (
	"image"
	"image/draw"
	"time"
	"unsafe"

	glpkg "github.com/tinyrange/gowin/internal/gl"
	"github.com/tinyrange/gowin/internal/window"
)

type glWindow struct {
	platform window.Window
	gl       glpkg.OpenGL

	clearEnabled bool
	clearColor   [4]float32
}

type glTexture struct {
	id uint32
	w  int
	h  int
}

type glFrame struct {
	w *glWindow
}

// Screenshot implements Frame.
func (f glFrame) Screenshot() (image.Image, error) {
	bw, bh := f.w.platform.BackingSize()
	rgba := image.NewRGBA(image.Rect(0, 0, bw, bh))
	f.w.gl.ReadPixels(0, 0, int32(bw), int32(bh), glpkg.RGBA, glpkg.UnsignedByte, unsafe.Pointer(&rgba.Pix[0]))

	// Flip the image vertically
	flipped := image.NewRGBA(image.Rect(0, 0, bw, bh))
	for y := 0; y < bh; y++ {
		srcStart := y * rgba.Stride
		srcEnd := srcStart + rgba.Stride
		dstStart := (bh - 1 - y) * flipped.Stride
		dstEnd := dstStart + flipped.Stride
		copy(flipped.Pix[dstStart:dstEnd], rgba.Pix[srcStart:srcEnd])
	}

	return flipped, nil
}

// New returns a Window backed by the macOS Cocoa + OpenGL implementation.
func New(title string, width, height int) (Window, error) {
	return newWithProfile(title, width, height, false)
}

func newWithProfile(title string, width, height int, useCoreProfile bool) (Window, error) {
	platform, err := window.New(title, width, height, useCoreProfile)
	if err != nil {
		return nil, err
	}
	gl, err := platform.GL()
	if err != nil {
		platform.Close()
		return nil, err
	}

	gl.Enable(glpkg.Texture2D)
	gl.Enable(glpkg.Blend)
	gl.BlendFunc(glpkg.SrcAlpha, glpkg.OneMinusSrcAlpha)

	return &glWindow{
		platform:     platform,
		gl:           gl,
		clearEnabled: true,
		clearColor:   [4]float32{0, 0, 0, 1},
	}, nil
}

func (w *glWindow) PlatformWindow() window.Window {
	return w.platform
}

func (w *glWindow) NewTexture(img image.Image) (Texture, error) {
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)

	var texID uint32
	w.gl.GenTextures(1, &texID)
	w.gl.BindTexture(glpkg.Texture2D, texID)
	w.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMinFilter, glpkg.Nearest)
	w.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMagFilter, glpkg.Nearest)

	if len(nrgba.Pix) > 0 {
		w.gl.TexImage2D(
			glpkg.Texture2D,
			0,
			int32(glpkg.RGBA),
			int32(nrgba.Rect.Dx()),
			int32(nrgba.Rect.Dy()),
			0,
			glpkg.RGBA,
			glpkg.UnsignedByte,
			unsafe.Pointer(&nrgba.Pix[0]),
		)
	}

	return &glTexture{id: texID, w: nrgba.Rect.Dx(), h: nrgba.Rect.Dy()}, nil
}

func (w *glWindow) SetClear(enabled bool) {
	w.clearEnabled = enabled
}

func (w *glWindow) SetClearColor(r, g, b, a float32) {
	w.clearColor = [4]float32{r, g, b, a}
}

func (w *glWindow) Loop(step func(f Frame) error) error {
	defer w.platform.Close()

	frame := glFrame{w: w}
	for w.platform.Poll() {
		w.prepareFrame()

		if err := step(frame); err != nil {
			return err
		}

		w.platform.Swap()
		time.Sleep(time.Second / 120)
	}
	return nil
}

func (w *glWindow) prepareFrame() {
	bw, bh := w.platform.BackingSize()

	w.gl.Viewport(0, 0, int32(bw), int32(bh))
	w.gl.MatrixMode(glpkg.Projection)
	w.gl.LoadIdentity()
	w.gl.Ortho(0, float64(bw), float64(bh), 0, -1, 1)
	w.gl.MatrixMode(glpkg.ModelView)
	w.gl.LoadIdentity()

	if w.clearEnabled {
		w.gl.ClearColor(w.clearColor[0], w.clearColor[1], w.clearColor[2], w.clearColor[3])
		w.gl.Clear(glpkg.ColorBufferBit)
	}
}

func (f glFrame) WindowSize() (int, int) {
	return f.w.platform.BackingSize()
}

func (f glFrame) CursorPos() (float32, float32) {
	return f.w.platform.Cursor()
}

func (f glFrame) GetKeyState(window.Key) KeyState {
	return KeyStateUp
}

func (f glFrame) GetButtonState(window.Button) ButtonState {
	return ButtonStateUp
}

func (f glFrame) RenderQuad(x, y, width, height float32, tex Texture, color [4]float32) {
	t, ok := tex.(*glTexture)
	if !ok {
		return
	}

	f.w.gl.Enable(glpkg.Texture2D)
	f.w.gl.BindTexture(glpkg.Texture2D, t.id)
	f.w.gl.Begin(glpkg.TriangleStrip)
	f.w.gl.Color4fv(&color[0])
	f.w.gl.TexCoord2f(0, 0)
	f.w.gl.Vertex2f(x, y)
	f.w.gl.TexCoord2f(1, 0)
	f.w.gl.Vertex2f(x+width, y)
	f.w.gl.TexCoord2f(0, 1)
	f.w.gl.Vertex2f(x, y+height)
	f.w.gl.TexCoord2f(1, 1)
	f.w.gl.Vertex2f(x+width, y+height)
	f.w.gl.End()
}

func (t *glTexture) Size() (int, int) {
	return t.w, t.h
}
