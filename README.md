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

## High-Level API

The root `github.com/tinyrange/gowin` package provides an XNA/raylib-inspired
application loop, custom math types, textures built from Go `image.Image`
values, `image/color` colors, render targets, meshes, prepared draw commands,
custom 3D shaders, and lightweight scene nodes. It is intended for small apps
and games that want a friendly API while still being able to move hot rendering
paths into persistent mesh and draw-command data.

```go
package main

import (
	"image/color"

	"github.com/tinyrange/gowin"
)

type Game struct {
	mesh  *gowin.Mesh
	draw  *gowin.DrawCommand
	atlas gowin.Texture2D
}

func (g *Game) Init(ctx *gowin.Context) error {
	atlas, err := gowin.LoadTexture(ctx, atlasReader)
	if err != nil {
		return err
	}
	g.atlas = atlas

	g.mesh = ctx.NewMesh()
	if err := g.mesh.SetData(myMeshData()); err != nil {
		return err
	}

	draw, err := ctx.PrepareDraw(g.mesh, gowin.DrawOptions{
		Shader: gowin.DefaultShader3D(),
		Textures: map[string]gowin.Texture2D{
			"u_texture": g.atlas,
		},
		Uniforms: gowin.Uniforms{
			"u_Ambient": 0.35,
		},
	})
	g.draw = draw
	return err
}

func (g *Game) Update(ctx *gowin.Context, dt float32) error { return nil }

func (g *Game) Draw(ctx *gowin.Context) error {
	ctx.Begin3D(gowin.Camera3D{Position: gowin.Vec3{Z: 5}, Up: gowin.Vec3{Y: 1}})
	ctx.Draw(g.draw)
	ctx.End3D()
	ctx.DrawText("hello gowin", 16, 24, 16, color.White)
	return nil
}
```

Root-package conveniences include `LoadImage`, `Context.NewTexture`,
`LoadTexture`, `Context.NewRenderTarget2D`, `Context.DrawTo`, `LoadShader`,
`Context.DrawScene`, `Mesh.Destroy`, and `Context.WriteScreenshotPNG`. Prepared
draw commands can bind texture samplers by uniform name and pass common shader
uniform values. See `examples/draw_commands` for a complete draw command example
and `examples/voxel` for a streaming editable voxel world demo with a generated
block texture atlas.

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
go run ./examples/draw_commands
go run ./examples/voxel
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
