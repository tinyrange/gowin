package graphics

import (
	"image"
	"image/color"
	"testing"
)

func TestResizeImageNearestBackingToLogical(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	src.Set(0, 0, color.RGBA{R: 10, A: 255})
	src.Set(2, 0, color.RGBA{G: 20, A: 255})
	src.Set(0, 2, color.RGBA{B: 30, A: 255})
	src.Set(2, 2, color.RGBA{R: 40, G: 50, B: 60, A: 255})

	got := ResizeImageNearest(src, 2, 2)
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("logical size = %v, want 2x2", got.Bounds())
	}

	tests := []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{R: 10, A: 255}},
		{1, 0, color.RGBA{G: 20, A: 255}},
		{0, 1, color.RGBA{B: 30, A: 255}},
		{1, 1, color.RGBA{R: 40, G: 50, B: 60, A: 255}},
	}
	for _, tt := range tests {
		if got := color.RGBAModel.Convert(got.At(tt.x, tt.y)).(color.RGBA); got != tt.want {
			t.Fatalf("pixel %d,%d = %#v, want %#v", tt.x, tt.y, got, tt.want)
		}
	}
}
