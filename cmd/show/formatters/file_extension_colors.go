package formatters

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
)

var extensionColorPalette = []string{
	"lightblue", "lightyellow", "mistyrose", "lightsalmon",
	"lightpink", "lavender", "peachpuff", "plum", "powderblue", "khaki",
	"palegoldenrod", "thistle",
}

// paletteColor returns the color for the i-th (0-indexed) distinct file type.
// The first len(extensionColorPalette) types use the curated named palette;
// beyond that, colors are generated rather than wrapping back to the start,
// so a graph spanning more file types than the curated palette never assigns
// the same color to two different types (CLR-72).
func paletteColor(i int) string {
	if i < len(extensionColorPalette) {
		return extensionColorPalette[i]
	}
	return generatedPastelColor(i - len(extensionColorPalette))
}

// generatedPastelColor produces a pastel hex color for overflow index i
// (0-based). Hues are spaced by the golden angle so consecutive colors stay
// visually distinct no matter how large i grows.
func generatedPastelColor(i int) string {
	const goldenAngle = 137.508
	hue := math.Mod(float64(i)*goldenAngle, 360)
	return hslToHex(hue, 0.55, 0.85)
}

// hslToHex converts an HSL color (hue in degrees, saturation/lightness in
// [0,1]) to a "#rrggbb" hex string.
func hslToHex(hue, saturation, lightness float64) string {
	c := (1 - math.Abs(2*lightness-1)) * saturation
	hPrime := hue / 60
	x := c * (1 - math.Abs(math.Mod(hPrime, 2)-1))
	m := lightness - c/2

	var r, g, b float64
	switch {
	case hPrime < 1:
		r, g, b = c, x, 0
	case hPrime < 2:
		r, g, b = x, c, 0
	case hPrime < 3:
		r, g, b = 0, c, x
	case hPrime < 4:
		r, g, b = 0, x, c
	case hPrime < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	toByte := func(v float64) int {
		return int(math.Round((v + m) * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", toByte(r), toByte(g), toByte(b))
}

// fileTypeKey returns the key used to group a file for color assignment.
// Files with an extension are grouped by extension (e.g. ".go"). Files without
// an extension are grouped by their basename, so `pre-commit` and `pre-push`
// become distinct types instead of collapsing into a single empty-string key.
func fileTypeKey(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		return ext
	}
	return base
}

func getExtensionColors(fileNames []string) map[string]string {
	uniqueExtensions := make(map[string]bool)
	for _, fileName := range fileNames {
		uniqueExtensions[fileTypeKey(fileName)] = true
	}

	sortedExtensions := make([]string, 0, len(uniqueExtensions))
	for ext := range uniqueExtensions {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	extensionColors := make(map[string]string)
	for i, ext := range sortedExtensions {
		extensionColors[ext] = paletteColor(i)
	}

	return extensionColors
}
