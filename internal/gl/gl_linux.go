//go:build linux

package gl

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// The Linux loader binds the fixed-function OpenGL 1.x entry points exposed by libGL.
type openGL struct {
	clearColor    func(float32, float32, float32, float32)
	clear         func(uint32)
	viewport      func(int32, int32, int32, int32)
	enable        func(uint32)
	disable       func(uint32)
	genTextures   func(int32, *uint32)
	bindTexture   func(uint32, uint32)
	texImage2D    func(uint32, int32, int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
	texSubImage2D func(uint32, int32, int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
	texParameteri func(uint32, uint32, int32)
	pixelStorei   func(uint32, int32)
	begin         func(uint32)
	end           func()
	color4fv      func(*float32)
	texCoord2f    func(float32, float32)
	vertex2f      func(float32, float32)
	ortho         func(float64, float64, float64, float64, float64, float64)
	matrixMode    func(uint32)
	loadIdentity  func()
	blendFunc     func(uint32, uint32)
	readPixels    func(int32, int32, int32, int32, uint32, uint32, unsafe.Pointer)
}

func (gl *openGL) ClearColor(r, g, b, a float32) {
	gl.clearColor(r, g, b, a)
}

func (gl *openGL) Clear(mask uint32) {
	gl.clear(mask)
}

func (gl *openGL) Viewport(x, y, width, height int32) {
	gl.viewport(x, y, width, height)
}

func (gl *openGL) Enable(cap uint32) {
	gl.enable(cap)
}

func (gl *openGL) Disable(cap uint32) {
	gl.disable(cap)
}

func (gl *openGL) GenTextures(n int32, textures *uint32) {
	gl.genTextures(n, textures)
}

func (gl *openGL) BindTexture(target, texture uint32) {
	gl.bindTexture(target, texture)
}

func (gl *openGL) TexImage2D(target uint32, level, internalFormat, width, height, border int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.texImage2D(target, level, internalFormat, width, height, border, format, xtype, pixels)
}

func (gl *openGL) TexSubImage2D(target uint32, level, xoffset, yoffset, width, height int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.texSubImage2D(target, level, xoffset, yoffset, width, height, format, xtype, pixels)
}

func (gl *openGL) TexParameteri(target, pname uint32, param int32) {
	gl.texParameteri(target, pname, param)
}

func (gl *openGL) PixelStorei(pname uint32, param int32) {
	gl.pixelStorei(pname, param)
}

func (gl *openGL) Begin(mode uint32) {
	gl.begin(mode)
}

func (gl *openGL) End() {
	gl.end()
}

func (gl *openGL) Color4fv(v *float32) {
	gl.color4fv(v)
}

func (gl *openGL) TexCoord2f(s, t float32) {
	gl.texCoord2f(s, t)
}

func (gl *openGL) Vertex2f(x, y float32) {
	gl.vertex2f(x, y)
}

func (gl *openGL) Ortho(left, right, bottom, top, zNear, zFar float64) {
	gl.ortho(left, right, bottom, top, zNear, zFar)
}

func (gl *openGL) MatrixMode(mode uint32) {
	gl.matrixMode(mode)
}

func (gl *openGL) LoadIdentity() {
	gl.loadIdentity()
}

func (gl *openGL) BlendFunc(sfactor, dfactor uint32) {
	gl.blendFunc(sfactor, dfactor)
}

func (gl *openGL) ReadPixels(x, y, width, height int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.readPixels(x, y, width, height, format, xtype, pixels)
}

func Load() (OpenGL, error) {
	handle, err := purego.Dlopen("libGL.so.1", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	register := func(dst interface{}, name string) {
		purego.RegisterLibFunc(dst, handle, name)
	}

	gl := &openGL{}
	register(&gl.clearColor, "glClearColor")
	register(&gl.clear, "glClear")
	register(&gl.viewport, "glViewport")
	register(&gl.enable, "glEnable")
	register(&gl.disable, "glDisable")
	register(&gl.genTextures, "glGenTextures")
	register(&gl.bindTexture, "glBindTexture")
	register(&gl.texImage2D, "glTexImage2D")
	register(&gl.texSubImage2D, "glTexSubImage2D")
	register(&gl.texParameteri, "glTexParameteri")
	register(&gl.pixelStorei, "glPixelStorei")
	register(&gl.begin, "glBegin")
	register(&gl.end, "glEnd")
	register(&gl.color4fv, "glColor4fv")
	register(&gl.texCoord2f, "glTexCoord2f")
	register(&gl.vertex2f, "glVertex2f")
	register(&gl.ortho, "glOrtho")
	register(&gl.matrixMode, "glMatrixMode")
	register(&gl.loadIdentity, "glLoadIdentity")
	register(&gl.blendFunc, "glBlendFunc")
	register(&gl.readPixels, "glReadPixels")
	return gl, nil
}
