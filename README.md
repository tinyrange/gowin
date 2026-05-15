# gowin

gowin is a small Go windowing and OpenGL drawing library. It provides native
windows, input, clipboard support, screenshots, 2D drawing helpers, simple 3D
mesh rendering, render targets, SVG helpers, and text rendering.

The library is intentionally low-level: application, UI, editor, and game code
decide what input events mean. gowin reports platform-neutral keys, text,
modifiers, mouse buttons, cursor motion, scrolling, pinch gestures, and window
state.

## Install

```sh
go get github.com/tinyrange/gowin
```

gowin currently targets Go 1.18 and newer.

## Packages

- `github.com/tinyrange/gowin/window`: native windows, input, clipboard, file
  dialogs, URL events, and OpenGL contexts.
- `github.com/tinyrange/gowin/graphics`: drawing API, shapes, meshes, render
  targets, screenshots, SVG helpers, and 3D transforms.
- `github.com/tinyrange/gowin/text`: embedded Roboto Mono text renderer for
  `graphics.Window`.
- `github.com/tinyrange/gowin/gl`: internal-style OpenGL binding surface used by
  the window and graphics packages.

## Example

```go
package main

import (
	"image/color"

	"github.com/tinyrange/gowin/graphics"
)

func main() {
	win, err := graphics.New("gowin example", 640, 360)
	if err != nil {
		panic(err)
	}
	win.SetClearColor(color.RGBA{R: 20, G: 24, B: 31, A: 255})

	err = win.Loop(func(f graphics.Frame) error {
		w, h := f.WindowSize()
		f.RenderQuad(0, 0, float32(w), float32(h), nil, color.RGBA{R: 34, G: 40, B: 52, A: 255})
		f.RenderQuad(96, 80, 240, 132, nil, color.RGBA{R: 89, G: 177, B: 255, A: 255})
		return nil
	})
	if err != nil {
		panic(err)
	}
}
```

More examples live in `examples/`:

```sh
go run ./examples/screenshot
go run ./examples/preview3d
```

## Platform Notes

gowin uses native platform APIs directly:

- macOS: Cocoa/OpenGL.
- Windows: Win32/WGL.
- Linux: X11/GLX.

Linux builds require X11 and OpenGL development libraries to be available on the
system. Headless CI or containers may need a virtual X server for examples that
open a real window.

## Status

gowin is pre-v1. Public APIs are useful today, but small breaking changes may
still happen while the package names and platform contracts settle.

## License

gowin is distributed under the MIT License. See `LICENSE`.

This repository includes third-party code and font assets under their own
licenses. See `THIRD_PARTY_NOTICES.md`.
