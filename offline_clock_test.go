package main

import (
	"image/color"
	"testing"

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
		r := &renderer{dc: dc, reg: f, bold: f, faces: make(map[faceKey]font.Face)}
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
