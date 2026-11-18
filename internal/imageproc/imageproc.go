// Package imageproc validates and normalizes user uploaded images.
//
// Every image is decoded and re-encoded: this strips metadata (such as GPS
// coordinates in photos), bounds the size of what is stored and guarantees
// that only well-formed JPEG or PNG data is ever served back.
package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registers the GIF decoder
	"image/jpeg"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

// Limits applied to uploads.
const (
	MaxUploadBytes = 10 << 20 // size of the uploaded file
	MaxPixels      = 50e6     // decoded size, guards against decompression bombs
	MaxSide        = 1600     // longest side of the stored image
	jpegQuality    = 85
)

var (
	// ErrTooLarge is returned when the file or its dimensions exceed the limits.
	ErrTooLarge = errors.New("image too large")
	// ErrUnsupported is returned for anything that is not a JPEG, PNG, GIF or WebP image.
	ErrUnsupported = errors.New("unsupported image")
)

// Image is a normalized image.
type Image struct {
	ContentType string
	Data        []byte
}

// Normalize reads an image, scales it down to fit MaxSide and re-encodes it:
// as PNG when it has transparency, as JPEG otherwise.
func Normalize(r io.Reader) (Image, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return Image{}, err
	}
	if len(raw) > MaxUploadBytes {
		return Image{}, ErrTooLarge
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Image{}, ErrUnsupported
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Image{}, ErrUnsupported
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Image{}, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Image{}, ErrUnsupported
	}
	img := fit(src, MaxSide)

	var out bytes.Buffer
	if opaque(img) {
		if err := jpeg.Encode(&out, flatten(img), &jpeg.Options{Quality: jpegQuality}); err != nil {
			return Image{}, err
		}
		return Image{ContentType: "image/jpeg", Data: out.Bytes()}, nil
	}
	if err := png.Encode(&out, img); err != nil {
		return Image{}, err
	}
	return Image{ContentType: "image/png", Data: out.Bytes()}, nil
}

// fit scales img down (never up) so that its longest side is at most side.
func fit(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		return img
	}
	if w >= h {
		h = max(1, h*side/w)
		w = side
	} else {
		w = max(1, w*side/h)
		h = side
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// opaque reports whether every pixel of img is fully opaque.
func opaque(img image.Image) bool {
	// All standard image types implement Opaque; treat others as transparent.
	o, ok := img.(interface{ Opaque() bool })
	return ok && o.Opaque()
}

// flatten draws img on a white background, which also converts paletted and
// other color models to something the JPEG encoder handles well.
func flatten(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	return dst
}
