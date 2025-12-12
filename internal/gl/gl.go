package gl

import "unsafe"

const (
	ColorBufferBit = 0x00004000

	Texture2D        = 0x0DE1
	UnpackAlignment  = 0x0CF5
	Intensity        = 0x8049
	TextureMinFilter = 0x2801
	TextureMagFilter = 0x2800
	Nearest          = 0x2600
	Linear           = 0x2601
	Alpha            = 0x1906
	RGBA             = 0x1908
	UnsignedByte     = 0x1401
	Quads            = 0x0007
	TriangleStrip    = 0x0005
	Projection       = 0x1701
	ModelView        = 0x1700

	// Blending
	Blend            = 0x0BE2
	SrcAlpha         = 0x0302
	OneMinusSrcAlpha = 0x0303

	// Texture formats
	LuminanceAlpha = 0x190A
)

type OpenGL interface {
	ClearColor(float32, float32, float32, float32)
	Clear(uint32)
	Viewport(int32, int32, int32, int32)
	Enable(uint32)
	Disable(uint32)
	GenTextures(int32, *uint32)
	BindTexture(uint32, uint32)
	TexImage2D(uint32, int32, int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
	TexSubImage2D(uint32, int32, int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
	TexParameteri(uint32, uint32, int32)
	PixelStorei(uint32, int32)
	Begin(uint32)
	End()
	Color4fv(*float32)
	TexCoord2f(float32, float32)
	Vertex2f(float32, float32)
	Ortho(float64, float64, float64, float64, float64, float64)
	MatrixMode(uint32)
	LoadIdentity()
	BlendFunc(uint32, uint32)
	ReadPixels(int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
}
