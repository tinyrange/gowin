package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/tinyrange/gowin/graphics"
)

var errDone = errors.New("screenshot captured")

func main() {
	out := flag.String("out", "preview3d.png", "path to write the captured PNG")
	frames := flag.Int("frames", 90, "number of frames to render before capture")
	flag.Parse()

	win, err := graphics.New("gowin 3D preview example", 900, 600)
	if err != nil {
		panic(err)
	}
	win.SetClearColor(color.RGBA{R: 18, G: 20, B: 24, A: 255})

	blockVerts, blockIdx := graphics.Cuboid3DGeometry(4.8, 3.0, 0.18, color.RGBA{R: 54, G: 99, B: 161, A: 255})
	block, err := win.NewMesh3D(blockVerts, blockIdx)
	if err != nil {
		panic(err)
	}
	defer block.Destroy()

	surfaceVerts, surfaceIdx := graphics.PlanarRect3DGeometry(7.0, 4.8, 0.12, color.RGBA{R: 203, G: 151, B: 50, A: 255})
	surface, err := win.NewMesh3D(surfaceVerts, surfaceIdx)
	if err != nil {
		panic(err)
	}
	defer surface.Destroy()

	clipVerts, clipIdx := graphics.PlanarRect3DGeometry(4.8, 3.0, 0.2, color.White)
	clip, err := win.NewMesh3D(clipVerts, clipIdx)
	if err != nil {
		panic(err)
	}
	defer clip.Destroy()

	frame := 0
	err = win.Loop(func(f graphics.Frame) error {
		frame++
		w, h := f.WindowSize()
		f.RenderQuad(0, 0, float32(w), float32(h), nil, color.RGBA{R: 18, G: 20, B: 24, A: 255})

		angle := float32(frame) * 0.025
		model := graphics.MulMat4(graphics.RotateYMat4(angle), graphics.RotateXMat4(-0.65))
		view := graphics.LookAtMat4(
			graphics.Vec3{X: 0, Y: 0.2, Z: 6.8},
			graphics.Vec3{},
			graphics.Vec3{Y: 1},
		)
		proj := graphics.PerspectiveMat4(float32(math.Pi/4), float32(w)/float32(h), 0.1, 100)
		f.RenderMesh3D(block, graphics.Draw3DOptions{
			Model:          model,
			View:           view,
			Projection:     proj,
			Ambient:        0.28,
			LightDirection: graphics.Vec3{X: -0.4, Y: 0.7, Z: 0.6},
		})
		opts := graphics.Draw3DOptions{
			Model:          model,
			View:           view,
			Projection:     proj,
			Ambient:        0.42,
			LightDirection: graphics.Vec3{X: -0.4, Y: 0.7, Z: 0.6},
		}
		f.PushClipMesh3D(clip, opts)
		f.RenderMesh3D(surface, opts)
		f.PopClipMesh3D()

		if frame < *frames {
			return nil
		}
		var img image.Image
		img, err = f.Screenshot()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil && filepath.Dir(*out) != "." {
			return err
		}
		file, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			return err
		}
		fmt.Printf("wrote %s after %d frames\n", *out, frame)
		return errDone
	})
	if err != nil && !errors.Is(err, errDone) {
		panic(err)
	}
}
