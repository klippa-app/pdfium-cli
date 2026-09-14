package pdf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CropUnit is the unit that the values of a crop region are given in.
type CropUnit int

const (
	// CropUnitPixels means that the values are pixels of the full page as it
	// would be rendered in the requested DPI.
	CropUnitPixels CropUnit = iota

	// CropUnitPoints means that the values are points, one point is 1/72 inch.
	// This is the same unit that the info command reports page sizes in.
	CropUnitPoints

	// CropUnitRelative means that the values are a fraction of the page size,
	// where 1 is the full width or height of the page.
	CropUnitRelative
)

// PageCrop is a rectangular region of a page. The origin (0,0) is the top-left
// corner of the page as it is rendered, and Y grows downwards.
type PageCrop struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ParsePageCrop parses a crop region in the format "x,y,width,height".
// Whitespace around the values is ignored. The values are not converted to a
// unit here, use ToPoints for that.
//
// The X and Y are allowed to be negative, a region that partly falls outside of
// the page is valid, so that a page can be cut into equally sized tiles.
func ParsePageCrop(crop string) (*PageCrop, error) {
	cropParts := strings.Split(strings.TrimSpace(crop), ",")
	if len(cropParts) != 4 {
		return nil, fmt.Errorf("a crop must have 4 comma separated values (x,y,width,height), got %d", len(cropParts))
	}

	cropNames := [4]string{"x", "y", "width", "height"}
	cropValues := [4]float64{}
	for i := range cropParts {
		cropPart := strings.TrimSpace(cropParts[i])

		parsedValue, err := strconv.ParseFloat(cropPart, 64)
		if err != nil || math.IsNaN(parsedValue) || math.IsInf(parsedValue, 0) {
			return nil, fmt.Errorf("crop %s '%s' is not a valid number", cropNames[i], cropPart)
		}

		cropValues[i] = parsedValue
	}

	if cropValues[2] <= 0 {
		return nil, fmt.Errorf("crop width must be larger than 0")
	}

	if cropValues[3] <= 0 {
		return nil, fmt.Errorf("crop height must be larger than 0")
	}

	return &PageCrop{
		X:      cropValues[0],
		Y:      cropValues[1],
		Width:  cropValues[2],
		Height: cropValues[3],
	}, nil
}

// ToPoints converts a crop region into points, which is the unit that pdfium
// renders regions in. The page size has to be given in points, and the DPI is
// the DPI that pixel values are relative to.
func (c PageCrop) ToPoints(unit CropUnit, pageWidth, pageHeight float64, dpi int) PageCrop {
	switch unit {
	case CropUnitPixels:
		scale := float64(dpi) / 72.0
		return PageCrop{
			X:      c.X / scale,
			Y:      c.Y / scale,
			Width:  c.Width / scale,
			Height: c.Height / scale,
		}
	case CropUnitRelative:
		return PageCrop{
			X:      c.X * pageWidth,
			Y:      c.Y * pageHeight,
			Width:  c.Width * pageWidth,
			Height: c.Height * pageHeight,
		}
	default:
		return c
	}
}

// IsOutsidePage returns whether the region does not overlap the page at all.
// A region that only partly falls outside of the page is fine, the part that
// falls outside of the page is filled with the background color.
func (c PageCrop) IsOutsidePage(pageWidth, pageHeight float64) bool {
	return c.X >= pageWidth || c.Y >= pageHeight || c.X+c.Width <= 0 || c.Y+c.Height <= 0
}
