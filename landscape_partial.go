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
		area = image.Rect(136, 55, 701, 599)
	case "news":
		// 横屏右侧热榜 (680,40)-(1410,1036) 顺时针旋转 90° 后的竖屏坐标。
		area = image.Rect(36, 680, 1032, 1410)
	case "pageheader":
		// 横屏页头 (620,24)-(1400,124) 顺时针旋转 90° 后的竖屏坐标。
		// 覆盖时间、日期和右侧“更新于”标签，不含标题与分隔线。
		area = image.Rect(948, 620, 1048, 1400)
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
