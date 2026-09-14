package pdf

import (
	"testing"
)

func TestParsePageCrop(t *testing.T) {
	tests := []struct {
		name    string
		crop    string
		want    PageCrop
		wantErr string
	}{
		{
			"test a simple crop",
			"0,0,100,200",
			PageCrop{X: 0, Y: 0, Width: 100, Height: 200},
			"",
		},
		{
			"test a crop with an offset",
			"36,72,144.5,72.25",
			PageCrop{X: 36, Y: 72, Width: 144.5, Height: 72.25},
			"",
		},
		{
			"test a crop with whitespace",
			"  36 , 72 , 144 , 72  ",
			PageCrop{X: 36, Y: 72, Width: 144, Height: 72},
			"",
		},
		{
			"test a crop in scientific notation",
			"1e1,2e1,1.5e2,1e2",
			PageCrop{X: 10, Y: 20, Width: 150, Height: 100},
			"",
		},
		{
			"test a crop with an explicit plus",
			"+10,+20,+30,+40",
			PageCrop{X: 10, Y: 20, Width: 30, Height: 40},
			"",
		},
		{
			"test a relative crop",
			"0,0,0.5,0.5",
			PageCrop{X: 0, Y: 0, Width: 0.5, Height: 0.5},
			"",
		},
		{
			"test a crop that starts before the page",
			"-50,-25,100,100",
			PageCrop{X: -50, Y: -25, Width: 100, Height: 100},
			"",
		},
		{
			"test a crop with too few values",
			"1,2,3",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 3",
		},
		{
			"test a crop with too many values",
			"1,2,3,4,5",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 5",
		},
		{
			"test an empty crop",
			"",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 1",
		},
		{
			"test a crop with only whitespace",
			"   ",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 1",
		},
		{
			"test a crop with the wrong separator",
			"0;0;100;100",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 1",
		},
		{
			"test a crop with a trailing separator",
			"0,0,100,100,",
			PageCrop{},
			"a crop must have 4 comma separated values (x,y,width,height), got 5",
		},
		{
			"test a crop with a value that is not a number",
			"1,2,abc,4",
			PageCrop{},
			"crop width 'abc' is not a valid number",
		},
		{
			"test a crop with an empty value",
			"1,,3,4",
			PageCrop{},
			"crop y '' is not a valid number",
		},
		{
			"test a crop with a unit suffix",
			"10pt,0,100,100",
			PageCrop{},
			"crop x '10pt' is not a valid number",
		},
		{
			"test a crop with a percentage",
			"0,0,50%,100",
			PageCrop{},
			"crop width '50%' is not a valid number",
		},
		{
			"test a crop that is not a number",
			"NaN,0,100,100",
			PageCrop{},
			"crop x 'NaN' is not a valid number",
		},
		{
			"test a crop that is infinite",
			"0,Inf,100,100",
			PageCrop{},
			"crop y 'Inf' is not a valid number",
		},
		{
			"test a crop that is out of range",
			"1e999,0,100,100",
			PageCrop{},
			"crop x '1e999' is not a valid number",
		},
		{
			"test a crop with a negative width",
			"0,0,-100,100",
			PageCrop{},
			"crop width must be larger than 0",
		},
		{
			"test a crop without a width",
			"0,0,0,100",
			PageCrop{},
			"crop width must be larger than 0",
		},
		{
			"test a crop with a negative height",
			"0,0,100,-100",
			PageCrop{},
			"crop height must be larger than 0",
		},
		{
			"test a crop without a height",
			"0,0,100,0",
			PageCrop{},
			"crop height must be larger than 0",
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			parsedCrop, err := ParsePageCrop(tests[i].crop)
			if tests[i].wantErr == "" && err != nil {
				t.Errorf("expected no error but got error %s", err.Error())
			} else if tests[i].wantErr != "" && err == nil {
				t.Errorf("expected error %s but got no error", tests[i].wantErr)
			} else if tests[i].wantErr != "" && err != nil && err.Error() != tests[i].wantErr {
				t.Errorf("expected error %s but got error %s", tests[i].wantErr, err.Error())
			} else if err == nil && tests[i].want != *parsedCrop {
				t.Errorf("expected %+v but got %+v", tests[i].want, *parsedCrop)
			}
		})
	}
}

func TestPageCropToPoints(t *testing.T) {
	// A4 in points.
	const pageWidth = 595.2755737304688
	const pageHeight = 841.8897094726562

	tests := []struct {
		name string
		crop PageCrop
		unit CropUnit
		dpi  int
		want PageCrop
	}{
		{
			"test that points are used as they are",
			PageCrop{X: 100, Y: 200, Width: 150, Height: 120},
			CropUnitPoints,
			200,
			PageCrop{X: 100, Y: 200, Width: 150, Height: 120},
		},
		{
			"test that pixels in 72 dpi are the same as points",
			PageCrop{X: 100, Y: 200, Width: 150, Height: 120},
			CropUnitPixels,
			72,
			PageCrop{X: 100, Y: 200, Width: 150, Height: 120},
		},
		{
			"test that pixels are converted with the dpi",
			PageCrop{X: 100, Y: 200, Width: 150, Height: 300},
			CropUnitPixels,
			144,
			PageCrop{X: 50, Y: 100, Width: 75, Height: 150},
		},
		{
			"test that a relative crop is converted with the page size",
			PageCrop{X: 0, Y: 0, Width: 1, Height: 1},
			CropUnitRelative,
			200,
			PageCrop{X: 0, Y: 0, Width: pageWidth, Height: pageHeight},
		},
		{
			"test that half a relative crop is half the page",
			PageCrop{X: 0.5, Y: 0.5, Width: 0.5, Height: 0.5},
			CropUnitRelative,
			200,
			PageCrop{X: pageWidth / 2, Y: pageHeight / 2, Width: pageWidth / 2, Height: pageHeight / 2},
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			converted := tests[i].crop.ToPoints(tests[i].unit, pageWidth, pageHeight, tests[i].dpi)
			if converted != tests[i].want {
				t.Errorf("expected %+v but got %+v", tests[i].want, converted)
			}
		})
	}
}

func TestPageCropIsOutsidePage(t *testing.T) {
	const pageWidth = 595.2755737304688
	const pageHeight = 841.8897094726562

	tests := []struct {
		name string
		crop PageCrop
		want bool
	}{
		{"test a crop inside the page", PageCrop{X: 100, Y: 100, Width: 100, Height: 100}, false},
		{"test a crop that covers the page", PageCrop{X: 0, Y: 0, Width: pageWidth, Height: pageHeight}, false},
		{"test a crop that runs past the right of the page", PageCrop{X: 500, Y: 100, Width: 200, Height: 100}, false},
		{"test a crop that starts before the page", PageCrop{X: -50, Y: -50, Width: 100, Height: 100}, false},
		{"test a crop fully to the right of the page", PageCrop{X: 600, Y: 100, Width: 100, Height: 100}, true},
		{"test a crop fully below the page", PageCrop{X: 100, Y: 900, Width: 100, Height: 100}, true},
		{"test a crop fully to the left of the page", PageCrop{X: -200, Y: 100, Width: 100, Height: 100}, true},
		{"test a crop fully above the page", PageCrop{X: 100, Y: -200, Width: 100, Height: 100}, true},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			if got := tests[i].crop.IsOutsidePage(pageWidth, pageHeight); got != tests[i].want {
				t.Errorf("expected %v but got %v", tests[i].want, got)
			}
		})
	}
}
