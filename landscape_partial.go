package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// landscapePartialPNG extracts a device-oriented region from the exact PW3
// framebuffer. Coordinates correspond to the clockwise rotation of the layout.
func landscapePartialPNG(src []byte, part string) ([]byte, image.Rectangle, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	if img.Bounds() != image.Rect(0, 0, 1072, 1448) {
		return nil, image.Rectangle{}, fmt.Errorf("unsupported framebuffer %v", img.Bounds())
	}
	var area image.Rectangle
	switch part {
	case "clock":
		area = image.Rect(852, 55, 1020, 599)
	case "weather":
		area = image.Rect(182, 55, 701, 599)
	default:
		return nil, image.Rectangle{}, fmt.Errorf("unknown region %q", part)
	}
	patch := image.NewGray(image.Rect(0, 0, area.Dx(), area.Dy()))
	draw.Draw(patch, patch.Bounds(), img, area.Min, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, patch); err != nil {
		return nil, image.Rectangle{}, err
	}
	return out.Bytes(), area, nil
}
