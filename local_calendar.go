package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/6tail/lunar-go/calendar"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
)

func loadCalendarFont(name string) (*sfnt.Font, error) {
	data, err := os.ReadFile(resolveFontPath(name))
	if err != nil {
		return nil, fmt.Errorf("读取字体 %s: %w", name, err)
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("解析字体 %s: %w", name, err)
	}
	face := selectSC(collection, "Gothic")
	if face == nil {
		return nil, fmt.Errorf("字体 %s 没有可用字形", name)
	}
	return face, nil
}

// nextSolarTerm searches calendar dates rather than adding 24 hours, so local
// date boundaries remain correct even if the caller uses a DST time zone.
func nextSolarTerm(now time.Time) (string, time.Time, int) {
	for days := 1; days <= 30; days++ {
		date := now.AddDate(0, 0, days)
		term := calendar.NewSolarFromYmd(date.Year(), int(date.Month()), date.Day()).GetLunar().GetJieQi()
		if term != "" {
			return term, date, days
		}
	}
	return "", time.Time{}, 0
}

func writeLocalCalendar(path string, now time.Time) error {
	img, err := renderLocalCalendar(now)
	if err != nil {
		return err
	}
	return os.WriteFile(path, img, 0644)
}

func renderLocalCalendar(now time.Time) (result []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("绘制本地日历失败: %v", p)
		}
	}()
	reg, err := loadCalendarFont(fontFile)
	if err != nil {
		return nil, err
	}
	bold, err := loadCalendarFont(boldFontFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		bold = reg
	}
	r := &renderer{dc: gg.NewContext(int(canvasW), int(canvasH)), reg: reg, bold: bold, faces: map[faceKey]font.Face{}}
	dc := r.dc
	dc.SetColor(color.White)
	dc.Clear()
	black, gray := color.Black, color.RGBA{85, 85, 85, 255}
	r.text(52, 260, fmt.Sprintf("%02d", now.Day()), 175, black, true, 0)
	dc.SetColor(color.RGBA{195, 195, 195, 255})
	dc.SetLineWidth(2)
	dc.DrawLine(295, 133, 295, 263)
	dc.DrawLine(715, 110, 715, 282)
	dc.Stroke()
	r.text(333, 182, "星期"+[]string{"日", "一", "二", "三", "四", "五", "六"}[int(now.Weekday())], 46, black, true, 0)
	r.text(333, 252, "农历 "+lunarMonthDay(now.Year(), int(now.Month()), now.Day()), 34, gray, false, 0)
	term, termDate, days := nextSolarTerm(now)
	r.text(747, 148, "下一个节气", 29, gray, false, 0)
	r.text(747, 215, term, 46, black, true, 0)
	r.fitText(747, 269, fmt.Sprintf("%d月%d日 · 还有%d天", termDate.Month(), termDate.Day(), days), 28, 272, gray, false, 0)
	dc.SetColor(black)
	dc.SetLineWidth(3)
	dc.DrawLine(52, 337, 1020, 337)
	dc.Stroke()
	r.text(52, 428, fmt.Sprintf("%d 年 %d 月", now.Year(), now.Month()), 39, black, true, 0)
	monthNames := []string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"}
	r.text(1020, 428, monthNames[int(now.Month())-1], 22, gray, false, 1)
	heads := []string{"一", "二", "三", "四", "五", "六", "日"}
	colW, top, rowH := 968.0/7, 513.0, 132.0
	for c, head := range heads {
		r.text(52+(float64(c)+0.5)*colW, 492, head, 25, black, true, 0.5)
	}
	dc.SetLineWidth(3)
	dc.DrawLine(52, top, 1020, top)
	dc.Stroke()
	grid := buildMonthGrid(now)
	for row := 0; row < 6; row++ {
		for col := 0; col < 7; col++ {
			cell := grid[row][col]
			x, y := 52+float64(col)*colW, top+float64(row)*rowH
			num, sub := color.Color(black), color.Color(gray)
			if cell.IsToday {
				dc.SetColor(black)
				dc.DrawRectangle(x, y, colW, rowH)
				dc.Fill()
				num = color.White
				sub = color.RGBA{220, 220, 220, 255}
			} else {
				if cell.IsCurrentMonth {
					dc.SetColor(color.RGBA{170, 170, 170, 255})
					dc.SetDash()
				} else {
					dc.SetColor(color.RGBA{190, 190, 190, 255})
					dc.SetDash(4, 4)
					num = color.RGBA{176, 176, 176, 255}
					sub = num
				}
				dc.SetLineWidth(1)
				dc.DrawRectangle(x, y, colW, rowH)
				dc.Stroke()
				dc.SetDash()
			}
			r.text(x+colW/2, y+65, strconv.Itoa(cell.Day), 55, num, true, 0.5)
			r.fitText(x+colW/2, y+109, cell.Lunar, 27, colW-8, sub, false, 0.5)
		}
	}
	lunarYear := calendar.NewSolarFromYmd(now.Year(), int(now.Month()), now.Day()).GetLunar().GetYearInGanZhi()
	r.text(52, 1355, fmt.Sprintf("%d · %s年", now.Year(), lunarYear), 20, gray, false, 0)
	r.text(1020, 1355, fmt.Sprintf("%s · %d月%d日", term, termDate.Month(), termDate.Day()), 20, gray, false, 1)
	var buf bytes.Buffer
	grayImage := image.NewGray(dc.Image().Bounds())
	draw.Draw(grayImage, grayImage.Bounds(), dc.Image(), image.Point{}, draw.Src)
	if err := png.Encode(&buf, grayImage); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
