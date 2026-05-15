package gowin

import "image/color"

// Color stores linear-style RGBA components in the range 0..1.
type Color struct {
	R float32
	G float32
	B float32
	A float32
}

var (
	Black       = Color{R: 0, G: 0, B: 0, A: 1}
	White       = Color{R: 1, G: 1, B: 1, A: 1}
	Red         = Color{R: 1, G: 0, B: 0, A: 1}
	Green       = Color{R: 0, G: 1, B: 0, A: 1}
	Blue        = Color{R: 0, G: 0, B: 1, A: 1}
	Yellow      = Color{R: 1, G: 1, B: 0, A: 1}
	Cyan        = Color{R: 0, G: 1, B: 1, A: 1}
	Magenta     = Color{R: 1, G: 0, B: 1, A: 1}
	Transparent = Color{}
)

func (c Color) RGBA() (r, g, b, a uint32) {
	return uint32(clamp01(c.R) * 0xffff),
		uint32(clamp01(c.G) * 0xffff),
		uint32(clamp01(c.B) * 0xffff),
		uint32(clamp01(c.A) * 0xffff)
}

func (c Color) color() color.Color {
	return c
}

func colorFrom(c color.Color) Color {
	r, g, b, a := c.RGBA()
	return Color{
		R: float32(r) / 0xffff,
		G: float32(g) / 0xffff,
		B: float32(b) / 0xffff,
		A: float32(a) / 0xffff,
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
