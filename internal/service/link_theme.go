package service

import (
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"panel/internal/model"
	"panel/pkg/runtimepath"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// LinkTheme stores the generated card palette for a link.
type LinkTheme struct {
	AccentColor  string
	BgStartColor string
	BgEndColor   string
	BorderColor  string
	TextColor    string
}

// BuildLinkTheme derives a stable card palette from a cached icon when possible,
// and falls back to a deterministic domain/title color when image decoding fails.
func BuildLinkTheme(uploadDir, iconCachedPath, rawURL, title string) LinkTheme {
	if iconTheme, err := buildLinkThemeFromIcon(uploadDir, iconCachedPath); err == nil && !iconTheme.IsZero() {
		return iconTheme
	}
	return buildLinkThemeFromSeed(rawURL, title)
}

// ApplyLinkTheme copies palette values into a NavLink model.
func ApplyLinkTheme(link *model.NavLink, theme LinkTheme) {
	if link == nil {
		return
	}
	link.ThemeAccentColor = strings.TrimSpace(theme.AccentColor)
	link.ThemeBgStartColor = strings.TrimSpace(theme.BgStartColor)
	link.ThemeBgEndColor = strings.TrimSpace(theme.BgEndColor)
	link.ThemeBorderColor = strings.TrimSpace(theme.BorderColor)
	link.ThemeTextColor = strings.TrimSpace(theme.TextColor)
}

// HasStoredTheme reports whether the link already has a full persisted theme.
func HasStoredTheme(link model.NavLink) bool {
	return strings.TrimSpace(link.ThemeAccentColor) != "" &&
		strings.TrimSpace(link.ThemeBgStartColor) != "" &&
		strings.TrimSpace(link.ThemeBgEndColor) != "" &&
		strings.TrimSpace(link.ThemeBorderColor) != "" &&
		strings.TrimSpace(link.ThemeTextColor) != ""
}

func (t LinkTheme) IsZero() bool {
	return strings.TrimSpace(t.AccentColor) == "" &&
		strings.TrimSpace(t.BgStartColor) == "" &&
		strings.TrimSpace(t.BgEndColor) == "" &&
		strings.TrimSpace(t.BorderColor) == "" &&
		strings.TrimSpace(t.TextColor) == ""
}

func buildLinkThemeFromIcon(uploadDir, iconCachedPath string) (LinkTheme, error) {
	localPath := runtimepath.LocalUploadPathFromPublic(uploadDir, strings.TrimSpace(iconCachedPath))
	if localPath == "" {
		return LinkTheme{}, fmt.Errorf("icon path is not a local upload")
	}

	ext := strings.ToLower(filepath.Ext(localPath))
	if ext == ".ico" {
		return LinkTheme{}, fmt.Errorf("ico theme decoding is not supported")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return LinkTheme{}, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return LinkTheme{}, err
	}

	base, ok := dominantColor(img)
	if !ok {
		return LinkTheme{}, fmt.Errorf("no visible pixels")
	}
	return themeFromColor(base), nil
}

func buildLinkThemeFromSeed(rawURL, title string) LinkTheme {
	seed := strings.TrimSpace(NormalizeIconDomain(rawURL))
	if seed == "" {
		seed = strings.TrimSpace(title)
	}
	if seed == "" {
		seed = "tiledock"
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(seed))
	hue := float64(hasher.Sum32() % 360)

	return LinkTheme{
		AccentColor:  hslHex(hue, 62, 60),
		BgStartColor: hslHex(hue, 40, 26),
		BgEndColor:   hslHex(math.Mod(hue+10, 360), 36, 16),
		BorderColor:  hslHex(hue, 50, 44),
		TextColor:    "#F8FBFF",
	}
}

type rgbColor struct {
	R float64
	G float64
	B float64
}

type weightedBucket struct {
	R      float64
	G      float64
	B      float64
	Weight float64
}

func dominantColor(img image.Image) (rgbColor, bool) {
	const sampleSize = 24

	canvas := image.NewRGBA(image.Rect(0, 0, sampleSize, sampleSize))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), img, img.Bounds(), xdraw.Over, nil)

	buckets := make(map[int]*weightedBucket)
	var fallback weightedBucket

	for y := 0; y < sampleSize; y++ {
		for x := 0; x < sampleSize; x++ {
			r16, g16, b16, a16 := canvas.At(x, y).RGBA()
			if a16 <= 0 {
				continue
			}

			r := float64(uint8(r16 >> 8))
			g := float64(uint8(g16 >> 8))
			b := float64(uint8(b16 >> 8))
			alpha := float64(uint8(a16>>8)) / 255.0

			maxChannel := math.Max(r, math.Max(g, b))
			minChannel := math.Min(r, math.Min(g, b))
			sat := 0.0
			if maxChannel > 0 {
				sat = (maxChannel - minChannel) / maxChannel
			}
			luma := (0.2126*r + 0.7152*g + 0.0722*b) / 255.0

			if luma > 0.985 || luma < 0.03 {
				continue
			}

			fallbackWeight := alpha * (0.45 + sat*0.55)
			fallback.R += r * fallbackWeight
			fallback.G += g * fallbackWeight
			fallback.B += b * fallbackWeight
			fallback.Weight += fallbackWeight

			if sat < 0.12 && (luma < 0.12 || luma > 0.9) {
				continue
			}

			weight := alpha * (0.35 + sat*0.95)
			key := (int(r)/32)<<16 | (int(g)/32)<<8 | (int(b) / 32)
			bucket := buckets[key]
			if bucket == nil {
				bucket = &weightedBucket{}
				buckets[key] = bucket
			}
			bucket.R += r * weight
			bucket.G += g * weight
			bucket.B += b * weight
			bucket.Weight += weight
		}
	}

	var best *weightedBucket
	for _, bucket := range buckets {
		if bucket.Weight <= 0 {
			continue
		}
		if best == nil || bucket.Weight > best.Weight {
			best = bucket
		}
	}

	if best != nil && best.Weight > 0 {
		return rgbColor{
			R: best.R / best.Weight,
			G: best.G / best.Weight,
			B: best.B / best.Weight,
		}, true
	}
	if fallback.Weight > 0 {
		return rgbColor{
			R: fallback.R / fallback.Weight,
			G: fallback.G / fallback.Weight,
			B: fallback.B / fallback.Weight,
		}, true
	}
	return rgbColor{}, false
}

func themeFromColor(base rgbColor) LinkTheme {
	h, s, l := rgbToHSL(base)

	accentS := clampFloat(math.Max(s, 0.48), 0.48, 0.82)
	accentL := clampFloat(math.Max(l, 0.56), 0.52, 0.68)
	bgS := clampFloat(math.Max(s*0.62, 0.24), 0.24, 0.54)
	borderS := clampFloat(math.Max(s*0.82, 0.34), 0.34, 0.72)

	return LinkTheme{
		AccentColor:  hslHex(h, accentS*100, accentL*100),
		BgStartColor: hslHex(h, bgS*100, 26),
		BgEndColor:   hslHex(math.Mod(h+12, 360), clampFloat(math.Max(bgS*0.86, 0.22), 0.22, 0.5)*100, 15),
		BorderColor:  hslHex(h, borderS*100, 43),
		TextColor:    "#F8FBFF",
	}
}

func rgbToHSL(c rgbColor) (float64, float64, float64) {
	r := clampFloat(c.R/255.0, 0, 1)
	g := clampFloat(c.G/255.0, 0, 1)
	b := clampFloat(c.B/255.0, 0, 1)

	maxValue := math.Max(r, math.Max(g, b))
	minValue := math.Min(r, math.Min(g, b))
	l := (maxValue + minValue) / 2

	if maxValue == minValue {
		return 0, 0, l
	}

	delta := maxValue - minValue
	s := delta / (1 - math.Abs(2*l-1))

	var h float64
	switch maxValue {
	case r:
		h = math.Mod((g-b)/delta, 6)
	case g:
		h = (b-r)/delta + 2
	default:
		h = (r-g)/delta + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}

	return h, s, l
}

func hslHex(h, s, l float64) string {
	r, g, b := hslToRGB(h, s/100.0, l/100.0)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := l - c/2

	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}

	return toByte((r1 + m) * 255), toByte((g1 + m) * 255), toByte((b1 + m) * 255)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func toByte(value float64) uint8 {
	value = math.Round(clampFloat(value, 0, 255))
	return uint8(value)
}
