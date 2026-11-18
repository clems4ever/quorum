package imageproc

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func solid(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func decode(t *testing.T, img Image) image.Image {
	t.Helper()
	out, format, err := image.Decode(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	if "image/"+format != img.ContentType {
		t.Fatalf("content type %s but data is %s", img.ContentType, format)
	}
	return out
}

func TestOpaqueImagesBecomeJPEG(t *testing.T) {
	var src bytes.Buffer
	if err := jpeg.Encode(&src, solid(40, 20, color.NRGBA{200, 10, 10, 255}), nil); err != nil {
		t.Fatal(err)
	}
	img, err := Normalize(&src)
	if err != nil {
		t.Fatal(err)
	}
	out := decode(t, img)
	if img.ContentType != "image/jpeg" || out.Bounds().Dx() != 40 || out.Bounds().Dy() != 20 {
		t.Fatalf("got %s %v", img.ContentType, out.Bounds())
	}
}

func TestTransparentImagesStayPNG(t *testing.T) {
	img, err := Normalize(bytes.NewReader(encodePNG(t, solid(10, 10, color.NRGBA{0, 0, 255, 100}))))
	if err != nil {
		t.Fatal(err)
	}
	if img.ContentType != "image/png" {
		t.Fatalf("got %s", img.ContentType)
	}
	if _, _, _, a := decode(t, img).At(5, 5).RGBA(); a == 0xffff {
		t.Fatal("transparency lost")
	}
}

func TestLargeImagesAreScaledDown(t *testing.T) {
	cases := []struct{ w, h, wantW, wantH int }{
		{3200, 800, MaxSide, 400},
		{800, 3200, 400, MaxSide},
		{4000, 1, MaxSide, 1},
	}
	for _, c := range cases {
		img, err := Normalize(bytes.NewReader(encodePNG(t, solid(c.w, c.h, color.White))))
		if err != nil {
			t.Fatal(err)
		}
		if b := decode(t, img).Bounds(); b.Dx() != c.wantW || b.Dy() != c.wantH {
			t.Errorf("%dx%d scaled to %v, want %dx%d", c.w, c.h, b, c.wantW, c.wantH)
		}
	}
}

func TestGIFAndWebP(t *testing.T) {
	var g bytes.Buffer
	pal := image.NewPaletted(image.Rect(0, 0, 4, 4), []color.Color{color.Black, color.White})
	if err := gif.Encode(&g, pal, nil); err != nil {
		t.Fatal(err)
	}
	// A 1x1 lossless WebP, as used by browser feature detection.
	webp, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"gif": g.Bytes(), "webp": webp} {
		img, err := Normalize(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		decode(t, img)
	}
}

func TestRejections(t *testing.T) {
	huge := encodePNG(t, image.NewGray(image.Rect(0, 0, 10000, 6000))) // 60 MP, compresses well
	cases := map[string]struct {
		data []byte
		want error
	}{
		"not an image":     {[]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), ErrUnsupported},
		"truncated":        {encodePNG(t, solid(8, 8, color.White))[:60], ErrUnsupported},
		"too many pixels":  {huge, ErrTooLarge},
		"file too large":   {make([]byte, MaxUploadBytes+1), ErrTooLarge},
		"empty dimensions": {[]byte("GIF89a\x00\x00\x00\x00\x00\x00\x00"), ErrUnsupported},
	}
	for name, c := range cases {
		if _, err := Normalize(bytes.NewReader(c.data)); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }

func TestReadError(t *testing.T) {
	if _, err := Normalize(failingReader{}); err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v", err)
	}
}
