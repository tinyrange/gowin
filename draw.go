package gowin

import (
	"fmt"
	"image/color"

	"github.com/tinyrange/gowin/graphics"
)

type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

type Uniforms map[string]interface{}

type DrawOptions struct {
	Shader    Shader
	Transform Mat4
	Textures  map[string]Texture2D
	Uniforms  Uniforms
}

type DrawCommand struct {
	mesh      *Mesh
	shader    Shader
	transform Mat4
	textures  map[string]Texture2D
	uniforms  Uniforms
}

func (c *Context) PrepareDraw(mesh *Mesh, opts DrawOptions) (*DrawCommand, error) {
	if mesh == nil {
		return nil, fmt.Errorf("nil mesh")
	}
	cmd := &DrawCommand{
		mesh:      mesh,
		shader:    opts.Shader,
		transform: opts.Transform,
		textures:  cloneTextures(opts.Textures),
		uniforms:  cloneUniforms(opts.Uniforms),
	}
	if cmd.shader == (Shader{}) {
		cmd.shader = DefaultShader3D()
	}
	return cmd, nil
}

func (c *Context) MustPrepareDraw(mesh *Mesh, opts DrawOptions) *DrawCommand {
	cmd, err := c.PrepareDraw(mesh, opts)
	if err != nil {
		panic(err)
	}
	return cmd
}

func (d *DrawCommand) SetUniform(name string, value interface{}) {
	if d.uniforms == nil {
		d.uniforms = Uniforms{}
	}
	d.uniforms[name] = value
}

func (d *DrawCommand) SetTransform(transform Mat4) {
	d.transform = transform
}

func (d *DrawCommand) SetTexture(name string, tex Texture2D) {
	if d.textures == nil {
		d.textures = map[string]Texture2D{}
	}
	d.textures[name] = tex
}

func (c *Context) Begin3D(camera Camera3D) {
	c.camera3D = &camera
}

func (c *Context) End3D() {
	c.camera3D = nil
}

func (c *Context) Draw(cmd *DrawCommand) {
	if c == nil || c.frame == nil || cmd == nil || cmd.mesh == nil || cmd.mesh.g3d == nil {
		return
	}
	w, h := c.frame.WindowSize()
	aspect := float32(1)
	if h != 0 {
		aspect = float32(w) / float32(h)
	}

	view := graphics.IdentityMat4()
	proj := graphics.PerspectiveMat4(0.7853982, aspect, 0.01, 1000)
	if c.camera3D != nil {
		view = c.camera3D.view()
		proj = c.camera3D.projection(aspect)
	}

	ambient := float32(0.28)
	if v, ok := uniformFloat32(cmd.uniforms, "u_Ambient"); ok {
		ambient = v
	}
	light := graphics.Vec3{X: -0.35, Y: 0.6, Z: 0.7}
	if v, ok := uniformVec3(cmd.uniforms, "u_LightDirection"); ok {
		light = v.graphics()
	}
	fogStart, _ := uniformFloat32(cmd.uniforms, "u_FogStart")
	fogEnd, _ := uniformFloat32(cmd.uniforms, "u_FogEnd")
	fogColor := color.Color(nil)
	if v, ok := uniformColor(cmd.uniforms, "u_FogColor"); ok {
		fogColor = v
	}

	model := graphics.Mat4(cmd.transform)
	if model == (graphics.Mat4{}) {
		model = graphics.IdentityMat4()
	}

	c.frame.RenderMesh3D(cmd.mesh.g3d, graphics.Draw3DOptions{
		Model:          model,
		View:           view,
		Projection:     proj,
		Shader:         cmd.shader.g3d,
		Textures:       graphicsTextures(cmd.textures),
		Uniforms:       graphicsUniforms(cmd.uniforms),
		Ambient:        ambient,
		LightDirection: light,
		FogStart:       fogStart,
		FogEnd:         fogEnd,
		FogColor:       fogColor,
	})
}

func (c *Context) DrawTexture(tex Texture2D, dst Rect) {
	if c == nil || c.frame == nil {
		return
	}
	c.frame.RenderQuad(dst.X, dst.Y, dst.Width, dst.Height, tex.g, color.White)
}

func (c *Context) DrawRectangle(rect Rect, col color.Color) {
	if c == nil || c.frame == nil {
		return
	}
	c.frame.RenderQuad(rect.X, rect.Y, rect.Width, rect.Height, nil, col)
}

func (c *Context) DrawText(s string, x, y, size float32, col color.Color) float32 {
	if c == nil || c.text == nil {
		return x
	}
	return c.text.RenderText(s, x, y, float64(size), col)
}

func (c *Context) DrawCube(pos, size Vec3, col color.Color) {
	if c == nil {
		return
	}
	verts, idx := graphics.Cuboid3DGeometry(size.X, size.Y, size.Z, col)
	mesh := c.NewMesh()
	data := MeshData{
		Vertices: make([]Vertex3D, len(verts)),
		Indices:  idx,
	}
	for i, v := range verts {
		data.Vertices[i] = Vertex3D{
			Position: Vec3{X: v.X, Y: v.Y, Z: v.Z},
			Normal:   Vec3{X: v.NX, Y: v.NY, Z: v.NZ},
			UV:       Vec2{X: v.U, Y: v.V},
			Color: color.NRGBA{
				R: uint8(v.R * 255),
				G: uint8(v.G * 255),
				B: uint8(v.B * 255),
				A: uint8(v.A * 255),
			},
		}
	}
	if err := mesh.SetData(data); err != nil {
		return
	}
	defer mesh.Destroy()
	cmd := c.MustPrepareDraw(mesh, DrawOptions{Transform: Translate3D(pos.X, pos.Y, pos.Z)})
	c.Draw(cmd)
}

func cloneUniforms(in Uniforms) Uniforms {
	if len(in) == 0 {
		return nil
	}
	out := make(Uniforms, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTextures(in map[string]Texture2D) map[string]Texture2D {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]Texture2D, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func graphicsTextures(in map[string]Texture2D) map[string]graphics.Texture {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]graphics.Texture, len(in))
	for k, v := range in {
		out[k] = v.g
	}
	return out
}

func graphicsUniforms(in Uniforms) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		switch value := v.(type) {
		case Vec3:
			out[k] = value.graphics()
		case Mat4:
			out[k] = graphics.Mat4(value)
		default:
			out[k] = value
		}
	}
	return out
}

func uniformFloat32(uniforms Uniforms, name string) (float32, bool) {
	if uniforms == nil {
		return 0, false
	}
	switch v := uniforms[name].(type) {
	case float32:
		return v, true
	case float64:
		return float32(v), true
	case int:
		return float32(v), true
	default:
		return 0, false
	}
}

func uniformVec3(uniforms Uniforms, name string) (Vec3, bool) {
	if uniforms == nil {
		return Vec3{}, false
	}
	v, ok := uniforms[name].(Vec3)
	return v, ok
}

func uniformColor(uniforms Uniforms, name string) (color.Color, bool) {
	if uniforms == nil {
		return nil, false
	}
	v, ok := uniforms[name].(color.Color)
	return v, ok
}
