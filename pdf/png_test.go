package pdf

import (
	"image/png"
	"testing"
)

func TestParsePNGCompression(t *testing.T) {
	tests := []struct {
		name        string
		compression string
		want        png.CompressionLevel
		wantErr     string
	}{
		{
			"test the default level",
			"default",
			png.DefaultCompression,
			"",
		},
		{
			"test no compression",
			"none",
			png.NoCompression,
			"",
		},
		{
			"test the fastest level",
			"speed",
			png.BestSpeed,
			"",
		},
		{
			"test the smallest level",
			"best",
			png.BestCompression,
			"",
		},
		{
			"test that the level is case insensitive",
			"BeSt",
			png.BestCompression,
			"",
		},
		{
			"test a level with whitespace",
			"  best  ",
			png.BestCompression,
			"",
		},
		{
			"test an unknown level",
			"maximum",
			png.DefaultCompression,
			"valid values are default, none, speed, best",
		},
		{
			"test an empty level",
			"",
			png.DefaultCompression,
			"valid values are default, none, speed, best",
		},
		{
			"test a numeric level",
			"9",
			png.DefaultCompression,
			"valid values are default, none, speed, best",
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			compression, err := ParsePNGCompression(tests[i].compression)
			if tests[i].wantErr == "" && err != nil {
				t.Errorf("expected no error but got error %s", err.Error())
			} else if tests[i].wantErr != "" && err == nil {
				t.Errorf("expected error %s but got no error", tests[i].wantErr)
			} else if tests[i].wantErr != "" && err != nil && err.Error() != tests[i].wantErr {
				t.Errorf("expected error %s but got error %s", tests[i].wantErr, err.Error())
			} else if err == nil && compression != tests[i].want {
				t.Errorf("expected %d but got %d", tests[i].want, compression)
			}
		})
	}
}
