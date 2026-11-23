// opencode_page.go —— OpenCode Go 订阅用量页面（A 版整行式）
//
// 页面尺寸与横屏看板一致：1448×1072，由 screen.png 的统一缩放流程适配到
// 1072×1448（PW3）或 758×1024（PW2）。只使用黑白灰，避免墨水屏出现中间色。
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

const (
	opencodeCanvasW = 1448.0
	opencodeCanvasH = 1072.0

	// 页头时钟的排版锚点与字号。
	opencodeClockCenterX  = 840.0
	opencodeClockBaseline = 93.0
	opencodeClockTimeSize = 54.0
	opencodeClockDateSize = 25.0
)

// opencodeDisplayRow 是页面真正绘制的一行，占位行用 Percent=-1 表示暂无数据。
type opencodeDisplayRow struct {
	Label   string
	Percent int
	Reset   string
	Badge   string
}

// renderOpencodeDashboard 渲染 OpenCode Go 用量页。
// pageIndex/pageTotal 用于底部页码点；now 决定“更新于/重置”的展示时区。
func renderOpencodeDashboard(usage OpencodeUsage, now time.Time, pageIndex, pageTotal int) (result []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("渲染 OpenCode 页面失败: %v", p)
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
	dc := gg.NewContext(int(opencodeCanvasW), int(opencodeCanvasH))
	dc.SetColor(color.White)
	dc.Clear()
	r := &renderer{dc: dc, reg: regular, bold: bold, faces: map[faceKey]font.Face{}}

	black := color.Color(color.Black)
	gray := color.RGBA{95, 95, 95, 255}
	light := color.RGBA{170, 170, 170, 255}
	white := color.Color(color.White)

	title := "OpenCode Go"
	titleWidth := r.measure(title, 50, true)
	r.text(55, 92, title, 50, black, true, 0)
	r.text(55+titleWidth+26, 90, "订阅用量", 28, gray, false, 0)
	r.text(1393, 88, opencodeUpdatedLabel(usage, now), 28, gray, false, 1)
	// 页头中间常驻当前时间与日期；固定中心点，保证局刷区域不会随文字宽度漂移。
	clock, date := opencodeHeaderClock(now)
	clockWidth := r.measure(clock, opencodeClockTimeSize, true)
	dateWidth := r.measure(date, opencodeClockDateSize, false)
	const clockGap = 22.0
	clockX := opencodeClockCenterX - (clockWidth+clockGap+dateWidth)/2
	r.text(clockX, opencodeClockBaseline, clock, opencodeClockTimeSize, black, true, 0)
	r.text(clockX+clockWidth+clockGap, opencodeClockBaseline, date, opencodeClockDateSize, gray, false, 0)
	dc.SetColor(black)
	dc.SetLineWidth(3)
	dc.DrawLine(55, 126, 1393, 126)
	dc.Stroke()

	rows := opencodeDisplayRows(usage, now)
	tops := opencodeRowTops(len(rows))
	for i, row := range rows {
		top := tops[i]
		r.text(55, top+40, row.Label, 40, black, true, 0)
		r.text(1393, top+38, row.Reset, 28, gray, false, 1)

		pctText := "—"
		if row.Percent >= 0 {
			pctText = fmt.Sprintf("%d%%", row.Percent)
		}
		pctWidth := r.measure(pctText, 122, true)
		r.text(55, top+168, pctText, 122, black, true, 0)
		if row.Percent >= 0 {
			remaining := fmt.Sprintf("剩余 %d%%", 100-row.Percent)
			remainingX := 55 + pctWidth + 38
			remainingWidth := r.measure(remaining, 30, false)
			r.text(remainingX, top+150, remaining, 30, gray, false, 0)
			if row.Badge != "" {
				badgeX := remainingX + remainingWidth + 34
				dc.SetColor(black)
				dc.DrawRectangle(badgeX, top+112, 168, 46)
				dc.Fill()
				r.text(badgeX+84, top+148, row.Badge, 26, white, true, 0.5)
			}
		}
		drawOpencodeBar(dc, 55, top+198, 1338, 32, row.Percent, gray)
	}

	dc.SetColor(light)
	dc.SetLineWidth(2)
	dc.DrawLine(55, 962, 1393, 962)
	dc.Stroke()
	r.fitText(55, 1016, opencodeFooterText(usage, now), 24, 880, gray, false, 0)
	drawPageDots(dc, 1080, 1008, pageTotal, pageIndex, gray)
	r.text(1393, 1014, fmt.Sprintf("第 %d 页 / 共 %d 页", pageIndex, pageTotal), 24, gray, false, 1)

	var out bytes.Buffer
	if err := png.Encode(&out, dc.Image()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// opencodeHeaderClock 返回页头显示的时间与日期；日期含星期，便于翻页后仍能快速看时间。
func opencodeHeaderClock(now time.Time) (string, string) {
	date := fmt.Sprintf("%d月%d日 %s", now.Month(), now.Day(), weekdayNames[int(now.Weekday())])
	return now.Format("15:04"), date
}

// opencodeDisplayRows 把用量快照转换为固定顺序的展示行；无数据时返回三个占位行。
func opencodeDisplayRows(usage OpencodeUsage, now time.Time) []opencodeDisplayRow {
	if !usage.Available || len(usage.Windows) == 0 {
		return []opencodeDisplayRow{
			{Label: "5 小时窗口", Percent: -1, Reset: "重置时间未知"},
			{Label: "本 周", Percent: -1, Reset: "重置时间未知"},
			{Label: "本 月", Percent: -1, Reset: "重置时间未知"},
		}
	}
	rows := make([]opencodeDisplayRow, 0, len(usage.Windows))
	for _, window := range usage.Windows {
		rows = append(rows, opencodeDisplayRow{
			Label:   window.Label,
			Percent: window.Percent,
			Reset:   opencodeResetLabel(window.ResetAt, now),
			Badge:   opencodeBadge(window),
		})
	}
	return rows
}

// opencodeRowTops 返回每行的起始 y；行数变化时保持整体纵向居中。
func opencodeRowTops(count int) []float64 {
	switch count {
	case 0:
		return nil
	case 1:
		return []float64{340}
	case 2:
		return []float64{210, 540}
	case 3:
		return []float64{172, 436, 700}
	default:
		tops := make([]float64, count)
		for i := range tops {
			tops[i] = 150 + float64(i)*240
		}
		return tops
	}
}

// opencodeBadge 返回 90% 以上时的提示文字。
func opencodeBadge(window OpencodeWindow) string {
	if window.Percent >= 100 {
		return "已用尽"
	}
	if window.Status != "" && window.Status != "ok" {
		return "受限"
	}
	if window.Percent >= 90 {
		return "接近上限"
	}
	return ""
}

// opencodeUpdatedLabel 返回右上角的更新时间。
func opencodeUpdatedLabel(usage OpencodeUsage, now time.Time) string {
	if usage.FetchedAt.IsZero() {
		return "尚未获取"
	}
	return "更新于 " + usage.FetchedAt.In(now.Location()).Format("15:04")
}

// opencodeResetLabel 用绝对时间描述重置点：当天只显示时分，一周内带星期。
func opencodeResetLabel(reset, now time.Time) string {
	if reset.IsZero() {
		return "重置时间未知"
	}
	local := reset.In(now.Location())
	if !local.After(now) {
		return "即将重置"
	}
	days := int(local.Sub(now).Hours() / 24)
	switch {
	case days == 0:
		return local.Format("15:04") + " 重置"
	case days < 7:
		return weekdayNames[int(local.Weekday())] + " " + local.Format("15:04") + " 重置"
	default:
		return local.Format("1月2日 15:04") + " 重置"
	}
}

// opencodeFooterText 组合数据来源、过期提示与错误信息。
func opencodeFooterText(usage OpencodeUsage, now time.Time) string {
	text := fmt.Sprintf("数据来源 opencode.ai/zen/go/v1/usage · %s更新 · 虚线为 80%% 警戒线", opencodeIntervalLabel())
	if usage.Err != "" {
		if len(usage.Windows) > 0 {
			text = "更新失败：" + usage.Err + " · 显示上次数据"
		} else {
			text = usage.Err
		}
	}
	if !usage.FetchedAt.IsZero() {
		if minutes := int(now.Sub(usage.FetchedAt).Minutes()); minutes > 15 {
			text = fmt.Sprintf("数据已过期 %d 分钟 · %s", minutes, text)
		}
	}
	return text
}

// opencodeIntervalLabel 把轮询间隔转成页面底部的简短说明。
func opencodeIntervalLabel() string {
	seconds := int(opencodePollInterval / time.Second)
	if seconds < 60 {
		seconds = 60
	}
	if seconds == 60 {
		return "每分钟"
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("每 %d 分钟", seconds/60)
	}
	return fmt.Sprintf("每 %d 秒", seconds)
}

// drawOpencodeBar 绘制带 80% 警戒线的横向进度条；percent<0 时只画空条。
func drawOpencodeBar(dc *gg.Context, x, y, w, h float64, percent int, gray color.Color) {
	dc.SetColor(color.Black)
	dc.SetLineWidth(3)
	dc.DrawRectangle(x, y, w, h)
	dc.Stroke()
	if percent > 0 {
		fill := (w - 6) * float64(percent) / 100
		dc.DrawRectangle(x+3, y+3, fill, h-6)
		dc.Fill()
	}
	markerX := x + 3 + (w-6)*0.8
	dc.SetColor(gray)
	dc.SetLineWidth(2)
	dc.SetDash(2, 5)
	dc.DrawLine(markerX, y-8, markerX, y+h+8)
	dc.Stroke()
	dc.SetDash()
}

// drawPageDots 绘制底部页码点；total<=1 时不画。
func drawPageDots(dc *gg.Context, cx, cy float64, total, active int, gray color.Color) {
	if total <= 1 {
		return
	}
	const radius, gap = 7.0, 34.0
	start := cx - float64(total-1)*gap/2
	for i := 0; i < total; i++ {
		x := start + float64(i)*gap
		if i == active-1 {
			dc.SetColor(color.Black)
			dc.DrawCircle(x, cy, radius)
			dc.Fill()
			continue
		}
		dc.SetColor(gray)
		dc.SetLineWidth(2)
		dc.DrawCircle(x, cy, radius)
		dc.Stroke()
	}
}

// measure 返回指定字号/字重下的文本宽度，用于动态排版。
func (r *renderer) measure(s string, size float64, bold bool) float64 {
	r.dc.SetFontFace(r.face(size, bold))
	width, _ := r.dc.MeasureString(s)
	return width
}
