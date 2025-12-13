package main

import (
	"image"
	"image/color"
	"log"

	"github.com/tinyrange/gowin/internal/gl"
	"github.com/tinyrange/gowin/internal/graphics"
	"github.com/tinyrange/gowin/internal/text"
)

func main() {
	gfx, err := graphics.New("Pure Go GL Demo", 800, 600)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	ogl, err := gfx.PlatformWindow().GL()
	if err != nil {
		log.Fatalf("gl: %v", err)
	}

	log.Printf("OpenGL version: %s", ogl.GetString(gl.Version))
	log.Printf("OpenGL vendor: %s", ogl.GetString(gl.Vendor))

	gfx.SetClear(true)
	gfx.SetClearColor(0.1, 0.12, 0.16, 1.0)

	tex, err := makeCheckerTexture(gfx)
	if err != nil {
		log.Fatalf("texture: %v", err)
	}

	font, err := text.Load(gfx)
	if err != nil {
		log.Fatalf("font: %v", err)
	}

	const quadSize = 200.0

	err = gfx.Loop(func(f graphics.Frame) error {
		x, y := f.CursorPos()

		f.RenderQuad(x, y, float32(quadSize), float32(quadSize), tex, [4]float32{1, 1, 1, 1})

		font.RenderText("The quick brown fox jumps over the lazy dog.", 10, 24, 16, [4]float32{1, 1, 0, 1})

		// screenshot, err := f.Screenshot()
		// if err != nil {
		// 	log.Fatalf("screenshot: %v", err)
		// }

		// screenshotPath := "screenshot.png"

		// file, err := os.Create(screenshotPath)
		// if err != nil {
		// 	return fmt.Errorf("create screenshot file: %v", err)
		// }
		// defer file.Close()

		// if err := png.Encode(file, screenshot); err != nil {
		// 	return fmt.Errorf("encode screenshot: %v", err)
		// }

		// return fmt.Errorf("taken screenshot at %s", screenshotPath)
		return nil
	})
	if err != nil {
		log.Fatalf("run loop: %v", err)
	}
}

func makeCheckerTexture(gfx graphics.Window) (graphics.Texture, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	red := color.NRGBA{R: 0xff, G: 0x66, B: 0x66, A: 0xff}
	green := color.NRGBA{R: 0x66, G: 0xff, B: 0x66, A: 0xff}

	for y := range 4 {
		for x := range 4 {
			if (x+y)%2 == 0 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, green)
			}
		}
	}

	return gfx.NewTexture(img)
}
