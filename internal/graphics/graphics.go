package graphics

import (
	"image"

	"github.com/tinyrange/gowin/internal/window"
)

// Color represents an RGBA color with components in the range [0, 1].
type Color [4]float32

// Default colors
var (
	ColorBlack     = Color{0, 0, 0, 1}
	ColorWhite     = Color{1, 1, 1, 1}
	ColorRed       = Color{1, 0, 0, 1}
	ColorGreen     = Color{0, 1, 0, 1}
	ColorBlue      = Color{0, 0, 1, 1}
	ColorYellow    = Color{1, 1, 0, 1}
	ColorCyan      = Color{0, 1, 1, 1}
	ColorMagenta   = Color{1, 0, 1, 1}
	ColorGray      = Color{0.5, 0.5, 0.5, 1}
	ColorDarkGray  = Color{0.25, 0.25, 0.25, 1}
	ColorLightGray = Color{0.75, 0.75, 0.75, 1}
)

type KeyState int

const (
	// The key was pressed this frame
	KeyStatePressed KeyState = iota
	// The key is currently down
	KeyStateDown
	// The key was released this frame
	KeyStateReleased
	// The key is currently up
	KeyStateUp
	// The key is being held down (repeated)
	KeyStateRepeated
)

func (ks KeyState) IsDown() bool {
	return ks == KeyStatePressed || ks == KeyStateDown || ks == KeyStateRepeated
}

type ButtonState int

const (
	// The mouse button was pressed this frame
	ButtonStatePressed ButtonState = iota
	// The mouse button is currently down
	ButtonStateDown
	// The mouse button was released this frame
	ButtonStateReleased
	// The mouse button is currently up
	ButtonStateUp
)

func (bs ButtonState) IsDown() bool {
	return bs == ButtonStatePressed || bs == ButtonStateDown
}

type Frame interface {
	WindowSize() (width, height int)
	CursorPos() (x, y float32)

	GetKeyState(key window.Key) KeyState
	GetButtonState(button window.Button) ButtonState

	RenderQuad(x, y, width, height float32, tex Texture, color Color)

	Screenshot() (image.Image, error)
}

type Texture interface {
	Size() (width, height int)
}

type Window interface {
	// Return the platform-specific window implementation.
	PlatformWindow() window.Window

	// Create a new texture from an image.
	NewTexture(image.Image) (Texture, error)

	SetClear(enabled bool)
	SetClearColor(color Color)

	// Scale returns the display scaling factor (e.g., 1.0 for 96 DPI, 2.0 for 192 DPI).
	Scale() float32

	// Call f for each frame until it returns an error.
	Loop(func(f Frame) error) error
}

// Each platform implements a New() method to return a Window.
