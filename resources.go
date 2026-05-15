package gowin

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/tinyrange/gowin/graphics"
)

type Texture2D struct {
	g graphics.Texture
}

func LoadTexture(ctx *Context, r io.Reader) (Texture2D, error) {
	if ctx == nil || ctx.win == nil {
		return Texture2D{}, fmt.Errorf("nil context")
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return Texture2D{}, err
	}
	tex, err := ctx.win.NewTexture(img)
	if err != nil {
		return Texture2D{}, err
	}
	return Texture2D{g: tex}, nil
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
	kind     ShaderKind
	vertex   string
	fragment string
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
	if ctx == nil {
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
	// Root-level shader resources are intentionally part of the public shape now.
	// Custom shader compilation will be wired into the graphics backend next.
	return Shader{kind: ShaderCustom, vertex: string(vertex), fragment: string(fragment)}, nil
}

type Vertex3D struct {
	Position Vec3
	Normal   Vec3
	UV       Vec2
	Color    Color
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
		if c == (Color{}) {
			c = White
		}
		verts[i] = graphics.Vertex3D{
			X: v.Position.X, Y: v.Position.Y, Z: v.Position.Z,
			NX: v.Normal.X, NY: v.Normal.Y, NZ: v.Normal.Z,
			U: v.UV.X, V: v.UV.Y,
			R: c.R, G: c.G, B: c.B, A: c.A,
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
