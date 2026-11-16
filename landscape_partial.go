package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// landscapePartialPNG extracts the corresponding region from a supported framebuffer.
func landscapePartialPNG(src []byte, part string) ([]byte, image.Rectangle, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	if img.Bounds() != image.Rect(0, 0, 1072, 1448) && img.Bounds() != image.Rect(0, 0, 758, 1024) {
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
	if img.Bounds().Dx() == 758 {
		// 向外取整，确保缩放后变化区域的边缘也包含在图块中。
		area = image.Rect(area.Min.X*758/1072, area.Min.Y*1024/1448,
			(area.Max.X*758+1071)/1072, (area.Max.Y*1024+1447)/1448)
	}
	patch := image.NewGray(image.Rect(0, 0, area.Dx(), area.Dy()))
	draw.Draw(patch, patch.Bounds(), img, area.Min, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, patch); err != nil {
		return nil, image.Rectangle{}, err
	}
	return out.Bytes(), area, nil
}
