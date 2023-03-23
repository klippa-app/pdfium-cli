package cmd

import (
	"image"
	"image/color"
)

// BGR is an in-memory image whose At method returns color.RGBA values.
type BGR struct {
	// Pix holds the image's pixels, in B, G, R order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*3].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

func (p *BGR) ColorModel() color.Model { return color.RGBAModel }

func (p *BGR) Bounds() image.Rectangle { return p.Rect }

func (p *BGR) At(x, y int) color.Color {
	return p.RGBAAt(x, y)
}

func (p *BGR) RGBAAt(x, y int) color.RGBA {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+3 : i+3] // Small cap improves performance, see https://golang.org/issue/27857
	return color.RGBA{s[2], s[1], s[0], 255}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *BGR) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*3
}
