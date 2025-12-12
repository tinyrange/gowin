package gl

import "unsafe"

const (
	// ColorBufferBit is a mask used with Clear to clear the color buffer.
	ColorBufferBit = 0x00004000

	// Texture2D is the texture target for 2D textures.
	Texture2D = 0x0DE1

	// UnpackAlignment specifies the alignment requirements for pixel data
	// when uploading textures (PixelStorei).
	UnpackAlignment = 0x0CF5

	// Intensity is a legacy internal texture format.
	Intensity = 0x8049

	// TextureWrapS selects the wrapping function for texture coordinate S.
	TextureWrapS = 0x2802
	// TextureWrapT selects the wrapping function for texture coordinate T.
	TextureWrapT = 0x2803

	// TextureMinFilter selects the texture minification filter.
	TextureMinFilter = 0x2801
	// TextureMagFilter selects the texture magnification filter.
	TextureMagFilter = 0x2800

	// Nearest selects nearest-neighbor filtering.
	Nearest = 0x2600
	// Linear selects linear filtering.
	Linear = 0x2601

	// ClampToEdge clamps texture coordinates to the edge of the texture.
	ClampToEdge = 0x812F

	// Alpha is a legacy pixel format representing alpha only.
	Alpha = 0x1906
	// RGBA is a pixel format representing red/green/blue/alpha.
	RGBA = 0x1908

	// UnsignedByte is a pixel data type indicating 8-bit unsigned values.
	UnsignedByte = 0x1401

	// Quads is a legacy primitive type for drawing quadrilaterals.
	Quads = 0x0007
	// TriangleStrip is a primitive type for drawing a connected strip of triangles.
	TriangleStrip = 0x0005

	// Projection selects the projection matrix stack for MatrixMode.
	Projection = 0x1701
	// ModelView selects the model-view matrix stack for MatrixMode.
	ModelView = 0x1700

	// Blending capabilities and factors.
	Blend            = 0x0BE2
	SrcAlpha         = 0x0302
	OneMinusSrcAlpha = 0x0303

	// Texture formats.
	LuminanceAlpha = 0x190A

	// GetString parameters.
	//
	// Vendor returns the company responsible for the GL implementation.
	Vendor = 0x1F00
	// Version returns the GL version string of the current context.
	Version = 0x1F02
)

// OpenGL describes the subset of OpenGL entry points used by this package.
//
// Implementations typically wrap platform-specific GL bindings. All methods are
// expected to operate on the currently current GL context for the calling thread.
type OpenGL interface {
	// ClearColor sets the clear color used by Clear when clearing the color buffer.
	ClearColor(r, g, b, a float32)

	// Clear clears buffers to preset values (e.g., ColorBufferBit).
	Clear(mask uint32)

	// Viewport sets the affine transformation of x and y from normalized device
	// coordinates to window coordinates.
	Viewport(x, y, width, height int32)

	// Enable enables a server-side GL capability (e.g., Blend).
	Enable(cap uint32)

	// Disable disables a server-side GL capability.
	Disable(cap uint32)

	// GenTextures generates texture object names.
	GenTextures(n int32, textures *uint32)

	// BindTexture binds a named texture to a texturing target (e.g., Texture2D).
	BindTexture(target, texture uint32)

	// TexImage2D specifies a two-dimensional texture image.
	//
	// The pixels pointer may be nil to allocate storage without uploading data.
	TexImage2D(
		target uint32,
		level int32,
		internalformat int32,
		width int32,
		height int32,
		border int32,
		format uint32,
		xtype uint32,
		pixels unsafe.Pointer,
	)

	// TexSubImage2D specifies a sub-region of an existing two-dimensional texture image.
	TexSubImage2D(
		target uint32,
		level int32,
		xoffset int32,
		yoffset int32,
		width int32,
		height int32,
		format uint32,
		xtype uint32,
		pixels unsafe.Pointer,
	)

	// TexParameteri sets texture parameters for the currently bound texture.
	TexParameteri(target, pname uint32, param int32)

	// PixelStorei sets pixel storage modes (e.g., UnpackAlignment).
	PixelStorei(pname uint32, param int32)

	// Begin begins specifying vertices for a primitive or a group of like primitives.
	//
	// This is part of OpenGL's legacy immediate mode API.
	Begin(mode uint32)

	// End marks the end of vertex specification started by Begin.
	End()

	// Color4fv sets the current color using a pointer to four float32 values.
	Color4fv(v *float32)

	// TexCoord2f sets the current texture coordinates.
	TexCoord2f(s, t float32)

	// Vertex2f specifies a vertex.
	Vertex2f(x, y float32)

	// Ortho multiplies the current matrix by an orthographic projection matrix.
	Ortho(x1, x2, y1, y2, near, far float64)

	// MatrixMode sets which matrix stack is the target for subsequent matrix operations
	// (e.g., Projection or ModelView).
	MatrixMode(mode uint32)

	// LoadIdentity replaces the current matrix with the identity matrix.
	LoadIdentity()

	// BlendFunc specifies the pixel arithmetic for blending (e.g., SrcAlpha and OneMinusSrcAlpha).
	BlendFunc(sfactor, dfactor uint32)

	// ReadPixels reads a block of pixels from the framebuffer into client memory.
	ReadPixels(
		x int32,
		y int32,
		width int32,
		height int32,
		format uint32,
		xtype uint32,
		pixels unsafe.Pointer,
	)

	// GetString returns a string describing a GL property for the current context.
	//
	// Common names are Vendor and Version.
	// If the name is not recognized or no context is current, implementations may
	// return the empty string.
	GetString(name uint32) string
}

func gostring(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	var bytes []byte
	for p := ptr; *p != 0; p = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 1)) {
		bytes = append(bytes, *p)
	}
	return string(bytes)
}
