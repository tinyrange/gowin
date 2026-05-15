package gowin

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"

	"github.com/tinyrange/gowin/graphics"
)

type Texture2D struct {
	g graphics.Texture
}

func LoadImage(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}

func (c *Context) NewTexture(img image.Image) (Texture2D, error) {
	if c == nil || c.win == nil {
		return Texture2D{}, fmt.Errorf("nil context")
	}
	tex, err := c.win.NewTexture(img)
	if err != nil {
		return Texture2D{}, err
	}
	return Texture2D{g: tex}, nil
}

func LoadTexture(ctx *Context, r io.Reader) (Texture2D, error) {
	if ctx == nil || ctx.win == nil {
		return Texture2D{}, fmt.Errorf("nil context")
	}
	img, err := LoadImage(r)
	if err != nil {
		return Texture2D{}, err
	}
	return ctx.NewTexture(img)
}

func (t Texture2D) Size() (int, int) {
	if t.g == nil {
		return 0, 0
	}
	return t.g.Size()
}

type ShaderKind uint8

const (
	ShaderDefault2D ShaderKind = iota
	ShaderDefault3D
	ShaderCustom
)

type Shader struct {
	kind ShaderKind
	g3d  graphics.Shader3D
}

type ShaderSources struct {
	Vertex   io.Reader
	Fragment io.Reader
}

func DefaultShader2D() Shader {
	return Shader{kind: ShaderDefault2D}
}

func DefaultShader3D() Shader {
	return Shader{kind: ShaderDefault3D}
}

func LoadShader(ctx *Context, src ShaderSources) (Shader, error) {
	if ctx == nil || ctx.win == nil {
		return Shader{}, fmt.Errorf("nil context")
	}
	if src.Vertex == nil || src.Fragment == nil {
		return Shader{}, fmt.Errorf("vertex and fragment shader sources are required")
	}
	vertex, err := io.ReadAll(src.Vertex)
	if err != nil {
		return Shader{}, err
	}
	fragment, err := io.ReadAll(src.Fragment)
	if err != nil {
		return Shader{}, err
	}
	shader, err := ctx.win.NewShader3D(string(vertex), string(fragment))
	if err != nil {
		return Shader{}, err
	}
	return Shader{kind: ShaderCustom, g3d: shader}, nil
}

type Vertex3D struct {
	Position Vec3
	Normal   Vec3
	UV       Vec2
	Color    color.Color
}

type MeshData struct {
	Vertices []Vertex3D
	Indices  []uint32
}

type MeshUsage uint8

const (
	StaticMesh MeshUsage = iota
	DynamicMesh
)

type MeshOptions struct {
	Usage MeshUsage
}

type Mesh struct {
	ctx  *Context
	opts MeshOptions
	data MeshData
	g3d  graphics.Mesh3D
}

func (c *Context) NewMesh(opts ...MeshOptions) *Mesh {
	var o MeshOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Mesh{ctx: c, opts: o}
}

func (m *Mesh) SetData(data MeshData) error {
	if m == nil || m.ctx == nil || m.ctx.win == nil {
		return fmt.Errorf("nil mesh context")
	}
	if len(data.Vertices) == 0 || len(data.Indices) == 0 {
		return fmt.Errorf("empty mesh data")
	}
	verts := make([]graphics.Vertex3D, len(data.Vertices))
	for i, v := range data.Vertices {
		c := v.Color
		if c == nil {
			c = White
		}
		r, g, b, a := c.RGBA()
		verts[i] = graphics.Vertex3D{
			X: v.Position.X, Y: v.Position.Y, Z: v.Position.Z,
			NX: v.Normal.X, NY: v.Normal.Y, NZ: v.Normal.Z,
			U: v.UV.X, V: v.UV.Y,
			R: float32(r) / 0xffff, G: float32(g) / 0xffff, B: float32(b) / 0xffff, A: float32(a) / 0xffff,
		}
	}
	g, err := m.ctx.win.NewMesh3D(verts, data.Indices)
	if err != nil {
		return err
	}
	if m.g3d != nil {
		m.g3d.Destroy()
	}
	m.data = data
	m.g3d = g
	return nil
}

func (m *Mesh) Destroy() {
	if m == nil || m.g3d == nil {
		return
	}
	m.g3d.Destroy()
	m.g3d = nil
}

func (s Shader) Destroy() {
	if s.g3d != nil {
		s.g3d.Destroy()
	}
}

func (c *Context) WriteScreenshotPNG(w io.Writer) error {
	if c == nil || c.frame == nil {
		return fmt.Errorf("no active frame")
	}
	img, err := c.frame.ScreenshotLogical()
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}
