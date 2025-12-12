//go:build windows

package window

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/tinyrange/gowin/internal/gl"
)

const (
	csOwnDC   = 0x0020
	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsOverlappedWindow = 0x00CF0000
	swShow             = 5

	wmClose       = 0x0010
	wmDestroy     = 0x0002
	pmRemove      = 0x0001
	swpShowWindow = 0x0040

	pfdTypeRGBA      = 0
	pfdMainPlane     = 0
	pfdDrawToWindow  = 0x00000004
	pfdSupportOpenGL = 0x00000020
	pfdDoubleBuffer  = 0x00000001
)

type (
	hwnd  = syscall.Handle
	hdc   = syscall.Handle
	hglrc = syscall.Handle
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type msg struct {
	hwnd     hwnd
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type point struct {
	x int32
	y int32
}

type rect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

type pixelFormatDescriptor struct {
	nSize          uint16
	nVersion       uint16
	dwFlags        uint32
	iPixelType     byte
	colorBits      byte
	redBits        byte
	redShift       byte
	greenBits      byte
	greenShift     byte
	blueBits       byte
	blueShift      byte
	alphaBits      byte
	alphaShift     byte
	accumBits      byte
	accumRedBits   byte
	accumGreenBits byte
	accumBlueBits  byte
	accumAlphaBits byte
	depthBits      byte
	stencilBits    byte
	auxBuffers     byte
	layerType      byte
	bReserved      byte
	dwLayerMask    uint32
	dwVisibleMask  uint32
	dwDamageMask   uint32
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	opengl32 = syscall.NewLazyDLL("opengl32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassEx  = user32.NewProc("RegisterClassExW")
	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
	procDefWindowProc    = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
	procGetClientRect    = user32.NewProc("GetClientRect")
	procPeekMessage      = user32.NewProc("PeekMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procScreenToClient   = user32.NewProc("ScreenToClient")
	procUpdateWindow     = user32.NewProc("UpdateWindow")

	procChoosePixelFormat = gdi32.NewProc("ChoosePixelFormat")
	procSetPixelFormat    = gdi32.NewProc("SetPixelFormat")
	procSwapBuffers       = gdi32.NewProc("SwapBuffers")

	procWglCreateContext = opengl32.NewProc("wglCreateContext")
	procWglMakeCurrent   = opengl32.NewProc("wglMakeCurrent")
	procWglDeleteContext = opengl32.NewProc("wglDeleteContext")

	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

var (
	windowClass = syscall.StringToUTF16Ptr("GoWin32Window")
	currentWin  *winWindow
)

type winWindow struct {
	hwnd    hwnd
	hdc     hdc
	ctx     hglrc
	running bool
}

func New(title string, width, height int, _ bool) (Window, error) {
	runtime.LockOSThread()

	if err := registerWindowClass(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}

	hwnd, hdc, err := createWindow(title, width, height)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}

	if err := setupPixelFormat(hdc); err != nil {
		user32.NewProc("DestroyWindow").Call(uintptr(hwnd))
		runtime.UnlockOSThread()
		return nil, err
	}

	ctx, err := createGLContext(hdc)
	if err != nil {
		user32.NewProc("DestroyWindow").Call(uintptr(hwnd))
		runtime.UnlockOSThread()
		return nil, err
	}

	win := &winWindow{hwnd: hwnd, hdc: hdc, ctx: ctx, running: true}
	currentWin = win

	return win, nil
}

func (w *winWindow) GL() (gl.OpenGL, error) {
	return gl.Load()
}

func (w *winWindow) Close() {
	if w.ctx != 0 {
		procWglMakeCurrent.Call(uintptr(w.hdc), 0)
		procWglDeleteContext.Call(uintptr(w.ctx))
		w.ctx = 0
	}
	if w.hdc != 0 && w.hwnd != 0 {
		procReleaseDC.Call(uintptr(w.hwnd), uintptr(w.hdc))
		w.hdc = 0
	}
	if w.hwnd != 0 {
		procDestroyWindow.Call(uintptr(w.hwnd))
		w.hwnd = 0
	}
	w.running = false
	runtime.UnlockOSThread()
}

func (w *winWindow) Poll() bool {
	if !w.running {
		return false
	}

	var m msg
	for {
		ret, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
		if ret == 0 {
			break
		}
		if m.message == wmDestroy {
			w.running = false
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	return w.running
}

func (w *winWindow) Swap() {
	if w.hdc != 0 {
		procSwapBuffers.Call(uintptr(w.hdc))
	}
}

func (w *winWindow) BackingSize() (int, int) {
	var r rect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r)))
	return int(r.right - r.left), int(r.bottom - r.top)
}

func (w *winWindow) Cursor() (float32, float32) {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if p.x != 0 || p.y != 0 {
		procScreenToClient.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&p)))
	}
	return float32(p.x), float32(p.y)
}

func registerWindowClass() error {
	cb := syscall.NewCallback(wndProc)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		style:         csOwnDC | csHRedraw | csVRedraw,
		lpfnWndProc:   cb,
		hInstance:     moduleHandle(),
		hCursor:       loadCursor(),
		lpszClassName: windowClass,
	}

	ret, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		// If the class already exists, RegisterClassEx returns 0 with ERROR_CLASS_ALREADY_EXISTS.
		if errno, ok := err.(syscall.Errno); ok && errno == 1410 {
			return nil
		}
		return err
	}
	return nil
}

func createWindow(title string, width, height int) (win hwnd, dc hdc, err error) {
	win = createNativeWindow(title, width, height)
	if win == 0 {
		return 0, 0, errors.New("CreateWindowExW failed")
	}
	procShowWindow.Call(uintptr(win), swShow)
	procUpdateWindow.Call(uintptr(win))

	ret, _, _ := procGetDC.Call(uintptr(win))
	if ret == 0 {
		procDestroyWindow.Call(uintptr(win))
		return 0, 0, errors.New("GetDC failed")
	}
	return win, hdc(ret), nil
}

func createNativeWindow(title string, width, height int) hwnd {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ret, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(windowClass)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsOverlappedWindow,
		0x80000000, 0x80000000, // CW_USEDEFAULT
		uintptr(width), uintptr(height),
		0, 0, uintptr(moduleHandle()), 0,
	)
	return hwnd(ret)
}

func setupPixelFormat(hdc hdc) error {
	var pfd pixelFormatDescriptor
	pfd.nSize = uint16(unsafe.Sizeof(pfd))
	pfd.nVersion = 1
	pfd.dwFlags = pfdDrawToWindow | pfdSupportOpenGL | pfdDoubleBuffer
	pfd.iPixelType = pfdTypeRGBA
	pfd.colorBits = 24
	pfd.depthBits = 24
	pfd.layerType = pfdMainPlane

	format, _, _ := procChoosePixelFormat.Call(uintptr(hdc), uintptr(unsafe.Pointer(&pfd)))
	if format == 0 {
		return errors.New("ChoosePixelFormat failed")
	}

	ok, _, _ := procSetPixelFormat.Call(uintptr(hdc), format, uintptr(unsafe.Pointer(&pfd)))
	if ok == 0 {
		return errors.New("SetPixelFormat failed")
	}
	return nil
}

func createGLContext(hdc hdc) (hglrc, error) {
	ctx, _, _ := procWglCreateContext.Call(uintptr(hdc))
	if ctx == 0 {
		return 0, errors.New("wglCreateContext failed")
	}
	if ret, _, _ := procWglMakeCurrent.Call(uintptr(hdc), ctx); ret == 0 {
		procWglDeleteContext.Call(ctx)
		return 0, errors.New("wglMakeCurrent failed")
	}
	return hglrc(ctx), nil
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmClose:
		current := currentWin
		if current != nil && current.hwnd == syscall.Handle(hwnd) {
			current.running = false
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return ret
}

func loadCursor() syscall.Handle {
	loadCursorProc := user32.NewProc("LoadCursorW")
	const idcArrow = 32512
	ret, _, _ := loadCursorProc.Call(0, uintptr(idcArrow))
	return syscall.Handle(ret)
}

func moduleHandle() syscall.Handle {
	h, _, _ := procGetModuleHandle.Call(0)
	return syscall.Handle(h)
}
