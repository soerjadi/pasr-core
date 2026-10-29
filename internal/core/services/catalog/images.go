package catalog

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

var (
	errImageInvalid = domain.NewError(domain.ErrValidationFailed, "IMAGE_INVALID", "file is not a valid JPEG, PNG or WebP image")
	errImageTooBig  = domain.NewError(domain.ErrValidationFailed, "IMAGE_TOO_LARGE", "image file is too large")
	errImagePixels  = domain.NewError(domain.ErrValidationFailed, "IMAGE_INVALID", "image has too many pixels")
)

// variantWidths are the target widths of the small, medium and large variants.
var variantWidths = [3]int{320, 640, 1080}

// variant is one encoded JPEG.
type variant struct {
	data []byte
}

// processImage validates data by content and returns the small, medium and large JPEG variants.
// The header is checked with DecodeConfig first so oversized images are rejected before any
// pixel buffer is allocated.
func processImage(data []byte, maxPixels int64) ([3]variant, error) {
	var out [3]variant

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return out, errImageInvalid
	}
	switch format {
	case "jpeg", "png", "webp":
	default:
		return out, errImageInvalid
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return out, errImageInvalid
	}
	if maxPixels > 0 && int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return out, errImagePixels
	}

	src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return out, errImageInvalid
	}
	src = flattenOnWhite(src)

	for i, w := range variantWidths {
		img := src
		if src.Bounds().Dx() > w { // never upscale
			img = imaging.Resize(src, w, 0, imaging.Lanczos)
		}
		var buf bytes.Buffer
		if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(80)); err != nil {
			return out, fmt.Errorf("catalog.processImage encode: %w", err)
		}
		out[i] = variant{data: buf.Bytes()}
	}
	return out, nil
}

// flattenOnWhite replaces transparency with white, since JPEG has no alpha channel.
func flattenOnWhite(src image.Image) image.Image {
	if o, ok := src.(interface{ Opaque() bool }); ok && o.Opaque() {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	return dst
}
