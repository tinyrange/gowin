package graphics

import (
	"image"

	"github.com/tinyrange/gowin/internal/window"
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

	RenderQuad(x, y, width, height float32, tex Texture, color [4]float32)

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
	SetClearColor(r, g, b, a float32)

	// Call f for each frame until it returns an error.
	Loop(func(f Frame) error) error
}

// Each platform implements a New() method to return a Window.
