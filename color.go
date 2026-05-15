package gowin

import "image/color"

var (
	Black     = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	White     = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	Red       = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	Green     = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	Blue      = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	Yellow    = color.NRGBA{R: 255, G: 255, B: 0, A: 255}
	Cyan      = color.NRGBA{R: 0, G: 255, B: 255, A: 255}
	Magenta   = color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	Gray      = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	DarkGray  = color.NRGBA{R: 64, G: 64, B: 64, A: 255}
	LightGray = color.NRGBA{R: 192, G: 192, B: 192, A: 255}
)
