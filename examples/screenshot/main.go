package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/tinyrange/gowin/graphics"
)

var errDone = errors.New("screenshot captured")

func main() {
	out := flag.String("out", "screenshot.png", "path to write the captured PNG")
	frames := flag.Int("frames", 5, "number of frames to keep the window open before capture")
	logical := flag.Bool("logical", false, "write logical-size screenshot instead of backing-pixel screenshot")
	flag.Parse()

	if *frames < 1 {
		*frames = 1
	}

	win, err := graphics.New("gowin screenshot example", 640, 360)
	if err != nil {
		panic(err)
	}
	win.SetClearColor(color.RGBA{R: 20, G: 24, B: 31, A: 255})

	frame := 0
	err = win.Loop(func(f graphics.Frame) error {
		frame++

		w, h := f.WindowSize()
		bw, bh := f.BackingSize()
		f.RenderQuad(0, 0, float32(w), float32(h), nil, color.RGBA{R: 34, G: 40, B: 52, A: 255})
		f.RenderQuad(72, 72, 240, 132, nil, color.RGBA{R: 89, G: 177, B: 255, A: 255})
		f.RenderQuad(360, 120, 180, 150, nil, color.RGBA{R: 255, G: 194, B: 87, A: 255})
		f.RenderQuad(108, 238, 424, 42, nil, color.RGBA{R: 130, G: 232, B: 156, A: 255})

		if frame < *frames {
			return nil
		}

		var img image.Image
		if *logical {
			img, err = f.ScreenshotLogical()
		} else {
			img, err = f.Screenshot()
		}
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

		fmt.Printf("wrote %s after %d frames (logical=%dx%d backing=%dx%d scale=%.2f)\n", *out, frame, w, h, bw, bh, f.Scale())
		return errDone
	})
	if err != nil && !errors.Is(err, errDone) {
		panic(err)
	}
}
