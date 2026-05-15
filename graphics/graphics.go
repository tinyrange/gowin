package graphics

import (
	"image"
	"image/color"

	"github.com/tinyrange/gowin/window"
)

// ColorToFloat32 converts a color.Color to RGBA float32 values in the range [0, 1].
func ColorToFloat32(c color.Color) [4]float32 {
	r, g, b, a := c.RGBA()
	// RGBA() returns values in range [0, 0xffff], convert to [0, 1]
	return [4]float32{
		float32(r) / 0xffff,
		float32(g) / 0xffff,
		float32(b) / 0xffff,
		float32(a) / 0xffff,
	}
}

// Default colors using image/color types
var (
	ColorBlack     = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	ColorWhite     = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	ColorRed       = color.RGBA{R: 255, G: 0, B: 0, A: 255}
	ColorGreen     = color.RGBA{R: 0, G: 255, B: 0, A: 255}
	ColorBlue      = color.RGBA{R: 0, G: 0, B: 255, A: 255}
	ColorYellow    = color.RGBA{R: 255, G: 255, B: 0, A: 255}
	ColorCyan      = color.RGBA{R: 0, G: 255, B: 255, A: 255}
	ColorMagenta   = color.RGBA{R: 255, G: 0, B: 255, A: 255}
	ColorGray      = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	ColorDarkGray  = color.RGBA{R: 64, G: 64, B: 64, A: 255}
	ColorLightGray = color.RGBA{R: 192, G: 192, B: 192, A: 255}
)

type Frame interface {
	// WindowSize returns the current logical window size. All Frame drawing
	// coordinates use this logical top-left origin coordinate space.
	WindowSize() (width, height int)
	// BackingSize returns the current physical framebuffer size in backing
	// pixels. On HiDPI displays this is typically WindowSize multiplied by Scale.
	BackingSize() (width, height int)
	// Scale returns the current backing-pixels-per-logical-pixel factor for this
	// frame. It can change when a window moves between displays.
	Scale() float32
	// CursorPos returns the cursor position in logical Frame coordinates.
	CursorPos() (x, y float32)

	GetKeyState(key window.Key) window.KeyState
	GetButtonState(button window.Button) window.ButtonState
	// TextInput returns the UTF-8 text entered since the last call to TextInput.
	TextInput() string

	RenderQuad(x, y, width, height float32, tex Texture, color color.Color)
	// RenderFBOTexture renders an FBO texture with flipped V coordinates.
	// Use this for textures created by render-to-texture operations (like blur).
	RenderFBOTexture(x, y, width, height float32, tex Texture, color color.Color)
	// RenderMaskedQuad renders tex modulated by mask alpha. Mask and texture use
	// the same quad UVs; render masks into a RenderTarget and pass its Texture.
	RenderMaskedQuad(x, y, width, height float32, tex Texture, mask Texture, color color.Color)
	RenderMesh(mesh Mesh, opts DrawOptions)
	// RenderMesh3D draws a colored, lit 3D mesh. It temporarily enables depth
	// testing, then restores the 2D render state so UI/text can be drawn before
	// or after 3D content in the same frame.
	RenderMesh3D(mesh Mesh3D, opts Draw3DOptions)

	// PushClip intersects subsequent rendering with rect until PopClip is called.
	// Coordinates are logical pixels in the same top-left origin space as drawing.
	PushClip(rect Rect)
	// PopClip restores the previous clipping rectangle.
	PopClip()
	// PushClipMesh intersects subsequent rendering with the filled area of mesh
	// using the stencil buffer. Use meshes tessellated from clip paths.
	PushClipMesh(mesh Mesh, opts DrawOptions)
	// PopClipMesh restores the previous stencil clip mesh.
	PopClipMesh()
	// PushClipMesh3D intersects subsequent 3D and 2D rendering with the projected
	// stencil footprint of mesh. By default stencil writing ignores depth so it
	// can be pushed before or after drawing the clipped surface. Set
	// Draw3DOptions.ClipDepthTest for volume/depth-sensitive clip meshes.
	PushClipMesh3D(mesh Mesh3D, opts Draw3DOptions)
	// PopClipMesh3D restores the previous stencil clip mesh.
	PopClipMesh3D()

	// RenderToTarget renders into target using a top-left origin projection whose
	// logical size matches target.Size, then restores window rendering.
	RenderToTarget(target RenderTarget, opts RenderTargetOptions, draw func(Frame) error) error

	// Screenshot returns the current framebuffer in backing pixels. For logical
	// size UI test images, use ScreenshotLogical.
	Screenshot() (image.Image, error)
	// ScreenshotLogical returns a screenshot resampled to WindowSize.
	ScreenshotLogical() (image.Image, error)
}

type Texture interface {
	Size() (width, height int)
}

// RenderTargetOptions controls Frame.RenderToTarget.
type RenderTargetOptions struct {
	// NoClear leaves the previous render target contents intact. By default the
	// target is cleared before drawing.
	NoClear bool
	// ClearColor is used when clearing. Nil means transparent black, which is the
	// usual starting point for vector mask textures.
	ClearColor color.Color
}

type Window interface {
	// Return the platform-specific window implementation.
	PlatformWindow() window.Window

	// Create a new texture from an image.
	NewTexture(image.Image) (Texture, error)
	// NewMesh uploads a set of vertices/indices to the GPU for repeated rendering.
	NewMesh(vertices []Vertex, indices []uint32, tex Texture) (Mesh, error)
	// NewDynamicMesh creates a mesh that supports efficient partial vertex updates.
	NewDynamicMesh(maxVertices, maxIndices int, tex Texture) (DynamicMesh, error)
	// NewMesh3D uploads 3D vertices/indices for repeated lit rendering. 3D uses
	// a conventional right-handed coordinate system; vertex winding is
	// counter-clockwise when viewed from the front face.
	NewMesh3D(vertices []Vertex3D, indices []uint32) (Mesh3D, error)
	// NewShader3D compiles a shader program that can draw Mesh3D resources.
	//
	// Custom 3D shaders use the same vertex layout as Mesh3D:
	// a_position vec3, a_normal vec3, a_texCoord vec2, and a_color vec4.
	// They may declare u_model, u_view, u_projection, u_lightDirection, and
	// u_ambient uniforms to receive values from Draw3DOptions.
	NewShader3D(vertexSource, fragmentSource string) (Shader3D, error)

	SetClear(enabled bool)
	SetClearColor(color color.Color)

	// Scale returns the most recently observed display scaling factor. During a
	// frame, prefer Frame.Scale so apps see the same scale used for drawing.
	Scale() float32

	// Call f for each frame until it returns an error.
	Loop(func(f Frame) error) error

	// GetShaderProgram returns the graphics shader program ID for state restoration.
	GetShaderProgram() uint32

	// NewRenderTarget creates an off-screen render target for render-to-texture.
	NewRenderTarget(width, height int) (RenderTarget, error)
}

// DynamicMesh supports efficient partial vertex updates via BufferSubData.
type DynamicMesh interface {
	Mesh
	// UpdateVertices updates a range of vertices starting at the given offset.
	UpdateVertices(offset int, vertices []Vertex)
	// UpdateAllVertices updates the entire vertex buffer.
	UpdateAllVertices(vertices []Vertex)
	// UpdateIndices updates the index buffer.
	UpdateIndices(indices []uint32)
	// Resize changes the buffer capacity (recreates GPU buffers).
	Resize(vertexCount, indexCount int)
	// VertexCount returns the current vertex capacity.
	VertexCount() int
}

// Each platform implements a New() method to return a Window.
