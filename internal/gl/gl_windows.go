//go:build windows

package gl

import (
	"math"
	"syscall"
	"unsafe"
)

type openGL struct {
	clearColor    *syscall.LazyProc
	clear         *syscall.LazyProc
	viewport      *syscall.LazyProc
	enable        *syscall.LazyProc
	disable       *syscall.LazyProc
	genTextures   *syscall.LazyProc
	bindTexture   *syscall.LazyProc
	texImage2D    *syscall.LazyProc
	texSubImage2D *syscall.LazyProc
	texParameteri *syscall.LazyProc
	pixelStorei   *syscall.LazyProc
	begin         *syscall.LazyProc
	end           *syscall.LazyProc
	color4fv      *syscall.LazyProc
	texCoord2f    *syscall.LazyProc
	vertex2f      *syscall.LazyProc
	ortho         *syscall.LazyProc
	matrixMode    *syscall.LazyProc
	loadIdentity  *syscall.LazyProc
	blendFunc     *syscall.LazyProc
	readPixels    *syscall.LazyProc
	getString     *syscall.LazyProc
}

func (gl *openGL) ClearColor(r, g, b, a float32) {
	gl.clearColor.Call(f32(r), f32(g), f32(b), f32(a))
}

func (gl *openGL) Clear(mask uint32) {
	gl.clear.Call(uintptr(mask))
}

func (gl *openGL) Viewport(x, y, width, height int32) {
	gl.viewport.Call(uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}

func (gl *openGL) Enable(cap uint32) {
	gl.enable.Call(uintptr(cap))
}

func (gl *openGL) Disable(cap uint32) {
	gl.disable.Call(uintptr(cap))
}

func (gl *openGL) GenTextures(n int32, textures *uint32) {
	gl.genTextures.Call(uintptr(n), uintptr(unsafe.Pointer(textures)))
}

func (gl *openGL) BindTexture(target, texture uint32) {
	gl.bindTexture.Call(uintptr(target), uintptr(texture))
}

func (gl *openGL) TexImage2D(target uint32, level, internalFormat, width, height, border int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.texImage2D.Call(uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(width), uintptr(height), uintptr(border), uintptr(format), uintptr(xtype), uintptr(pixels))
}

func (gl *openGL) TexSubImage2D(target uint32, level, xoffset, yoffset, width, height int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.texSubImage2D.Call(uintptr(target), uintptr(level), uintptr(xoffset), uintptr(yoffset), uintptr(width), uintptr(height), uintptr(format), uintptr(xtype), uintptr(pixels))
}

func (gl *openGL) TexParameteri(target, pname uint32, param int32) {
	gl.texParameteri.Call(uintptr(target), uintptr(pname), uintptr(param))
}

func (gl *openGL) PixelStorei(pname uint32, param int32) {
	gl.pixelStorei.Call(uintptr(pname), uintptr(param))
}

func (gl *openGL) Begin(mode uint32) {
	gl.begin.Call(uintptr(mode))
}

func (gl *openGL) End() {
	gl.end.Call()
}

func (gl *openGL) Color4fv(v *float32) {
	gl.color4fv.Call(uintptr(unsafe.Pointer(v)))
}

func (gl *openGL) TexCoord2f(s, t float32) {
	gl.texCoord2f.Call(f32(s), f32(t))
}

func (gl *openGL) Vertex2f(x, y float32) {
	gl.vertex2f.Call(f32(x), f32(y))
}

func (gl *openGL) Ortho(left, right, bottom, top, zNear, zFar float64) {
	gl.ortho.Call(f64(left), f64(right), f64(bottom), f64(top), f64(zNear), f64(zFar))
}

func (gl *openGL) MatrixMode(mode uint32) {
	gl.matrixMode.Call(uintptr(mode))
}

func (gl *openGL) LoadIdentity() {
	gl.loadIdentity.Call()
}

func (gl *openGL) BlendFunc(sfactor, dfactor uint32) {
	gl.blendFunc.Call(uintptr(sfactor), uintptr(dfactor))
}

func (gl *openGL) ReadPixels(x, y, width, height int32, format, xtype uint32, pixels unsafe.Pointer) {
	gl.readPixels.Call(uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(xtype), uintptr(pixels))
}

func (gl *openGL) GetString(name uint32) string {
	ptr, _, _ := gl.getString.Call(uintptr(name))
	return gostring((*byte)(unsafe.Pointer(ptr)))
}

func Load() (OpenGL, error) {
	opengl32 := syscall.NewLazyDLL("opengl32.dll")
	gl := &openGL{
		clearColor:    opengl32.NewProc("glClearColor"),
		clear:         opengl32.NewProc("glClear"),
		viewport:      opengl32.NewProc("glViewport"),
		enable:        opengl32.NewProc("glEnable"),
		disable:       opengl32.NewProc("glDisable"),
		genTextures:   opengl32.NewProc("glGenTextures"),
		bindTexture:   opengl32.NewProc("glBindTexture"),
		texImage2D:    opengl32.NewProc("glTexImage2D"),
		texSubImage2D: opengl32.NewProc("glTexSubImage2D"),
		texParameteri: opengl32.NewProc("glTexParameteri"),
		pixelStorei:   opengl32.NewProc("glPixelStorei"),
		begin:         opengl32.NewProc("glBegin"),
		end:           opengl32.NewProc("glEnd"),
		color4fv:      opengl32.NewProc("glColor4fv"),
		texCoord2f:    opengl32.NewProc("glTexCoord2f"),
		vertex2f:      opengl32.NewProc("glVertex2f"),
		ortho:         opengl32.NewProc("glOrtho"),
		matrixMode:    opengl32.NewProc("glMatrixMode"),
		loadIdentity:  opengl32.NewProc("glLoadIdentity"),
		blendFunc:     opengl32.NewProc("glBlendFunc"),
		readPixels:    opengl32.NewProc("glReadPixels"),
		getString:     opengl32.NewProc("glGetString"),
	}
	return gl, nil
}

func f32(v float32) uintptr {
	return uintptr(math.Float32bits(v))
}

func f64(v float64) uintptr {
	return uintptr(math.Float64bits(v))
}
