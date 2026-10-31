package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestEncodeGrayscalePNG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.Black)
	src.Set(1, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	result, err := encodeGrayscalePNG(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) < 26 || result[25] != 0 {
		t.Fatalf("PNG color type = %d, want grayscale (0)", result[25])
	}
	img, err := png.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 1 {
		t.Fatalf("unexpected dimensions %v", img.Bounds())
	}
	if color.GrayModel.Convert(img.At(0, 0)).(color.Gray).Y != 0 || color.GrayModel.Convert(img.At(1, 0)).(color.Gray).Y != 255 {
		t.Fatal("black or white pixels changed")
	}
}
func TestEncodeGrayscaleRejectsInvalidPNG(t *testing.T) {
	if _, err := encodeGrayscalePNG([]byte("invalid")); err == nil {
		t.Fatal("expected decode error")
	}
}
