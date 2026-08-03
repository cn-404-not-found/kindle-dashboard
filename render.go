// render.go —— 墨水屏看板 2D 绘制层
//
// 平台：Kindle Paperwhite 3（物理分辨率 1072 × 1448，纯黑白墨水屏）。
// 职责：接收 main.go 中定义的 DashboardData，使用 github.com/fogleman/gg
//       在纯白画布上绘制「时间/天气 / 5 日预报 / 整月日历」三区布局，
//       最终以 image/png 编码返回 PNG 字节流。
//
// 布局严格翻译自项目提供的 HTML/CSS 源码（body 1072×1448，padding 50/45）：
//   1. 顶部核心区：大号时间 + 公历/农历日期（左），圆角天气卡片（右），下方分割线。
//   2. 中部 5 日预报：横向 5 列卡片，IsToday 反色（黑底白字），其余白底黑框。
//   3. 下部月历：7 列 × 6 行网格，非本月浅灰虚线淡字，IsToday 黑底白字高亮。
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/truetype"
)

// ----------------------------------------------------------------------------
// 画布常量（与 CSS 中的 body / padding 一一对应）
// ----------------------------------------------------------------------------
const (
	canvasW = 1072.0 // Kindle Paperwhite 3 物理宽度
	canvasH = 1448.0 // Kindle Paperwhite 3 物理高度
	padX    = 45.0   // body padding-left/right
	padY    = 50.0   // body padding-top/bottom

	fontFile = "msyh.ttc" // 同目录中文字体（微软雅黑 TrueType Collection）

	asc = 0.86 // CJK 字形近似 ascent 比例，用于将「顶对齐」换算为基线 y
)

// renderer 封装绘制上下文与按尺寸缓存的字体面，避免重复解析 TTC。
type renderer struct {
	dc    *gg.Context
	col   *truetype.Collection
	faces map[float64]font.Face
}

// face 返回指定字号（pt，1:1 映射 CSS px）的字体面，并按需缓存。
// msyh.ttc 不含粗体，粗体通过「描边 + 填充」在 text() 中模拟。
func (r *renderer) face(size float64) font.Face {
	if f, ok := r.faces[size]; ok {
		return f
	}
	f := r.col.NewFace(0, &truetype.Options{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	r.faces[size] = f
	return f
}

// text 以基线位于 (x, y) 绘制字符串。
//   - hAlign：0=左对齐(x为左缘)，0.5=居中(x为中心)，1=右对齐(x为右缘)。
//   - bold：为 true 时在当前颜色上叠加同色细描边，模拟粗体（墨水屏观感更扎实）。
func (r *renderer) text(x, y float64, s string, size float64, c color.Color, bold bool, hAlign float64) {
	if s == "" {
		return
	}
	r.dc.SetFontFace(r.face(size))
	r.dc.SetColor(c)
	if bold {
		r.dc.SetLineWidth(1.8)
		r.dc.StrokeStringAnchored(s, x, y, hAlign, 0)
	}
	r.dc.DrawStringAnchored(s, x, y, hAlign, 0)
}

// baselineFromTop 把「文字上沿 yTop」换算为「文字基线 y」（用于纵向堆叠排版）。
func baselineFromTop(yTop, size float64) float64 {
	return yTop + size*asc
}

// ----------------------------------------------------------------------------
// 对外入口
// ----------------------------------------------------------------------------

// RenderDashboard 将看板数据绘制为 1072×1448 的 PNG 字节流。
// 字体 msyh.ttc 必须与可执行文件同目录；缺失或损坏时返回 error。
func RenderDashboard(data DashboardData) ([]byte, error) {
	// 1. 加载中文字体（TTC 合集，优先取可执行文件同目录，回退到工作目录）。
	fontData, err := os.ReadFile(resolveFontPath())
	if err != nil {
		return nil, fmt.Errorf("读取字体文件 %s 失败: %w", fontFile, err)
	}
	col, err := truetype.ParseCollection(fontData)
	if err != nil {
		return nil, fmt.Errorf("解析字体合集 %s 失败: %w", fontFile, err)
	}

	// 2. 初始化纯白画布。
	dc := gg.NewContext(int(canvasW), int(canvasH))
	dc.SetColor(color.White)
	dc.DrawRectangle(0, 0, canvasW, canvasH)
	dc.Fill()

	r := &renderer{dc: dc, col: col, faces: make(map[float64]font.Face)}

	// 基准日期：用于推导 5 日预报的 MM/DD 与月历标题（数据体本身不含这些字段）。
	baseDate := parseDate(data.DateStr)

	drawTopSection(r, data)
	drawForecast(r, data, baseDate)
	drawCalendar(r, data, baseDate)

	// 3. 编码 PNG。
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}
	return buf.Bytes(), nil
}

// resolveFontPath 优先返回「可执行文件同目录」下的字体路径，否则回退到相对路径。
func resolveFontPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), fontFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return fontFile // 回退：依赖进程工作目录中存在该文件
}

// parseDate 从 "2006年01月02日" 解析出日期；解析失败时回退到当前本地时间。
func parseDate(s string) time.Time {
	var y, mo, d int
	if _, err := fmt.Sscanf(s, "%d年%d月%d日", &y, &mo, &d); err == nil && y > 0 {
		return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	}
	return time.Now()
}

// ----------------------------------------------------------------------------
// 1. 顶部核心区：时间 + 日期（左） / 圆角天气卡片（右） / 分割线
// ----------------------------------------------------------------------------

func drawTopSection(r *renderer, data DashboardData) {
	left := padX
	right := canvasW - padX
	sectionTop := padY        // 50
	sectionH := 250.0         // CSS .top-section height
	sectionBottom := sectionTop + sectionH // 300

	// ---- 左侧时间区（纵向居中）----
	const (
		timeSize  = 115.0
		dateSize  = 32.0
		lunarSize = 24.0
	)
	blockH := timeSize + 15 + dateSize + 6 + lunarSize
	startY := sectionTop + (sectionH-blockH)/2 // 文本块上沿

	b1 := baselineFromTop(startY, timeSize)
	r.text(left, b1, data.TimeStr, timeSize, color.Black, true, 0)

	b2 := baselineFromTop(startY+timeSize+15, dateSize)
	r.text(left, b2, data.DateStr, dateSize, color.Black, true, 0)

	b3 := baselineFromTop(startY+timeSize+15+dateSize+6, lunarSize)
	r.text(left, b3, "农历 "+data.LunarStr, lunarSize, color.RGBA{0x33, 0x33, 0x33, 0xff}, false, 0)

	// ---- 右侧天气卡片（圆角 16，4px 黑边，内容右对齐）----
	cardW := 330.0
	cardX := right - cardW
	cardH := 170.0 // temp(68)+8+desc(26)+6+range(22)=130，+ 上下 padding 40
	cardY := sectionTop + (sectionH-cardH)/2

	r.dc.SetLineWidth(4)
	r.dc.SetColor(color.Black)
	r.dc.DrawRoundedRectangle(cardX, cardY, cardW, cardH, 16)
	r.dc.Stroke() // 白底已由画布背景保证

	innerRight := cardX + cardW - 30 // 右边距 30
	contH := 68.0 + 8 + 26 + 6 + 22  // 卡片内容总高
	cy := cardY + (cardH-contH)/2    // 内容块上沿（纵向居中）

	tb := baselineFromTop(cy, 68)
	r.text(innerRight, tb, fmt.Sprintf("%.0f°C", data.Temp), 68, color.Black, true, 1)

	db := baselineFromTop(cy+68+8, 26)
	r.text(innerRight, db, data.WeatherText, 26, color.Black, true, 1)

	rb := baselineFromTop(cy+68+8+26+6, 22)
	r.text(innerRight, rb, fmt.Sprintf("%.0f°C ~ %.0f°C", data.LowTemp, data.HighTemp),
		22, color.RGBA{0x44, 0x44, 0x44, 0xff}, false, 1)

	// ---- 分割线（4px 黑线）----
	r.dc.SetLineWidth(4)
	r.dc.SetColor(color.Black)
	r.dc.DrawLine(left, sectionBottom, right, sectionBottom)
	r.dc.Stroke()
}

// ----------------------------------------------------------------------------
// 2. 最近 5 日天气预报：横向 5 列卡片
// ----------------------------------------------------------------------------

func drawForecast(r *renderer, data DashboardData, baseDate time.Time) {
	sectionTop := padY + 250 + 30 // 顶部区底(300) + margin-top 30 = 330
	gap := 15.0
	contentW := canvasW - 2*padX
	cols := 5
	cardW := (contentW - float64(cols-1)*gap) / float64(cols) // ≈184.4
	cardH := 156.0                                            // 内容120 + 上下 padding 36

	for i, f := range data.Forecast {
		if i >= cols {
			break
		}
		cx := padX + float64(i)*(cardW+gap)
		cy := sectionTop
		isToday := f.IsToday

		// 卡片底：IsToday 黑底白字，其余白底黑框。
		r.dc.SetLineWidth(3)
		r.dc.SetColor(color.Black)
		r.dc.DrawRoundedRectangle(cx, cy, cardW, cardH, 14)
		if isToday {
			r.dc.Fill()
		} else {
			r.dc.Stroke()
		}

		centerX := cx + cardW/2
		textColor := color.Color(color.Black)
		if isToday {
			textColor = color.White
		}

		// 内容纵向堆叠并居中：day(24)+2+date(18)+10+weather(34)+10+temp(22)
		d := baseDate.AddDate(0, 0, i)
		dateStr := fmt.Sprintf("%02d/%02d", int(d.Month()), d.Day())

		contH := 24.0 + 2 + 18 + 10 + 34 + 10 + 22 // 120
		y := cy + (cardH-contH)/2

		r.text(centerX, baselineFromTop(y, 24), f.Weekday, 24, textColor, true, 0.5)
		y += 24 + 2
		r.text(centerX, baselineFromTop(y, 18), dateStr, 18, textColor, false, 0.5)
		y += 18 + 10
		// 用 WeatherText（中文天气现象）替代 HTML 中的 emoji 图标，保证 msyh.ttc 可渲染。
		r.text(centerX, baselineFromTop(y, 34), f.WeatherText, 34, textColor, false, 0.5)
		y += 34 + 10
		r.text(centerX, baselineFromTop(y, 22),
			fmt.Sprintf("%d°/%d°", f.HighTemp, f.LowTemp), 22, textColor, true, 0.5)
	}
}

// ----------------------------------------------------------------------------
// 3. 完整整月日历：月标题 + 7 列表头 + 6 行网格
// ----------------------------------------------------------------------------

func drawCalendar(r *renderer, data DashboardData, baseDate time.Time) {
	left := padX
	right := canvasW - padX
	contentW := canvasW - 2*padX

	sectionTop := padY + 250 + 30 + 156 + 35 // 预报区底(486) + margin-top 35 = 521

	// ---- 月标题（40px 居中，粗体）----
	year, month, _ := baseDate.Date()
	header := fmt.Sprintf("%d 年 %d 月", year, int(month))
	headerBaseline := baselineFromTop(sectionTop, 40)
	r.text((left+right)/2, headerBaseline, header, 40, color.Black, true, 0.5)

	// ---- 表头行（26px 粗体，底部 4px 黑线）----
	thTop := sectionTop + 40 + 20 // 标题(40) + margin-bottom(20)
	thBottom := thTop + 26 + 12 + 4
	headers := []string{"一", "二", "三", "四", "五", "六", "日"} // 周一为首列
	colW := contentW / 7.0
	for c := 0; c < 7; c++ {
		cx := left + float64(c)*colW + colW/2
		r.text(cx, baselineFromTop(thTop, 26), headers[c], 26, color.Black, true, 0.5)
	}
	r.dc.SetLineWidth(4)
	r.dc.SetColor(color.Black)
	r.dc.DrawLine(left, thBottom, right, thBottom)
	r.dc.Stroke()

	// ---- 6×7 网格 ----
	gridTop := thBottom
	gridBottom := canvasH - padY // 一直延伸到画布底部留白处
	rowH := (gridBottom - gridTop) / 6.0

	for row := 0; row < 6; row++ {
		for c := 0; c < 7; c++ {
			// 显示列 c（周一为首）映射到数据列 (c+1)%7（数据本身为周日为首）。
			cell := data.Calendar[row][(c+1)%7]
			x := left + float64(c)*colW
			y := gridTop + float64(row)*rowH

			// 单元格底：IsToday 黑底；其余边框（本月实线 #d0d0d0，非本月浅灰虚线）。
			if cell.IsToday {
				r.dc.SetColor(color.Black)
				r.dc.DrawRectangle(x, y, colW, rowH)
				r.dc.Fill()
			} else {
				r.dc.SetLineWidth(1)
				if cell.IsCurrentMonth {
					r.dc.SetColor(color.RGBA{0xd0, 0xd0, 0xd0, 0xff})
					r.dc.SetDash() // 实线
				} else {
					r.dc.SetColor(color.RGBA{0xc0, 0xc0, 0xc0, 0xff})
					r.dc.SetDash(4, 4) // 虚线
				}
				r.dc.DrawRectangle(x, y, colW, rowH)
				r.dc.Stroke()
				r.dc.SetDash() // 复位为实线
			}

			// 文字配色：今日白字（农历淡白），本月黑字（农历 #555），非本月淡灰。
			var numColor, lunarColor color.Color
			switch {
			case cell.IsToday:
				numColor = color.White
				lunarColor = color.RGBA{0xdd, 0xdd, 0xdd, 0xff}
			case !cell.IsCurrentMonth:
				numColor = color.RGBA{0x99, 0x99, 0x99, 0xff}
				lunarColor = color.RGBA{0xaa, 0xaa, 0xaa, 0xff}
			default:
				numColor = color.Black
				lunarColor = color.RGBA{0x55, 0x55, 0x55, 0xff}
			}

			cx := x + colW/2
			numH, lunarH, g := 28.0, 15.0, 3.0
			total := numH + g + lunarH
			cy0 := y + (rowH-total)/2 // 两行整体居中
			r.text(cx, baselineFromTop(cy0, numH), strconv.Itoa(cell.Day), numH, numColor, true, 0.5)
			r.text(cx, baselineFromTop(cy0+numH+g, lunarH), cell.Lunar, lunarH, lunarColor, false, 0.5)
		}
	}
}
