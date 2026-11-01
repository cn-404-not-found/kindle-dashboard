package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
)

// clockTiles generates the original monospaced clock glyphs as grayscale PNGs.
// The 64×128 cells correspond to the base canvas; a white background erases
// the preceding digit when Kindle draws a tile over it.
func clockTiles(reg, bold *sfnt.Font) ([]byte, error) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	for _, ch := range "0123456789:" {
		dc := gg.NewContext(400, 260)
		dc.SetColor(color.White)
		dc.Clear()
		r := &renderer{dc: dc, reg: reg, bold: bold, faces: make(map[faceKey]font.Face)}
		r.dc.SetFontFace(r.face(128, true))
		for _, test := range "0123456789:" {
			w, _ := r.dc.MeasureString(string(test))
			if w != 64 {
				return nil, fmt.Errorf("clock font has unsupported glyph width %.2f", w)
			}
		}
		baseline := baselineFromTop(50+(250-128)/2, 128)
		r.text(45, baseline, string(ch), 128, color.Black, true, 0)
		tile := image.NewGray(image.Rect(0, 0, 64, 128))
		draw.Draw(tile, tile.Bounds(), dc.Image(), image.Pt(45, 110), draw.Src)
		var b bytes.Buffer
		if err := png.Encode(&b, tile); err != nil {
			return nil, err
		}
		name := string(ch)
		if ch == ':' {
			name = "colon"
		}
		hdr := &tar.Header{Name: name + ".png", Mode: 0644, Size: int64(b.Len()), ModTime: time.Unix(0, 0)}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(b.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

func clockTilesHandler(w http.ResponseWriter, r *http.Request) {
	reg, err := loadSCFont(fontFile)
	if err != nil {
		http.Error(w, "clock font unavailable", http.StatusInternalServerError)
		return
	}
	bold, err := loadSCFont(boldFontFile)
	if err != nil {
		bold = reg
	}
	b, err := clockTiles(reg, bold)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Cache-Control", "no-store")
	if _, err = w.Write(b); err != nil {
		return
	}
}
