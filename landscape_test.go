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
