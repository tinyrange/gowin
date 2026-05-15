package graphics

import (
	"image"
	"image/draw"
)

// ResizeImageNearest returns src resampled to width x height with nearest-neighbor
// sampling. It is intended for logical-size UI screenshots derived from backing
// pixel screenshots.
func ResizeImageNearest(src image.Image, width, height int) image.Image {
	if src == nil || width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	sb := src.Bounds()
	if sb.Dx() == width && sb.Dy() == height {
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Src)
		return dst
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sy := sb.Min.Y + y*sb.Dy()/height
		for x := 0; x < width; x++ {
			sx := sb.Min.X + x*sb.Dx()/width
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
