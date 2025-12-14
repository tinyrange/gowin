//go:build linux

package window

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tinyrange/gowin/internal/gl"
)

const (
	glxRGBA         = 4
	glxDoubleBuffer = 5
	glxDepthSize    = 12
	glxNone         = 0

	inputOutput = 1

	exposureMask        = 1 << 15
	structureNotifyMask = 1 << 17
	keyPressMask        = 1 << 0
	keyReleaseMask      = 1 << 1
	buttonPressMask     = 1 << 2
	buttonReleaseMask   = 1 << 3
	pointerMotionMask   = 1 << 6

	clientMessage = 33
	destroyNotify = 17
)

type XVisualInfo struct {
	Visual       uintptr
	VisualID     uint
	Screen       int32
	Depth        int32
	Class        int32
	RedMask      uint64
	GreenMask    uint64
	BlueMask     uint64
	ColormapSize int32
	BitsPerRGB   int32
	MapEntries   int32
	pad          int32
}

type xclientMessage struct {
	Type        int32
	Serial      uint64
	SendEvent   int32
	Display     uintptr
	Window      uintptr
	MessageType uintptr
	Format      int32
	Data        [5]uint64
}

var (
	x11lib uintptr
	gllib  uintptr

	xOpenDisplay           func(*byte) uintptr
	xDefaultScreen         func(uintptr) int32
	xRootWindow            func(uintptr, int32) uintptr
	xCreateColormap        func(uintptr, uintptr, uintptr, int32) uintptr
	xCreateWindow          func(uintptr, uintptr, int32, int32, uint32, uint32, uint32, int32, uint32, uintptr, uint64, unsafe.Pointer) uintptr
	xMapWindow             func(uintptr, uintptr) int32
	xStoreName             func(uintptr, uintptr, *byte) int32
	xInternAtom            func(uintptr, *byte, int32) uintptr
	xSetWMProtocols        func(uintptr, uintptr, *uintptr, int32) int32
	xSelectInput           func(uintptr, uintptr, int64)
	xPending               func(uintptr) int32
	xNextEvent             func(uintptr, unsafe.Pointer)
	xGetGeometry           func(uintptr, uintptr, *uintptr, *int32, *int32, *uint32, *uint32, *uint32, *uint32) int32
	xDestroyWindow         func(uintptr, uintptr) int32
	xCloseDisplay          func(uintptr) int32
	xQueryPointer          func(uintptr, uintptr, *uintptr, *uintptr, *int32, *int32, *int32, *int32, *uint32) int32
	xDisplayWidth          func(uintptr, int32) int32
	xDisplayWidthMM        func(uintptr, int32) int32
	xDisplayHeight         func(uintptr, int32) int32
	xDisplayHeightMM       func(uintptr, int32) int32
	xResourceManagerString func(uintptr) *byte

	glxChooseVisual   func(uintptr, int32, *int32) *XVisualInfo
	glxCreateContext  func(uintptr, *XVisualInfo, uintptr, int32) uintptr
	glxMakeCurrent    func(uintptr, uintptr, uintptr) int32
	glxSwapBuffers    func(uintptr, uintptr)
	glxDestroyContext func(uintptr, uintptr)
)

type x11Window struct {
	display  uintptr
	window   uintptr
	ctx      uintptr
	wmDelete uintptr
	running  bool
	scale    float32
}

func New(title string, width, height int, _ bool) (Window, error) {
	runtime.LockOSThread()
	if err := ensureLibs(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		runtime.UnlockOSThread()
		return nil, errors.New("XOpenDisplay failed")
	}

	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)

	attrs := []int32{glxRGBA, glxDoubleBuffer, glxDepthSize, 24, glxNone}
	visual := glxChooseVisual(dpy, screen, &attrs[0])
	if visual == nil {
		xCloseDisplay(dpy)
		runtime.UnlockOSThread()
		return nil, errors.New("glXChooseVisual failed")
	}

	cmap := xCreateColormap(dpy, root, visual.Visual, 0)

	var swa xSetWindowAttributes
	swa.Colormap = cmap
	swa.EventMask = exposureMask | structureNotifyMask | keyPressMask | keyReleaseMask | buttonPressMask | buttonReleaseMask | pointerMotionMask

	const (
		cwColormap    = 1 << 13
		cwEventMask   = 1 << 11
		cwBorderPixel = 1 << 3
	)

	win := xCreateWindow(
		dpy, root,
		0, 0,
		uint32(width), uint32(height),
		0,
		visual.Depth,
		inputOutput,
		visual.Visual,
		cwBorderPixel|cwColormap|cwEventMask,
		unsafe.Pointer(&swa),
	)
	if win == 0 {
		xCloseDisplay(dpy)
		runtime.UnlockOSThread()
		return nil, errors.New("XCreateWindow failed")
	}
	xSelectInput(dpy, win, swa.EventMask)

	titleBytes := append([]byte(title), 0)
	xStoreName(dpy, win, &titleBytes[0])
	xMapWindow(dpy, win)

	wmDelete := xInternAtom(dpy, cString("WM_DELETE_WINDOW"), 0)
	xSetWMProtocols(dpy, win, &wmDelete, 1)

	ctx := glxCreateContext(dpy, visual, 0, 1)
	if ctx == 0 {
		xDestroyWindow(dpy, win)
		xCloseDisplay(dpy)
		runtime.UnlockOSThread()
		return nil, errors.New("glXCreateContext failed")
	}
	if glxMakeCurrent(dpy, win, ctx) == 0 {
		glxDestroyContext(dpy, ctx)
		xDestroyWindow(dpy, win)
		xCloseDisplay(dpy)
		runtime.UnlockOSThread()
		return nil, errors.New("glXMakeCurrent failed")
	}

	// Calculate scale factor from DPI
	scale := calculateScale(dpy, screen)

	w := &x11Window{
		display:  dpy,
		window:   win,
		ctx:      ctx,
		wmDelete: wmDelete,
		running:  true,
		scale:    scale,
	}
	return w, nil
}

func (w *x11Window) GL() (gl.OpenGL, error) {
	return gl.Load()
}

func (w *x11Window) Close() {
	if w.ctx != 0 {
		glxMakeCurrent(w.display, 0, 0)
		glxDestroyContext(w.display, w.ctx)
		w.ctx = 0
	}
	if w.window != 0 {
		xDestroyWindow(w.display, w.window)
		w.window = 0
	}
	if w.display != 0 {
		xCloseDisplay(w.display)
		w.display = 0
	}
	w.running = false
	runtime.UnlockOSThread()
}

func (w *x11Window) Poll() bool {
	if !w.running {
		return false
	}

	for xPending(w.display) > 0 {
		var ev [192]byte
		xNextEvent(w.display, unsafe.Pointer(&ev[0]))
		etype := *(*int32)(unsafe.Pointer(&ev[0]))
		switch etype {
		case clientMessage:
			cm := (*xclientMessage)(unsafe.Pointer(&ev[0]))
			if cm.Format == 32 && cm.Data[0] == uint64(w.wmDelete) {
				w.running = false
			}
		case destroyNotify:
			w.running = false
		}
	}
	return w.running
}

func (w *x11Window) Swap() {
	if w.display != 0 && w.window != 0 {
		glxSwapBuffers(w.display, w.window)
	}
}

func (w *x11Window) BackingSize() (int, int) {
	var root uintptr
	var x, y int32
	var width, height uint32
	var border, depth uint32
	if xGetGeometry(w.display, w.window, &root, &x, &y, &width, &height, &border, &depth) == 0 {
		return 0, 0
	}
	return int(width), int(height)
}

func (w *x11Window) Cursor() (float32, float32) {
	var root, child uintptr
	var rootX, rootY, winX, winY int32
	var mask uint32
	if xQueryPointer(w.display, w.window, &root, &child, &rootX, &rootY, &winX, &winY, &mask) == 0 {
		return 0, 0
	}
	return float32(winX), float32(winY)
}

func (w *x11Window) Scale() float32 {
	return w.scale
}

// calculateScale calculates the display scale factor based on DPI.
// It tries multiple methods in order of reliability:
// 1. GTK_SCALE environment variable (common on Wayland)
// 2. GDK_SCALE environment variable (GTK/GNOME)
// 3. QT_SCALE_FACTOR environment variable (Qt/KDE)
// 4. Xft.dpi from X resources (set by desktop environments)
// 5. DPI calculation from DisplayWidth/DisplayWidthMM
// 6. Default to 1.0 if all methods fail
func calculateScale(dpy uintptr, screen int32) float32 {
	// Check environment variables first (most reliable on Wayland)
	if scale := getEnvScale("GTK_SCALE"); scale > 0 {
		return roundScale(scale)
	}
	if scale := getEnvScale("GDK_SCALE"); scale > 0 {
		return roundScale(scale)
	}
	if scale := getEnvScale("QT_SCALE_FACTOR"); scale > 0 {
		return roundScale(scale)
	}

	// Try to get Xft.dpi from X resources (for X11)
	if xResourceManagerString != nil {
		rmString := xResourceManagerString(dpy)
		if rmString != nil {
			// Parse Xft.dpi from resource string
			// Format is like: "Xft.dpi:\t96\n..."
			rmStr := gostring(rmString)
			if dpi := parseXftDPI(rmStr); dpi > 0 {
				scale := dpi / 96.0
				// Round to common scale factors (1.0, 1.25, 1.5, 2.0, etc.)
				return roundScale(scale)
			}
		}
	}

	// Fall back to calculating DPI from physical dimensions
	if xDisplayWidth != nil && xDisplayWidthMM != nil {
		widthPx := xDisplayWidth(dpy, screen)
		widthMM := xDisplayWidthMM(dpy, screen)

		if widthMM > 0 && widthPx > 0 {
			// Calculate DPI: (pixels / millimeters) * 25.4 mm/inch
			dpi := (float32(widthPx) / float32(widthMM)) * 25.4

			// Only use this if it's reasonable (between 72 and 300 DPI)
			if dpi >= 72 && dpi <= 300 {
				scale := dpi / 96.0
				return roundScale(scale)
			}
		}
	}

	// Default to 1.0 if we can't determine scale
	return 1.0
}

// parseXftDPI extracts Xft.dpi value from X resource manager string
func parseXftDPI(rmString string) float32 {
	// Look for "Xft.dpi:" followed by a number
	// Simple parsing - look for the pattern
	start := -1
	for i := 0; i < len(rmString)-8; i++ {
		if rmString[i:i+8] == "Xft.dpi:" {
			start = i + 8
			break
		}
	}
	if start < 0 {
		return 0
	}

	// Skip whitespace
	for start < len(rmString) && (rmString[start] == ' ' || rmString[start] == '\t') {
		start++
	}

	// Parse the number
	var dpi float32
	var found bool
	for i := start; i < len(rmString); i++ {
		c := rmString[i]
		if c >= '0' && c <= '9' {
			dpi = dpi*10 + float32(c-'0')
			found = true
		} else if c == '.' && i+1 < len(rmString) {
			// Handle decimal point
			divisor := float32(10)
			for j := i + 1; j < len(rmString); j++ {
				c2 := rmString[j]
				if c2 >= '0' && c2 <= '9' {
					dpi += float32(c2-'0') / divisor
					divisor *= 10
					found = true
				} else {
					break
				}
			}
			break
		} else if found {
			break
		}
	}

	if found && dpi > 0 {
		return dpi
	}
	return 0
}

// roundScale rounds scale factor to common values to avoid precision issues
func roundScale(scale float32) float32 {
	// Common scale factors: 0.75, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 4.0
	commonScales := []float32{0.75, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 4.0}

	// Find the closest common scale
	bestScale := float32(1.0)
	minDiff := float32(1000.0)

	for _, cs := range commonScales {
		diff := abs(scale - cs)
		if diff < minDiff {
			minDiff = diff
			bestScale = cs
		}
	}

	// Only round if we're close enough (within 0.1)
	if minDiff < 0.1 {
		return bestScale
	}

	// Otherwise return the original scale, clamped
	if scale < 0.5 {
		return 0.5
	} else if scale > 4.0 {
		return 4.0
	}
	return scale
}

// getEnvScale reads a scale factor from an environment variable
func getEnvScale(envVar string) float32 {
	val := os.Getenv(envVar)
	if val == "" {
		return 0
	}

	scale, err := strconv.ParseFloat(val, 32)
	if err != nil {
		return 0
	}

	if scale > 0 {
		return float32(scale)
	}
	return 0
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
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

type xSetWindowAttributes struct {
	BackgroundPixmap uintptr
	BackgroundPixel  uint64
	BorderPixmap     uint64
	BorderPixel      uint64
	BitGravity       int32
	WinGravity       int32
	BackingStore     int32
	BackingPlanes    uint64
	BackingPixel     uint64
	SaveUnder        int32
	EventMask        int64
	DoNotPropagate   int64
	OverrideRedirect int32
	Colormap         uintptr
	Cursor           uintptr
}

func ensureLibs() error {
	var err error
	if x11lib == 0 {
		x11lib, err = purego.Dlopen("libX11.so.6", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return err
		}
		registerX11()
	}
	if gllib == 0 {
		gllib, err = purego.Dlopen("libGL.so.1", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return err
		}
		registerGLX()
	}
	return nil
}

func registerX11() {
	purego.RegisterLibFunc(&xOpenDisplay, x11lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, x11lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, x11lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateColormap, x11lib, "XCreateColormap")
	purego.RegisterLibFunc(&xCreateWindow, x11lib, "XCreateWindow")
	purego.RegisterLibFunc(&xMapWindow, x11lib, "XMapWindow")
	purego.RegisterLibFunc(&xStoreName, x11lib, "XStoreName")
	purego.RegisterLibFunc(&xInternAtom, x11lib, "XInternAtom")
	purego.RegisterLibFunc(&xSetWMProtocols, x11lib, "XSetWMProtocols")
	purego.RegisterLibFunc(&xSelectInput, x11lib, "XSelectInput")
	purego.RegisterLibFunc(&xPending, x11lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, x11lib, "XNextEvent")
	purego.RegisterLibFunc(&xGetGeometry, x11lib, "XGetGeometry")
	purego.RegisterLibFunc(&xDestroyWindow, x11lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xCloseDisplay, x11lib, "XCloseDisplay")
	purego.RegisterLibFunc(&xQueryPointer, x11lib, "XQueryPointer")
	purego.RegisterLibFunc(&xDisplayWidth, x11lib, "XDisplayWidth")
	purego.RegisterLibFunc(&xDisplayWidthMM, x11lib, "XDisplayWidthMM")
	purego.RegisterLibFunc(&xDisplayHeight, x11lib, "XDisplayHeight")
	purego.RegisterLibFunc(&xDisplayHeightMM, x11lib, "XDisplayHeightMM")
	// Try to register XResourceManagerString, but don't fail if it's not available
	if _, err := purego.Dlsym(x11lib, "XResourceManagerString"); err == nil {
		purego.RegisterLibFunc(&xResourceManagerString, x11lib, "XResourceManagerString")
	} else {
		// Function not available, will fall back to DPI calculation
		xResourceManagerString = nil
	}
}

func registerGLX() {
	purego.RegisterLibFunc(&glxChooseVisual, gllib, "glXChooseVisual")
	purego.RegisterLibFunc(&glxCreateContext, gllib, "glXCreateContext")
	purego.RegisterLibFunc(&glxMakeCurrent, gllib, "glXMakeCurrent")
	purego.RegisterLibFunc(&glxSwapBuffers, gllib, "glXSwapBuffers")
	purego.RegisterLibFunc(&glxDestroyContext, gllib, "glXDestroyContext")
}

func cString(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}
