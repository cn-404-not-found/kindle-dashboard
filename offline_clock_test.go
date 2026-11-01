package main

import (
	"archive/tar"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

func TestLocalClockLeavesRestOfTopSectionUnchanged(t *testing.T) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	draw := func(hide bool) *gg.Context {
		dc := gg.NewContext(int(canvasW), int(canvasH))
		dc.SetColor(color.White)
		dc.Clear()
		r := &renderer{dc: dc, reg: f, bold: f, faces: make(map[faceKey]font.Face), layoutTop: topSectionY}
		drawTopSection(r, DashboardData{TimeStr: "10:28", DateStr: "2026年11月1日"}, hide)
		return dc
	}
	with, without := draw(false).Image(), draw(true).Image()
	differences := 0
	for y := 0; y < int(canvasH); y++ {
		for x := 0; x < int(canvasW); x++ {
			if with.At(x, y) != without.At(x, y) {
				differences++
				if x > 450 || y > 300 {
					t.Fatalf("unexpected change outside clock at %d,%d", x, y)
				}
			}
		}
	}
	if differences == 0 {
		t.Fatal("clock area did not change")
	}
}

func TestClockTilesRejectsNonMatchingFont(t *testing.T) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clockTiles(f, f); err == nil {
		t.Fatal("expected incompatible glyph width to fail")
	}
}

func TestClockTilesMatchOriginalClock(t *testing.T) {
	reg, err := loadSCFont(fontFile)
	if err != nil {
		t.Skip("Sarasa TTC unavailable")
	}
	bold, err := loadSCFont(boldFontFile)
	if err != nil {
		bold = reg
	}
	packed, err := clockTiles(reg, bold)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(packed))
	tiles := map[rune]image.Image{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(tr)
		if err != nil {
			t.Fatal(err)
		}
		key := []rune(hdr.Name)[0]
		if hdr.Name == "colon.png" {
			key = ':'
		}
		tiles[key] = img
	}
	for _, top := range []float64{legacyTopSectionY, topSectionY} {
		tileY := int(110 + top - legacyTopSectionY)
		for _, text := range []string{"00:00", "18:58", "23:47"} {
			dc := gg.NewContext(400, 260)
			dc.SetColor(color.White)
			dc.Clear()
			r := &renderer{dc: dc, reg: reg, bold: bold, faces: make(map[faceKey]font.Face), layoutTop: top}
			r.text(45, baselineFromTop(top+(250-128)/2, 128), text, 128, color.Black, true, 0)
			for pos, ch := range text {
				tile := tiles[ch]
				if tile == nil {
					t.Fatalf("missing glyph %c", ch)
				}
				for y := 0; y < 128; y++ {
					for x := 0; x < 64; x++ {
						want := color.GrayModel.Convert(dc.Image().At(45+pos*64+x, tileY+y))
						got := color.GrayModel.Convert(tile.At(x, y))
						if got != want {
							t.Fatalf("layout %.0f %s at %d,%d differs", top, text, 45+pos*64+x, tileY+y)
						}
					}
				}
			}
		}
	}
}

func TestWeatherUpdateLabel(t *testing.T) {
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.FixedZone("CST", 8*3600))
	for _, tc := range []struct {
		data DashboardData
		want string
	}{
		{DashboardData{}, "天气更新 --"},
		{DashboardData{WeatherAvailable: true, WeatherUpdatedAt: at}, "天气更新 11-01 09:00"},
		{DashboardData{WeatherAvailable: true, WeatherUpdatedAt: at, WeatherStatus: "数据未更新"}, "天气未更新 11-01 09:00"},
	} {
		if got := weatherUpdateLabel(tc.data); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestDashboardLayoutCompatibility(t *testing.T) {
	for _, tc := range []struct {
		local  bool
		header string
		want   float64
	}{
		{false, "", topSectionY}, {true, "", legacyTopSectionY},
		{true, "top-v2", topSectionY}, {true, "unknown", legacyTopSectionY},
	} {
		if got := dashboardLayoutTop(tc.local, tc.header); got != tc.want {
			t.Errorf("local=%v header=%q: %v, want %v", tc.local, tc.header, got, tc.want)
		}
	}
}
