package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestFutureForecastStartsTomorrowAndCrossesMonth(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	forecast := make([]ForecastItem, 6)
	for i := range forecast {
		forecast[i].HighTemp = i
	}
	got := buildFutureForecast(now, weatherResult{forecastDate: now, forecast: forecast})
	if len(got) != 5 || got[0].Weekday != "周六" || got[0].HighTemp != 1 || got[4].Weekday != "周三" || got[4].HighTemp != 5 {
		t.Fatalf("unexpected forecast: %+v", got)
	}
	if got := buildFutureForecast(now.AddDate(0, 0, 1), weatherResult{forecastDate: now, forecast: forecast}); len(got) != 0 {
		t.Fatalf("stale forecast: %+v", got)
	}
	if got := buildFutureForecast(now, weatherResult{forecastDate: now, forecast: forecast[:3]}); len(got) != 2 {
		t.Fatalf("short forecast: %+v", got)
	}
}

func TestFitLandscapeToPortraitRotatesWithoutCropping(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := fitToDevice(buf.Bytes(), 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 3 || decoded.Bounds().Dy() != 4 {
		t.Fatalf("bounds %v", decoded.Bounds())
	}
	if color.RGBAModel.Convert(decoded.At(2, 0)).(color.RGBA).R != 0 {
		t.Fatal("top-left marker was not rotated clockwise")
	}
}

func TestLandscapePartialImagesUseFramebufferCoordinates(t *testing.T) {
	full := image.NewGray(image.Rect(0, 0, 1072, 1448))
	for _, tc := range []struct {
		part string
		rect image.Rectangle
	}{
		{"clock", image.Rect(852, 55, 1020, 599)},
		{"weather", image.Rect(182, 55, 701, 599)},
		{"pageclock", image.Rect(950, 640, 1046, 1040)},
	} {
		full.SetGray(tc.rect.Min.X, tc.rect.Min.Y, color.Gray{Y: 123})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, full); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		part string
		rect image.Rectangle
	}{
		{"clock", image.Rect(852, 55, 1020, 599)},
		{"weather", image.Rect(182, 55, 701, 599)},
		{"pageclock", image.Rect(950, 640, 1046, 1040)},
	} {
		output, rect, err := landscapePartialPNG(buf.Bytes(), tc.part)
		if err != nil || rect != tc.rect {
			t.Fatalf("%s: rect %v err %v", tc.part, rect, err)
		}
		patch, err := png.Decode(bytes.NewReader(output))
		if err != nil {
			t.Fatal(err)
		}
		if patch.Bounds().Dx() != rect.Dx() || patch.Bounds().Dy() != rect.Dy() {
			t.Fatalf("%s: bounds %v", tc.part, patch.Bounds())
		}
		if color.GrayModel.Convert(patch.At(0, 0)).(color.Gray).Y != 123 {
			t.Fatalf("%s: wrong origin", tc.part)
		}
	}
	if _, _, err := landscapePartialPNG(buf.Bytes(), "unknown"); err == nil {
		t.Fatal("unknown region must fail")
	}
}

func TestLandscapePartialPW2(t *testing.T) {
	full := image.NewGray(image.Rect(0, 0, 758, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 758; x++ {
			full.SetGray(x, y, color.Gray{Y: uint8((x + y) % 256)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, full); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		part string
		rect image.Rectangle
	}{
		{"clock", image.Rect(602, 38, 722, 424)},
		{"weather", image.Rect(128, 38, 496, 424)},
		{"pageclock", image.Rect(671, 452, 740, 736)},
	} {
		data, rect, err := landscapePartialPNG(buf.Bytes(), tc.part)
		if err != nil || rect != tc.rect {
			t.Fatalf("%s: rect %v, error %v", tc.part, rect, err)
		}
		patch, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < rect.Dy(); y++ {
			for x := 0; x < rect.Dx(); x++ {
				if patch.At(x, y) != full.At(x+rect.Min.X, y+rect.Min.Y) {
					t.Fatalf("%s: pixel mismatch at %d,%d", tc.part, x, y)
				}
			}
		}
	}
}
