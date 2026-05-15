package graphics

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"
	"unsafe"

	glpkg "github.com/tinyrange/gowin/gl"
	"github.com/tinyrange/gowin/window"
)

const (
	vertexShaderSource = `#version 150
in vec2 a_position;
in vec2 a_texCoord;
in vec4 a_color;

out vec2 v_texCoord;
out vec4 v_color;

uniform mat4 u_proj;
uniform mat4 u_model;

void main() {
	gl_Position = u_proj * u_model * vec4(a_position, 0.0, 1.0);
	v_texCoord = a_texCoord;
	v_color = a_color;
}`

	fragmentShaderSource = `#version 150
in vec2 v_texCoord;
in vec4 v_color;

out vec4 fragColor;

uniform sampler2D u_texture;
uniform sampler2D u_mask;
uniform int u_useMask;

void main() {
	vec4 color = texture(u_texture, v_texCoord) * v_color;
	if (u_useMask == 1) {
		color.a *= texture(u_mask, v_texCoord).a;
	}
	fragColor = color;
}`

	vertex3DShaderSource = `#version 150
in vec3 a_position;
in vec3 a_normal;
in vec2 a_texCoord;
in vec4 a_color;

out vec3 v_normal;
out vec4 v_color;

uniform mat4 u_model;
uniform mat4 u_view;
uniform mat4 u_projection;

void main() {
	gl_Position = u_projection * u_view * u_model * vec4(a_position, 1.0);
	v_normal = mat3(u_model) * a_normal;
	v_color = a_color;
}`

	fragment3DShaderSource = `#version 150
in vec3 v_normal;
in vec4 v_color;

out vec4 fragColor;

uniform vec4 u_lightDirection;
uniform float u_ambient;

void main() {
	vec3 n = normalize(v_normal);
	vec3 l = normalize(u_lightDirection.xyz);
	float diffuse = max(dot(n, l), 0.0);
	float lit = clamp(u_ambient + diffuse * (1.0 - u_ambient), 0.0, 1.0);
	fragColor = vec4(v_color.rgb * lit, v_color.a);
}`
)

type glWindow struct {
	platform window.Window
	gl       glpkg.OpenGL

	clearEnabled bool
	clearColor   color.Color
	scale        float32

	// GL3 resources
	shaderProgram  uint32
	vao            uint32
	vbo            uint32
	projUniform    int32
	modelUniform   int32
	textureUniform int32
	maskUniform    int32
	useMaskUniform int32

	shader3DProgram     uint32
	model3DUniform      int32
	view3DUniform       int32
	projection3DUniform int32
	light3DUniform      int32
	ambient3DUniform    int32

	// Lazily-created 1x1 white texture for callers that pass nil.
	whiteTex *glTexture

	// Meshes created via NewMesh/NewDynamicMesh, for cleanup when the window loop exits.
	meshes   []Mesh
	meshes3D []Mesh3D

	clipStack    []Rect
	stencilDepth int
	projectionW  float32
	projectionH  float32
}

type glTexture struct {
	id uint32
	w  int
	h  int
}

type glMesh struct {
	vao uint32
	vbo uint32
	ebo uint32

	indexCount int32

	tex *glTexture
	w   *glWindow
}

func (*glMesh) isMesh() {}

type glDynamicMesh struct {
	vao uint32
	vbo uint32
	ebo uint32

	vertexCap  int
	indexCount int32

	tex *glTexture
	w   *glWindow
}

func (*glDynamicMesh) isMesh() {}

type glMesh3D struct {
	vao uint32
	vbo uint32
	ebo uint32

	indexCount int32
	w          *glWindow
}

func (*glMesh3D) isMesh3D() {}

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

func (f glFrame) ScreenshotLogical() (image.Image, error) {
	img, err := f.Screenshot()
	if err != nil {
		return nil, err
	}
	w, h := f.WindowSize()
	return ResizeImageNearest(img, w, h), nil
}

// New returns a Window backed by OpenGL implementation.
//
// width and height are logical pixels on every platform. Platform backends
// choose a backing framebuffer size from that logical size and the current
// display scale; use Frame.BackingSize and Frame.Scale to inspect it per frame.
func New(title string, width, height int) (Window, error) {
	return newWithProfile(title, width, height, true)
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

	// Check GL version
	versionStr := gl.GetString(glpkg.Version)
	var major, minor int
	if _, err := fmt.Sscanf(versionStr, "%d.%d", &major, &minor); err != nil || major < 3 {
		platform.Close()
		return nil, fmt.Errorf("OpenGL 3.0+ required, got version: %s", versionStr)
	}

	gl.Enable(glpkg.Blend)
	gl.BlendFunc(glpkg.SrcAlpha, glpkg.OneMinusSrcAlpha)

	w := &glWindow{
		platform:     platform,
		gl:           gl,
		clearEnabled: true,
		clearColor:   ColorBlack,
		scale:        platform.Scale(),
	}

	// Create shader program
	program, err := createShaderProgram(gl, vertexShaderSource, fragmentShaderSource)
	if err != nil {
		platform.Close()
		return nil, fmt.Errorf("failed to create shader program: %v", err)
	}
	w.shaderProgram = program
	w.projUniform = gl.GetUniformLocation(program, "u_proj")
	w.modelUniform = gl.GetUniformLocation(program, "u_model")
	w.textureUniform = gl.GetUniformLocation(program, "u_texture")
	w.maskUniform = gl.GetUniformLocation(program, "u_mask")
	w.useMaskUniform = gl.GetUniformLocation(program, "u_useMask")

	program3D, err := createShaderProgram(gl, vertex3DShaderSource, fragment3DShaderSource)
	if err != nil {
		platform.Close()
		return nil, fmt.Errorf("failed to create 3D shader program: %v", err)
	}
	w.shader3DProgram = program3D
	w.model3DUniform = gl.GetUniformLocation(program3D, "u_model")
	w.view3DUniform = gl.GetUniformLocation(program3D, "u_view")
	w.projection3DUniform = gl.GetUniformLocation(program3D, "u_projection")
	w.light3DUniform = gl.GetUniformLocation(program3D, "u_lightDirection")
	w.ambient3DUniform = gl.GetUniformLocation(program3D, "u_ambient")

	// Create VAO and VBO
	var vao, vbo uint32
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	w.vao = vao
	w.vbo = vbo

	gl.BindVertexArray(vao)
	gl.BindBuffer(glpkg.ArrayBuffer, vbo)
	// Allocate buffer for 6 vertices (2 triangles) * (2 pos + 2 tex + 4 color) floats
	gl.BufferData(glpkg.ArrayBuffer, 6*8*4, nil, glpkg.DynamicDraw)

	// Set up vertex attributes
	// Position: 2 floats at offset 0
	posLoc := gl.GetAttribLocation(program, "a_position")
	texLoc := gl.GetAttribLocation(program, "a_texCoord")
	colLoc := gl.GetAttribLocation(program, "a_color")
	gl.VertexAttribPointer(uint32(posLoc), 2, glpkg.Float, false, 8*4, 0)
	gl.EnableVertexAttribArray(uint32(posLoc))
	// TexCoord: 2 floats at offset 2*4 = 8
	gl.VertexAttribPointer(uint32(texLoc), 2, glpkg.Float, false, 8*4, 8)
	gl.EnableVertexAttribArray(uint32(texLoc))
	// Color: 4 floats at offset 4*4 = 16
	gl.VertexAttribPointer(uint32(colLoc), 4, glpkg.Float, false, 8*4, 16)
	gl.EnableVertexAttribArray(uint32(colLoc))

	return w, nil
}

func createShaderProgram(gl glpkg.OpenGL, vertexSrc, fragmentSrc string) (uint32, error) {
	// Create and compile vertex shader
	vertexShader := gl.CreateShader(glpkg.VertexShader)
	gl.ShaderSource(vertexShader, vertexSrc)
	gl.CompileShader(vertexShader)
	var status int32
	gl.GetShaderiv(vertexShader, glpkg.CompileStatus, &status)
	if status == 0 {
		log := gl.GetShaderInfoLog(vertexShader)
		gl.DeleteShader(vertexShader)
		return 0, fmt.Errorf("vertex shader compilation failed: %s", log)
	}

	// Create and compile fragment shader
	fragmentShader := gl.CreateShader(glpkg.FragmentShader)
	gl.ShaderSource(fragmentShader, fragmentSrc)
	gl.CompileShader(fragmentShader)
	gl.GetShaderiv(fragmentShader, glpkg.CompileStatus, &status)
	if status == 0 {
		log := gl.GetShaderInfoLog(fragmentShader)
		gl.DeleteShader(vertexShader)
		gl.DeleteShader(fragmentShader)
		return 0, fmt.Errorf("fragment shader compilation failed: %s", log)
	}

	// Create program and link
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)
	gl.GetProgramiv(program, glpkg.LinkStatus, &status)
	if status == 0 {
		log := gl.GetProgramInfoLog(program)
		gl.DeleteShader(vertexShader)
		gl.DeleteShader(fragmentShader)
		gl.DeleteProgram(program)
		return 0, fmt.Errorf("program linking failed: %s", log)
	}

	// Shaders can be deleted after linking
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func (w *glWindow) PlatformWindow() window.Window {
	return w.platform
}

func (w *glWindow) Scale() float32 {
	return w.scale
}

func (w *glWindow) GetShaderProgram() uint32 {
	return w.shaderProgram
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

func (w *glWindow) SetClearColor(c color.Color) {
	w.clearColor = c
}

func (w *glWindow) Loop(step func(f Frame) error) error {
	defer w.platform.Close()
	defer func() {
		var vao, vbo uint32 = w.vao, w.vbo
		for len(w.meshes) > 0 {
			w.meshes[len(w.meshes)-1].Destroy()
		}
		for len(w.meshes3D) > 0 {
			w.meshes3D[len(w.meshes3D)-1].Destroy()
		}
		w.gl.DeleteVertexArrays(1, &vao)
		w.gl.DeleteBuffers(1, &vbo)
		w.gl.DeleteProgram(w.shader3DProgram)
		w.gl.DeleteProgram(w.shaderProgram)
	}()

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
	// Refresh scale every frame. Some platforms can change DPI/scale at runtime
	// (e.g. moving between monitors), and macOS can report updated backing metrics
	// after maximize/fullscreen transitions.
	w.scale = w.platform.Scale()

	w.bindWindowFramebuffer()
	w.resetFrameState()

	if w.clearEnabled {
		rgba := ColorToFloat32(w.clearColor)
		w.gl.ClearColor(rgba[0], rgba[1], rgba[2], rgba[3])
		w.gl.Clear(glpkg.ColorBufferBit)
	}
}

func (w *glWindow) bindWindowFramebuffer() {
	bw, bh := w.platform.BackingSize()
	w.gl.BindFramebuffer(glpkg.Framebuffer, 0)
	w.gl.Viewport(0, 0, int32(bw), int32(bh))
	w.setProjection(float32(bw)/w.scale, float32(bh)/w.scale)
}

func (w *glWindow) resetFrameState() {
	w.clipStack = w.clipStack[:0]
	w.gl.Disable(glpkg.ScissorTest)
	w.gl.Disable(glpkg.DepthTest)
	w.stencilDepth = 0
	w.gl.Disable(glpkg.StencilTest)
	w.gl.StencilMask(0xff)
	w.gl.ClearStencil(0)
	w.gl.Clear(glpkg.StencilBufferBit | glpkg.DepthBufferBit)
}

func (w *glWindow) setProjection(width, height float32) {
	w.projectionW = width
	w.projectionH = height
	proj := orthoMatrix(0, width, height, 0, -1, 1)
	w.gl.UseProgram(w.shaderProgram)
	w.gl.BindVertexArray(w.vao)
	w.gl.UniformMatrix4fv(w.projUniform, 1, false, &proj[0])
	model := IdentityMat4()
	w.gl.UniformMatrix4fv(w.modelUniform, 1, false, &model[0])
}

// orthoMatrix creates an orthographic projection matrix (column-major)
func orthoMatrix(left, right, bottom, top, near, far float32) [16]float32 {
	// Column-major order
	return [16]float32{
		2.0 / (right - left), 0, 0, 0,
		0, 2.0 / (top - bottom), 0, 0,
		0, 0, -2.0 / (far - near), 0,
		-(right + left) / (right - left), -(top + bottom) / (top - bottom), -(far + near) / (far - near), 1,
	}
}

func (f glFrame) WindowSize() (int, int) {
	bw, bh := f.w.platform.BackingSize()
	// The graphics coordinate system is logical units (backing/scale).
	return int(math.Round(float64(float32(bw) / f.w.scale))), int(math.Round(float64(float32(bh) / f.w.scale)))
}

func (f glFrame) BackingSize() (int, int) {
	return f.w.platform.BackingSize()
}

func (f glFrame) Scale() float32 {
	return f.w.scale
}

func (f glFrame) CursorPos() (float32, float32) {
	x, y := f.w.platform.Cursor()
	// Convert from physical pixel coordinates to logical coordinates
	// by dividing by the scale factor
	return x / f.w.scale, y / f.w.scale
}

func (f glFrame) GetKeyState(key window.Key) window.KeyState {
	return f.w.platform.GetKeyState(key)
}

func (f glFrame) GetButtonState(button window.Button) window.ButtonState {
	return f.w.platform.GetButtonState(button)
}

func (f glFrame) TextInput() string {
	return f.w.platform.TextInput()
}

func (f glFrame) RenderQuad(x, y, width, height float32, tex Texture, c color.Color) {
	f.renderQuadWithUV(x, y, width, height, tex, c, false)
}

// RenderFBOTexture renders an FBO texture with flipped V coordinates.
func (f glFrame) RenderFBOTexture(x, y, width, height float32, tex Texture, c color.Color) {
	f.renderQuadWithUV(x, y, width, height, tex, c, true)
}

func (f glFrame) renderQuadWithUV(x, y, width, height float32, tex Texture, c color.Color, flipV bool) {
	f.renderQuadWithUVMask(x, y, width, height, tex, nil, c, flipV)
}

func (f glFrame) renderQuadWithUVMask(x, y, width, height float32, tex Texture, mask Texture, c color.Color, flipV bool) {
	var t *glTexture
	if tex == nil {
		t = f.w.getWhiteTexture()
	} else {
		var ok bool
		t, ok = tex.(*glTexture)
		if !ok {
			return
		}
	}

	// Bind texture
	f.w.gl.ActiveTexture(glpkg.Texture0)
	f.w.gl.BindTexture(glpkg.Texture2D, t.id)
	f.w.gl.Uniform1i(f.w.textureUniform, 0)
	if maskTex, ok := mask.(*glTexture); ok && maskTex != nil {
		f.w.gl.ActiveTexture(glpkg.Texture1)
		f.w.gl.BindTexture(glpkg.Texture2D, maskTex.id)
		f.w.gl.Uniform1i(f.w.maskUniform, 1)
		f.w.gl.Uniform1i(f.w.useMaskUniform, 1)
	} else {
		f.w.gl.Uniform1i(f.w.useMaskUniform, 0)
	}

	// Convert color to float32 RGBA
	rgba := ColorToFloat32(c)

	// Ensure quads render with identity model matrix.
	model := IdentityMat4()
	f.w.gl.UseProgram(f.w.shaderProgram)
	f.w.gl.UniformMatrix4fv(f.w.modelUniform, 1, false, &model[0])

	var vertices [6 * 8]float32
	if flipV {
		// Flipped V for FBO textures: read from (0,1) at top, (0,0) at bottom
		vertices = [6 * 8]float32{
			// Triangle 1
			x, y, 0, 1, rgba[0], rgba[1], rgba[2], rgba[3], // top-left
			x + width, y, 1, 1, rgba[0], rgba[1], rgba[2], rgba[3], // top-right
			x, y + height, 0, 0, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-left
			// Triangle 2
			x + width, y, 1, 1, rgba[0], rgba[1], rgba[2], rgba[3], // top-right
			x + width, y + height, 1, 0, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-right
			x, y + height, 0, 0, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-left
		}
	} else {
		// Standard UV for normal textures: (0,0) at top-left
		vertices = [6 * 8]float32{
			// Triangle 1
			x, y, 0, 0, rgba[0], rgba[1], rgba[2], rgba[3], // top-left
			x + width, y, 1, 0, rgba[0], rgba[1], rgba[2], rgba[3], // top-right
			x, y + height, 0, 1, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-left
			// Triangle 2
			x + width, y, 1, 0, rgba[0], rgba[1], rgba[2], rgba[3], // top-right
			x + width, y + height, 1, 1, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-right
			x, y + height, 0, 1, rgba[0], rgba[1], rgba[2], rgba[3], // bottom-left
		}
	}

	f.w.gl.BindBuffer(glpkg.ArrayBuffer, f.w.vbo)
	f.w.gl.BufferSubData(glpkg.ArrayBuffer, 0, len(vertices)*4, unsafe.Pointer(&vertices[0]))

	// Draw
	f.w.gl.BindVertexArray(f.w.vao)
	f.w.gl.DrawArrays(glpkg.Triangles, 0, 6)
	f.w.gl.Uniform1i(f.w.useMaskUniform, 0)
}

func (f glFrame) RenderMaskedQuad(x, y, width, height float32, tex Texture, mask Texture, c color.Color) {
	f.renderQuadWithUVMask(x, y, width, height, tex, mask, c, false)
}

func (w *glWindow) NewMesh(vertices []Vertex, indices []uint32, tex Texture) (Mesh, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("empty mesh (vertices=%d indices=%d)", len(vertices), len(indices))
	}

	var t *glTexture
	if tex == nil {
		t = w.getWhiteTexture()
	} else {
		var ok bool
		t, ok = tex.(*glTexture)
		if !ok {
			return nil, fmt.Errorf("unsupported texture implementation")
		}
	}

	// Create VAO/VBO/EBO for this mesh.
	var vao, vbo, ebo uint32
	w.gl.GenVertexArrays(1, &vao)
	w.gl.GenBuffers(1, &vbo)
	w.gl.GenBuffers(1, &ebo)

	w.gl.BindVertexArray(vao)

	// Vertex buffer.
	w.gl.BindBuffer(glpkg.ArrayBuffer, vbo)
	w.gl.BufferData(
		glpkg.ArrayBuffer,
		len(vertices)*8*4,
		unsafe.Pointer(&vertices[0]),
		glpkg.StaticDraw,
	)

	// Index buffer.
	w.gl.BindBuffer(glpkg.ElementArrayBuffer, ebo)
	w.gl.BufferData(
		glpkg.ElementArrayBuffer,
		len(indices)*4,
		unsafe.Pointer(&indices[0]),
		glpkg.StaticDraw,
	)

	// Set up vertex attributes for this VAO.
	posLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_position")
	texLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_texCoord")
	colLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_color")
	w.gl.VertexAttribPointer(uint32(posLoc), 2, glpkg.Float, false, 8*4, 0)
	w.gl.EnableVertexAttribArray(uint32(posLoc))
	w.gl.VertexAttribPointer(uint32(texLoc), 2, glpkg.Float, false, 8*4, 8)
	w.gl.EnableVertexAttribArray(uint32(texLoc))
	w.gl.VertexAttribPointer(uint32(colLoc), 4, glpkg.Float, false, 8*4, 16)
	w.gl.EnableVertexAttribArray(uint32(colLoc))

	m := &glMesh{
		vao:        vao,
		vbo:        vbo,
		ebo:        ebo,
		indexCount: int32(len(indices)),
		tex:        t,
		w:          w,
	}
	w.meshes = append(w.meshes, m)
	return m, nil
}

func (w *glWindow) NewDynamicMesh(maxVertices, maxIndices int, tex Texture) (DynamicMesh, error) {
	if maxVertices == 0 || maxIndices == 0 {
		return nil, fmt.Errorf("empty dynamic mesh (maxVertices=%d maxIndices=%d)", maxVertices, maxIndices)
	}

	var t *glTexture
	if tex == nil {
		t = w.getWhiteTexture()
	} else {
		var ok bool
		t, ok = tex.(*glTexture)
		if !ok {
			return nil, fmt.Errorf("unsupported texture implementation")
		}
	}

	// Create VAO/VBO/EBO for this mesh.
	var vao, vbo, ebo uint32
	w.gl.GenVertexArrays(1, &vao)
	w.gl.GenBuffers(1, &vbo)
	w.gl.GenBuffers(1, &ebo)

	w.gl.BindVertexArray(vao)

	// Vertex buffer with DynamicDraw for frequent updates.
	w.gl.BindBuffer(glpkg.ArrayBuffer, vbo)
	w.gl.BufferData(
		glpkg.ArrayBuffer,
		maxVertices*8*4, // 8 floats per vertex * 4 bytes
		nil,
		glpkg.DynamicDraw,
	)

	// Index buffer.
	w.gl.BindBuffer(glpkg.ElementArrayBuffer, ebo)
	w.gl.BufferData(
		glpkg.ElementArrayBuffer,
		maxIndices*4,
		nil,
		glpkg.DynamicDraw,
	)

	// Set up vertex attributes for this VAO.
	posLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_position")
	texLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_texCoord")
	colLoc := w.gl.GetAttribLocation(w.shaderProgram, "a_color")
	w.gl.VertexAttribPointer(uint32(posLoc), 2, glpkg.Float, false, 8*4, 0)
	w.gl.EnableVertexAttribArray(uint32(posLoc))
	w.gl.VertexAttribPointer(uint32(texLoc), 2, glpkg.Float, false, 8*4, 8)
	w.gl.EnableVertexAttribArray(uint32(texLoc))
	w.gl.VertexAttribPointer(uint32(colLoc), 4, glpkg.Float, false, 8*4, 16)
	w.gl.EnableVertexAttribArray(uint32(colLoc))

	m := &glDynamicMesh{
		vao:        vao,
		vbo:        vbo,
		ebo:        ebo,
		vertexCap:  maxVertices,
		indexCount: 0, // Set by UpdateIndices when indices are uploaded
		tex:        t,
		w:          w,
	}

	// Unbind VAO to prevent accidental state pollution.
	// EBO binding is stored in VAO, so leaving a VAO bound can cause
	// subsequent BindBuffer(ElementArrayBuffer) calls to corrupt it.
	w.gl.BindVertexArray(0)

	w.meshes = append(w.meshes, m)
	return m, nil
}

func (w *glWindow) NewMesh3D(vertices []Vertex3D, indices []uint32) (Mesh3D, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("empty 3D mesh (vertices=%d indices=%d)", len(vertices), len(indices))
	}

	var vao, vbo, ebo uint32
	w.gl.GenVertexArrays(1, &vao)
	w.gl.GenBuffers(1, &vbo)
	w.gl.GenBuffers(1, &ebo)

	w.gl.BindVertexArray(vao)
	w.gl.BindBuffer(glpkg.ArrayBuffer, vbo)
	w.gl.BufferData(
		glpkg.ArrayBuffer,
		len(vertices)*12*4,
		unsafe.Pointer(&vertices[0]),
		glpkg.StaticDraw,
	)

	w.gl.BindBuffer(glpkg.ElementArrayBuffer, ebo)
	w.gl.BufferData(
		glpkg.ElementArrayBuffer,
		len(indices)*4,
		unsafe.Pointer(&indices[0]),
		glpkg.StaticDraw,
	)

	posLoc := w.gl.GetAttribLocation(w.shader3DProgram, "a_position")
	normLoc := w.gl.GetAttribLocation(w.shader3DProgram, "a_normal")
	texLoc := w.gl.GetAttribLocation(w.shader3DProgram, "a_texCoord")
	colLoc := w.gl.GetAttribLocation(w.shader3DProgram, "a_color")
	w.gl.VertexAttribPointer(uint32(posLoc), 3, glpkg.Float, false, 12*4, 0)
	w.gl.EnableVertexAttribArray(uint32(posLoc))
	w.gl.VertexAttribPointer(uint32(normLoc), 3, glpkg.Float, false, 12*4, 12)
	w.gl.EnableVertexAttribArray(uint32(normLoc))
	w.gl.VertexAttribPointer(uint32(texLoc), 2, glpkg.Float, false, 12*4, 24)
	w.gl.EnableVertexAttribArray(uint32(texLoc))
	w.gl.VertexAttribPointer(uint32(colLoc), 4, glpkg.Float, false, 12*4, 32)
	w.gl.EnableVertexAttribArray(uint32(colLoc))

	m := &glMesh3D{
		vao:        vao,
		vbo:        vbo,
		ebo:        ebo,
		indexCount: int32(len(indices)),
		w:          w,
	}
	w.meshes3D = append(w.meshes3D, m)
	return m, nil
}

func (m *glDynamicMesh) UpdateVertices(offset int, vertices []Vertex) {
	if len(vertices) == 0 {
		return
	}
	m.w.gl.BindBuffer(glpkg.ArrayBuffer, m.vbo)
	m.w.gl.BufferSubData(
		glpkg.ArrayBuffer,
		offset*8*4, // offset in bytes
		len(vertices)*8*4,
		unsafe.Pointer(&vertices[0]),
	)
}

func (m *glDynamicMesh) UpdateAllVertices(vertices []Vertex) {
	if len(vertices) == 0 {
		return
	}
	m.w.gl.BindBuffer(glpkg.ArrayBuffer, m.vbo)
	m.w.gl.BufferSubData(
		glpkg.ArrayBuffer,
		0,
		len(vertices)*8*4,
		unsafe.Pointer(&vertices[0]),
	)
}

func (m *glDynamicMesh) UpdateIndices(indices []uint32) {
	if len(indices) == 0 {
		return
	}
	// Bind our VAO before EBO operations. In OpenGL, the ElementArrayBuffer
	// binding is stored in the VAO, so we must bind our VAO first to ensure
	// the EBO is associated with the correct VAO.
	m.w.gl.BindVertexArray(m.vao)
	m.w.gl.BindBuffer(glpkg.ElementArrayBuffer, m.ebo)
	m.w.gl.BufferSubData(
		glpkg.ElementArrayBuffer,
		0,
		len(indices)*4,
		unsafe.Pointer(&indices[0]),
	)
	m.indexCount = int32(len(indices))
}

func (m *glDynamicMesh) Resize(vertexCount, indexCount int) {
	// Recreate buffers with new capacity
	m.w.gl.BindBuffer(glpkg.ArrayBuffer, m.vbo)
	m.w.gl.BufferData(
		glpkg.ArrayBuffer,
		vertexCount*8*4,
		nil,
		glpkg.DynamicDraw,
	)

	m.w.gl.BindBuffer(glpkg.ElementArrayBuffer, m.ebo)
	m.w.gl.BufferData(
		glpkg.ElementArrayBuffer,
		indexCount*4,
		nil,
		glpkg.DynamicDraw,
	)

	m.vertexCap = vertexCount
	m.indexCount = 0 // Reset; set by UpdateIndices when indices are uploaded
}

func (m *glDynamicMesh) VertexCount() int {
	return m.vertexCap
}

func (m *glMesh) Destroy() {
	if m == nil || m.w == nil {
		return
	}
	w := m.w
	w.destroyMeshBuffers(&m.vao, &m.vbo, &m.ebo)
	w.removeMesh(m)
	m.indexCount = 0
	m.w = nil
}

func (m *glDynamicMesh) Destroy() {
	if m == nil || m.w == nil {
		return
	}
	w := m.w
	w.destroyMeshBuffers(&m.vao, &m.vbo, &m.ebo)
	w.removeMesh(m)
	m.indexCount = 0
	m.vertexCap = 0
	m.w = nil
}

func (m *glMesh3D) Destroy() {
	if m == nil || m.w == nil {
		return
	}
	w := m.w
	w.destroyMeshBuffers(&m.vao, &m.vbo, &m.ebo)
	w.removeMesh3D(m)
	m.indexCount = 0
	m.w = nil
}

func (w *glWindow) destroyMeshBuffers(vao, vbo, ebo *uint32) {
	if *vao != 0 {
		w.gl.DeleteVertexArrays(1, vao)
		*vao = 0
	}
	if *vbo != 0 {
		w.gl.DeleteBuffers(1, vbo)
		*vbo = 0
	}
	if *ebo != 0 {
		w.gl.DeleteBuffers(1, ebo)
		*ebo = 0
	}
}

func (w *glWindow) removeMesh3D(mesh Mesh3D) {
	for i, m := range w.meshes3D {
		if m != mesh {
			continue
		}
		copy(w.meshes3D[i:], w.meshes3D[i+1:])
		w.meshes3D[len(w.meshes3D)-1] = nil
		w.meshes3D = w.meshes3D[:len(w.meshes3D)-1]
		return
	}
}

func (w *glWindow) removeMesh(mesh Mesh) {
	for i, m := range w.meshes {
		if m != mesh {
			continue
		}
		copy(w.meshes[i:], w.meshes[i+1:])
		w.meshes[len(w.meshes)-1] = nil
		w.meshes = w.meshes[:len(w.meshes)-1]
		return
	}
}

func (f glFrame) RenderMesh(mesh Mesh, opts DrawOptions) {
	var vao uint32
	var indexCount int32
	var tex *glTexture

	switch m := mesh.(type) {
	case *glMesh:
		if m == nil || m.vao == 0 || m.indexCount == 0 {
			return
		}
		vao = m.vao
		indexCount = m.indexCount
		tex = m.tex
	case *glDynamicMesh:
		if m == nil || m.vao == 0 || m.indexCount == 0 {
			return
		}
		vao = m.vao
		indexCount = m.indexCount
		tex = m.tex
	default:
		return
	}

	f.w.gl.UseProgram(f.w.shaderProgram)

	// Projection is set in prepareFrame(); just set model.
	model := opts.Model
	if model == (Mat4{}) {
		model = IdentityMat4()
	}
	f.w.gl.UniformMatrix4fv(f.w.modelUniform, 1, false, &model[0])

	// Bind texture.
	f.w.gl.ActiveTexture(glpkg.Texture0)
	if tex != nil {
		f.w.gl.BindTexture(glpkg.Texture2D, tex.id)
	} else {
		t := f.w.getWhiteTexture()
		f.w.gl.BindTexture(glpkg.Texture2D, t.id)
	}
	f.w.gl.Uniform1i(f.w.textureUniform, 0)
	if mask, ok := opts.Mask.(*glTexture); ok && mask != nil {
		f.w.gl.ActiveTexture(glpkg.Texture1)
		f.w.gl.BindTexture(glpkg.Texture2D, mask.id)
		f.w.gl.Uniform1i(f.w.maskUniform, 1)
		f.w.gl.Uniform1i(f.w.useMaskUniform, 1)
	} else {
		f.w.gl.Uniform1i(f.w.useMaskUniform, 0)
	}

	// Draw.
	f.w.gl.BindVertexArray(vao)
	f.w.gl.DrawElements(glpkg.Triangles, indexCount, glpkg.UnsignedInt, 0)
	f.w.gl.Uniform1i(f.w.useMaskUniform, 0)
}

func (f glFrame) RenderMesh3D(mesh Mesh3D, opts Draw3DOptions) {
	m, ok := mesh.(*glMesh3D)
	if !ok || m == nil || m.vao == 0 || m.indexCount == 0 {
		return
	}
	model, view, projection, ambient, light := f.resolve3DOptions(opts)

	f.w.gl.Enable(glpkg.DepthTest)
	f.w.gl.UseProgram(f.w.shader3DProgram)
	f.w.gl.UniformMatrix4fv(f.w.model3DUniform, 1, false, &model[0])
	f.w.gl.UniformMatrix4fv(f.w.view3DUniform, 1, false, &view[0])
	f.w.gl.UniformMatrix4fv(f.w.projection3DUniform, 1, false, &projection[0])
	f.w.gl.Uniform4f(f.w.light3DUniform, light.X, light.Y, light.Z, 0)
	f.w.gl.Uniform1f(f.w.ambient3DUniform, ambient)
	f.w.gl.BindVertexArray(m.vao)
	f.w.gl.DrawElements(glpkg.Triangles, m.indexCount, glpkg.UnsignedInt, 0)

	f.w.gl.Disable(glpkg.DepthTest)
	f.w.setProjection(f.w.projectionW, f.w.projectionH)
}

func (f glFrame) resolve3DOptions(opts Draw3DOptions) (Mat4, Mat4, Mat4, float32, Vec3) {
	model := opts.Model
	if model == (Mat4{}) {
		model = IdentityMat4()
	}
	view := opts.View
	if view == (Mat4{}) {
		view = IdentityMat4()
	}
	projection := opts.Projection
	if projection == (Mat4{}) {
		aspect := float32(1)
		if f.w.projectionH != 0 {
			aspect = f.w.projectionW / f.w.projectionH
		}
		projection = PerspectiveMat4(float32(math.Pi/4), aspect, 0.01, 1000)
	}

	ambient := opts.Ambient
	if ambient == 0 {
		ambient = 0.22
	}
	light := opts.LightDirection
	if light == (Vec3{}) {
		light = Vec3{X: -0.35, Y: 0.6, Z: 0.7}
	}
	return model, view, projection, ambient, light
}

func (f glFrame) PushClip(rect Rect) {
	if len(f.w.clipStack) > 0 {
		rect = intersectRects(f.w.clipStack[len(f.w.clipStack)-1], rect)
	}
	f.w.clipStack = append(f.w.clipStack, rect)
	f.w.applyClip(rect)
}

func (f glFrame) PopClip() {
	if len(f.w.clipStack) == 0 {
		return
	}
	f.w.clipStack = f.w.clipStack[:len(f.w.clipStack)-1]
	if len(f.w.clipStack) == 0 {
		f.w.gl.Disable(glpkg.ScissorTest)
		return
	}
	f.w.applyClip(f.w.clipStack[len(f.w.clipStack)-1])
}

func (f glFrame) PushClipMesh(mesh Mesh, opts DrawOptions) {
	w := f.w
	if w.stencilDepth >= 8 {
		return
	}

	currentMask := uint32(0)
	if w.stencilDepth > 0 {
		currentMask = (1 << uint(w.stencilDepth)) - 1
	}
	nextBit := uint32(1 << uint(w.stencilDepth))
	nextMask := currentMask | nextBit

	if w.stencilDepth == 0 {
		w.gl.Enable(glpkg.StencilTest)
		w.gl.StencilMask(0xff)
		w.gl.ClearStencil(0)
		w.gl.Clear(glpkg.StencilBufferBit)
	}

	w.gl.ColorMask(false, false, false, false)
	w.gl.StencilMask(nextBit)
	w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Replace)
	if currentMask == 0 {
		w.gl.StencilFunc(glpkg.Always, int32(nextMask), 0xff)
	} else {
		w.gl.StencilFunc(glpkg.Equal, int32(nextMask), currentMask)
	}
	f.RenderMesh(mesh, opts)
	w.gl.ColorMask(true, true, true, true)
	w.stencilDepth++
	w.gl.StencilMask(0x00)
	w.gl.StencilFunc(glpkg.Equal, int32(nextMask), nextMask)
	w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Keep)
}

func (f glFrame) PopClipMesh() {
	w := f.w
	if w.stencilDepth == 0 {
		return
	}
	w.stencilDepth--
	if w.stencilDepth == 0 {
		w.gl.StencilMask(0xff)
		w.gl.ClearStencil(0)
		w.gl.Clear(glpkg.StencilBufferBit)
		w.gl.Disable(glpkg.StencilTest)
		return
	}
	currentMask := uint32((1 << uint(w.stencilDepth)) - 1)
	w.gl.StencilMask(0x00)
	w.gl.StencilFunc(glpkg.Equal, int32(currentMask), currentMask)
	w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Keep)
}

func (f glFrame) PushClipMesh3D(mesh Mesh3D, opts Draw3DOptions) {
	m, ok := mesh.(*glMesh3D)
	if !ok || m == nil || m.vao == 0 || m.indexCount == 0 {
		return
	}
	w := f.w
	if w.stencilDepth >= 8 {
		return
	}

	currentMask := uint32(0)
	if w.stencilDepth > 0 {
		currentMask = (1 << uint(w.stencilDepth)) - 1
	}
	nextBit := uint32(1 << uint(w.stencilDepth))
	nextMask := currentMask | nextBit

	if w.stencilDepth == 0 {
		w.gl.Enable(glpkg.StencilTest)
		w.gl.StencilMask(0xff)
		w.gl.ClearStencil(0)
		w.gl.Clear(glpkg.StencilBufferBit)
	}

	model, view, projection, ambient, light := f.resolve3DOptions(opts)
	if opts.ClipDepthTest {
		w.gl.Enable(glpkg.DepthTest)
	} else {
		w.gl.Disable(glpkg.DepthTest)
	}
	w.gl.ColorMask(false, false, false, false)
	w.gl.StencilMask(nextBit)
	w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Replace)
	if currentMask == 0 {
		w.gl.StencilFunc(glpkg.Always, int32(nextMask), 0xff)
	} else {
		w.gl.StencilFunc(glpkg.Equal, int32(nextMask), currentMask)
	}

	w.gl.UseProgram(w.shader3DProgram)
	w.gl.UniformMatrix4fv(w.model3DUniform, 1, false, &model[0])
	w.gl.UniformMatrix4fv(w.view3DUniform, 1, false, &view[0])
	w.gl.UniformMatrix4fv(w.projection3DUniform, 1, false, &projection[0])
	w.gl.Uniform4f(w.light3DUniform, light.X, light.Y, light.Z, 0)
	w.gl.Uniform1f(w.ambient3DUniform, ambient)
	w.gl.BindVertexArray(m.vao)
	w.gl.DrawElements(glpkg.Triangles, m.indexCount, glpkg.UnsignedInt, 0)

	w.gl.ColorMask(true, true, true, true)
	w.gl.Disable(glpkg.DepthTest)
	w.stencilDepth++
	w.gl.StencilMask(0x00)
	w.gl.StencilFunc(glpkg.Equal, int32(nextMask), nextMask)
	w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Keep)
	w.setProjection(w.projectionW, w.projectionH)
}

func (f glFrame) PopClipMesh3D() {
	f.PopClipMesh()
}

func (f glFrame) RenderToTarget(target RenderTarget, opts RenderTargetOptions, drawFn func(Frame) error) error {
	rt, ok := target.(*glRenderTarget)
	if !ok || rt == nil || rt.fbo == 0 {
		return fmt.Errorf("unsupported render target")
	}
	if rt.window != f.w {
		return fmt.Errorf("render target belongs to a different window")
	}
	if drawFn == nil {
		return nil
	}

	w := f.w
	savedClips := append([]Rect(nil), w.clipStack...)
	savedStencilDepth := w.stencilDepth

	rt.Bind()
	w.setProjection(float32(rt.width), float32(rt.height))
	w.resetFrameState()
	if !opts.NoClear {
		clear := opts.ClearColor
		if clear == nil {
			clear = color.RGBA{}
		}
		rgba := ColorToFloat32(clear)
		w.gl.ClearColor(rgba[0], rgba[1], rgba[2], rgba[3])
		w.gl.Clear(glpkg.ColorBufferBit)
	}

	err := drawFn(f)

	w.bindWindowFramebuffer()
	w.clipStack = savedClips
	if len(w.clipStack) == 0 {
		w.gl.Disable(glpkg.ScissorTest)
	} else {
		w.applyClip(w.clipStack[len(w.clipStack)-1])
	}
	w.stencilDepth = savedStencilDepth
	if w.stencilDepth == 0 {
		w.gl.Disable(glpkg.StencilTest)
	} else {
		mask := uint32((1 << uint(w.stencilDepth)) - 1)
		w.gl.Enable(glpkg.StencilTest)
		w.gl.StencilMask(0x00)
		w.gl.StencilFunc(glpkg.Equal, int32(mask), mask)
		w.gl.StencilOp(glpkg.Keep, glpkg.Keep, glpkg.Keep)
	}

	return err
}

func (w *glWindow) applyClip(rect Rect) {
	w.gl.Enable(glpkg.ScissorTest)

	bw, bh := w.platform.BackingSize()
	logicalH := float32(bh) / w.scale

	x := int32(math.Round(float64(rect.X * w.scale)))
	y := int32(math.Round(float64((logicalH - rect.Y - rect.Height) * w.scale)))
	width := int32(math.Round(float64(rect.Width * w.scale)))
	height := int32(math.Round(float64(rect.Height * w.scale)))

	if x < 0 {
		width += x
		x = 0
	}
	if y < 0 {
		height += y
		y = 0
	}
	if x+width > int32(bw) {
		width = int32(bw) - x
	}
	if y+height > int32(bh) {
		height = int32(bh) - y
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	w.gl.Scissor(x, y, width, height)
}

func intersectRects(a, b Rect) Rect {
	x1 := maxf(a.X, b.X)
	y1 := maxf(a.Y, b.Y)
	x2 := minf(a.X+a.Width, b.X+b.Width)
	y2 := minf(a.Y+a.Height, b.Y+b.Height)
	return Rect{X: x1, Y: y1, Width: maxf(0, x2-x1), Height: maxf(0, y2-y1)}
}

func (t *glTexture) Size() (int, int) {
	return t.w, t.h
}

func (w *glWindow) getWhiteTexture() *glTexture {
	if w.whiteTex != nil {
		return w.whiteTex
	}

	var texID uint32
	w.gl.GenTextures(1, &texID)
	w.gl.BindTexture(glpkg.Texture2D, texID)
	w.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMinFilter, glpkg.Nearest)
	w.gl.TexParameteri(glpkg.Texture2D, glpkg.TextureMagFilter, glpkg.Nearest)

	// 1x1 RGBA pixel (white).
	pix := [4]byte{0xff, 0xff, 0xff, 0xff}
	w.gl.TexImage2D(
		glpkg.Texture2D,
		0,
		int32(glpkg.RGBA),
		1,
		1,
		0,
		glpkg.RGBA,
		glpkg.UnsignedByte,
		unsafe.Pointer(&pix[0]),
	)

	w.whiteTex = &glTexture{id: texID, w: 1, h: 1}
	return w.whiteTex
}
