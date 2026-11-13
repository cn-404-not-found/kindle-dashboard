package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"

	"github.com/6tail/lunar-go/calendar"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

// renderLandscapeDashboard renders the online layout at its native landscape size.
func renderLandscapeDashboard(data DashboardData) (result []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("渲染横屏看板失败: %v", p)
		}
	}()
	regular, err := loadCalendarFont(fontFile)
	if err != nil {
		return nil, err
	}
	bold, err := loadCalendarFont(boldFontFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		bold = regular
	}
	dc := gg.NewContext(1448, 1072)
	dc.SetColor(color.White)
	dc.Clear()
	r := &renderer{dc: dc, reg: regular, bold: bold, faces: map[faceKey]font.Face{}}
	black, gray, border := color.Black, color.RGBA{70, 70, 70, 255}, color.RGBA{160, 160, 160, 255}
	line := func(x1, y1, x2, y2, w float64, c color.Color) {
		dc.SetColor(c)
		dc.SetLineWidth(w)
		dc.DrawLine(x1, y1, x2, y2)
		dc.Stroke()
	}
	date := parseDate(data.DateStr)
	center := 327.0
	r.fitText(center, 200, data.TimeStr, 168, 535, black, true, .5)
	r.fitText(center, 282, fmt.Sprintf("%s  星期%s", data.DateStr, []string{"日", "一", "二", "三", "四", "五", "六"}[int(date.Weekday())]), 34, 540, black, true, .5)
	r.fitText(center, 334, "农历 "+data.LunarStr, 27, 540, gray, false, .5)
	line(55, 370, 599, 370, 3, black)
	r.fitText(center, 429, data.Location, 28, 500, black, true, .5)
	if data.WeatherAvailable {
		r.fitText(179, 553, fmt.Sprintf("%.0f°", data.Temp), 105, 210, black, true, .5)
		r.fitText(429, 515, data.WeatherText, 35, 240, black, true, .5)
		r.fitText(429, 568, fmt.Sprintf("最高 %.0f° / 最低 %.0f°", data.HighTemp, data.LowTemp), 28, 250, black, false, .5)
	} else {
		r.text(center, 530, "天气暂不可用", 34, gray, false, .5)
	}
	r.fitText(center, 629, weatherUpdateLabel(data), 21, 540, gray, false, .5)
	line(55, 664, 599, 664, 3, black)
	r.text(center, 719, "未来五日", 25, black, true, .5)
	const boxW = 103.2
	for i := 0; i < 5; i++ {
		x := 55 + float64(i)*(boxW+7)
		dc.SetColor(border)
		dc.SetLineWidth(1)
		dc.DrawRectangle(x, 742, boxW, 147)
		dc.Stroke()
		day := weekdayNames[int(date.AddDate(0, 0, i+1).Weekday())]
		weather, temperature := "--", "--/--"
		if i < len(data.FutureForecast) {
			f := data.FutureForecast[i]
			day = f.Weekday
			weather = f.WeatherText
			temperature = fmt.Sprintf("%d°/%d°", f.HighTemp, f.LowTemp)
		}
		r.fitText(x+boxW/2, 781, day, 26, boxW-4, black, true, .5)
		r.fitText(x+boxW/2, 824, weather, 25, boxW-4, black, true, .5)
		r.fitText(x+boxW/2, 866, temperature, 23, boxW-4, black, true, .5)
	}
	line(55, 959, 599, 959, 1, border)
	lunar := calendar.NewSolarFromYmd(date.Year(), int(date.Month()), date.Day()).GetLunar()
	r.fitText(55, 1012, fmt.Sprintf("%s年 · %s年", lunar.GetYearInGanZhi(), lunar.GetYearShengXiao()), 32, 225, black, false, 0)
	term, termDate, _ := nextSolarTerm(date)
	r.fitText(599, 1012, fmt.Sprintf("%s %d月%d日", term, termDate.Month(), termDate.Day()), 32, 280, black, true, 1)
	line(646, 52, 646, 1020, 2, black)
	left, right, top := 691.0, 1393.0, 188.0
	r.text((left+right)/2, 100, fmt.Sprintf("%d 年 %d 月", date.Year(), date.Month()), 43, black, true, .5)
	colW := (right - left) / 7
	for i, day := range []string{"一", "二", "三", "四", "五", "六", "日"} {
		r.text(left+(float64(i)+.5)*colW, 166, day, 25, black, true, .5)
	}
	line(left, top, right, top, 3, black)
	rowH := 832.0 / 6
	for row := 0; row < 6; row++ {
		for col := 0; col < 7; col++ {
			cell := data.Calendar[row][col]
			x, y := left+float64(col)*colW, top+float64(row)*rowH
			num, sub := color.Color(black), color.Color(gray)
			if cell.IsToday {
				dc.SetColor(black)
				dc.DrawRectangle(x, y, colW, rowH)
				dc.Fill()
				num = color.White
				sub = color.RGBA{220, 220, 220, 255}
			} else {
				dc.SetColor(border)
				dc.SetLineWidth(1)
				if !cell.IsCurrentMonth {
					dc.SetDash(4, 4)
					num = color.RGBA{175, 175, 175, 255}
					sub = num
				}
				dc.DrawRectangle(x, y, colW, rowH)
				dc.Stroke()
				dc.SetDash()
			}
			r.text(x+colW/2, y+65, fmt.Sprint(cell.Day), 46, num, true, .5)
			r.fitText(x+colW/2, y+104, cell.Lunar, 23, colW-8, sub, false, .5)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dc.Image()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
