package pdf

import (
	"fmt"
	"image/png"
	"strings"
)

// PNGCompressionLevels are the compression levels that can be used when
// rendering PNG images, in the order that they are shown to the user.
var PNGCompressionLevels = []string{"default", "none", "speed", "best"}

// ParsePNGCompression parses the name of a PNG compression level into the
// level that the PNG encoder uses. The levels trade encoding speed for file
// size, they do not change the image itself, PNG compression is lossless.
func ParsePNGCompression(compression string) (png.CompressionLevel, error) {
	switch strings.ToLower(strings.TrimSpace(compression)) {
	case "default":
		return png.DefaultCompression, nil
	case "none":
		return png.NoCompression, nil
	case "speed":
		return png.BestSpeed, nil
	case "best":
		return png.BestCompression, nil
	}

	return png.DefaultCompression, fmt.Errorf("valid values are %s", strings.Join(PNGCompressionLevels, ", "))
}
