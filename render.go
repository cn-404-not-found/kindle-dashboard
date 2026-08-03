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
//
// 字体：更纱黑体（Sarasa Gothic，TTC 合集）。
//   默认只需 1 个常规字重文件 sarasa.ttc（取内部简体 Mono 子变体），放在可执行文件同目录即可。
//   粗体默认用「极小偏移重复填充」模拟；若同目录另有 sarasa-bold.ttc，则改用真实 Bold 字重（更锐利）。
//   （更纱官方 TTC 按字重拆文件、且每个文件内含 SC/TC/J/K/HC 全正字法 + 多风格子变体，selectSC 负责挑简体。）
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// ----------------------------------------------------------------------------
// 画布常量（与 CSS 中的 body / padding 一一对应）
// ----------------------------------------------------------------------------
const (
	canvasW = 1072.0 // Kindle Paperwhite 3 物理宽度
	canvasH = 1448.0 // Kindle Paperwhite 3 物理高度
	padX    = 45.0   // body padding-left/right
	padY    = 50.0   // body padding-top/bottom

	fontFile     = "Sarasa-Regular.ttc" // 默认字体（常规字重；更纱官方 TTC 包内文件名，无需改名）
	boldFontFile = "Sarasa-Bold.ttc"    // 可选：存在则用真实 Bold 字重，否则回退偏移模拟
	fontStyle    = "Mono"           // 首选子变体：Mono(等宽数字)/Gothic/Term/Fixed/UI；""=任意简体(SC)

	asc = 0.86 // CJK 字形近似 ascent 比例，用于将「顶对齐」换算为基线 y
)

// renderer 封装绘制上下文、TTC 内选出的简体常规/粗体字形，以及按 (字号,粗体) 缓存的字体面。
type renderer struct {
	dc    *gg.Context
	reg   *sfnt.Font // 常规字重（简体子变体）
	bold  *sfnt.Font // 粗体字重；与 reg 相同表示无独立粗体，回退偏移模拟
	faces map[faceKey]font.Face
}

// faceKey 同时区分字号与是否粗体，避免不同字重的面互相覆盖。
type faceKey struct {
	size float64
	bold bool
}

// face 返回指定字号、指定粗体状态的字体面，并按需缓存。
// bold 为 true 且 TTC 含独立粗体字重时，使用真实 Bold/Medium 字重（观感更扎实锐利）。
func (r *renderer) face(size float64, bold bool) font.Face {
	key := faceKey{size: size, bold: bold}
	if f, ok := r.faces[key]; ok {
		return f
	}
	src := r.reg
	if bold {
		src = r.bold
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		// 字体已成功解析、FaceOptions 为常量合法值，此错误实际不可达；
		// 作为编程不变量以 panic 暴露，由 RenderDashboard 的 recover 收敛为 error。
		panic(fmt.Errorf("创建 %dpx 字体面失败: %w", int(size), err))
	}
	r.faces[key] = f
	return f
}

// text 以基线位于 (x, y) 绘制字符串。
//   - hAlign：0=左对齐(x为左缘)，0.5=居中(x为中心)，1=右对齐(x为右缘)。
//   - bold：为 true 时优先用 TTC 内真实粗体字重；仅当字体无独立粗体时，
//     退而用极小偏移重复填充文字模拟粗体（墨水屏观感更扎实）。
func (r *renderer) text(x, y float64, s string, size float64, c color.Color, bold bool, hAlign float64) {
	if s == "" {
		return
	}
	r.dc.SetFontFace(r.face(size, bold))
	r.dc.SetColor(c)
	if bold && r.bold == r.reg {
		// 无独立粗体字重（如单字重 TTF）：偏移叠绘模拟粗体。
		for _, dx := range []float64{-0.8, 0.8} {
			for _, dy := range []float64{-0.8, 0.8} {
				r.dc.DrawStringAnchored(s, x+dx, y+dy, hAlign, 0)
			}
		}
	}
	r.dc.DrawStringAnchored(s, x, y, hAlign, 0)
}

// baselineFromTop 把「文字上沿 yTop」换算为「文字基线 y」（用于纵向堆叠排版）。
func baselineFromTop(yTop, size float64) float64 {
	return yTop + size*asc
}

// selectSC 从 TTC 合集中选出「简体中文(SC)」子字体，并尽量匹配首选风格(如 Mono/Gothic)。
// 更纱的 TTC 把多种正字法(SC/TC/J/K/HC)与多种风格(Gothic/Mono/Term/Fixed/UI/Slab)
// 都打包在同一个文件里(共数十个子字体)，需按全名过滤后选取：
//   - 优先取全名含 "SC"(简体) 的子字体；含首选风格关键词(如 "mono")者加分优先。
//   - 找不到任何 SC 子字体时退回合集内的第一个字形（兼容单字重普通 TTF，拖一个文件即可用）。
func selectSC(col *sfnt.Collection, stylePref string) *sfnt.Font {
	var buf sfnt.Buffer
	var best *sfnt.Font
	bestScore := -1
	var first *sfnt.Font
	for i := 0; i < col.NumFonts(); i++ {
		f, err := col.Font(i)
		if err != nil {
			continue
		}
		if first == nil {
			first = f
		}
		full, err := f.Name(&buf, sfnt.NameIDFull)
		if err != nil {
			continue
		}
		s := strings.ToLower(full)
		if !strings.Contains(s, "sc") { // 仅简体中文
			continue
		}
		score := 0
		if stylePref != "" && strings.Contains(s, strings.ToLower(stylePref)) {
			score += 2
		}
		if score > bestScore { // 严格更优才替换，保留先出现的同分者
			bestScore = score
			best = f
		}
	}
	if best == nil {
		return first // 无简体子变体时退回首个字形（兼容任意单字重 TTF/TTC）
	}
	return best
}

// ----------------------------------------------------------------------------
// 对外入口
// ----------------------------------------------------------------------------

// RenderDashboard 将看板数据绘制为 1072×1448 的 PNG 字节流。
// 常规/粗体字体 TTC 必须与可执行文件同目录；缺失或损坏时返回 error。
// 若仅提供常规字体(无粗体文件)，粗体回退为「偏移叠绘」模拟。
func RenderDashboard(data DashboardData) ([]byte, error) {
	// 1. 加载中文字体。更纱 TTC 每个文件仅含单一字重，但内含 SC/TC/J/K/HC 全部正字法，
	//    故常规、粗体分两个文件加载，再各自选出简体(SC)子变体（优先字体风格 fontStyle）。
	regFont, err := loadSCFont(fontFile)
	if err != nil {
		return nil, err
	}
	boldFont, err := loadSCFont(boldFontFile)
	if err != nil {
		// 粗体文件缺失：回退到偏移模拟（用常规字重叠绘）。
		boldFont = regFont
	}

	// 2. 初始化纯白画布。
	dc := gg.NewContext(int(canvasW), int(canvasH))
	dc.SetColor(color.White)
	dc.DrawRectangle(0, 0, canvasW, canvasH)
	dc.Fill()

	r := &renderer{
		dc:    dc,
		reg:   regFont,
		bold:  boldFont,
		faces: make(map[faceKey]font.Face),
	}

	// 3. 绘制三区并编码 PNG。字体面创建失败属编程不变量，用 recover 收敛为 error。
	baseDate := parseDate(data.DateStr)
	var (
		pngBytes  []byte
		renderErr error
	)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				renderErr = fmt.Errorf("渲染看板失败: %v", rec)
			}
		}()
		drawTopSection(r, data)
		drawForecast(r, data, baseDate)
		drawCalendar(r, data, baseDate)

		var buf bytes.Buffer
		if err := png.Encode(&buf, dc.Image()); err != nil {
			renderErr = fmt.Errorf("编码 PNG 失败: %w", err)
			return
		}
		pngBytes = buf.Bytes()
	}()
	if renderErr != nil {
		return nil, renderErr
	}
	return pngBytes, nil
}

// resolveFontPath 优先返回「可执行文件同目录」下的字体路径，否则回退到相对路径。
func resolveFontPath(name string) string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name // 回退：依赖进程工作目录中存在该文件
}

// loadSCFont 读取指定 TTC 文件，解析合集并选出简体(SC)子变体（优先 fontStyle 风格）。
// 文件缺失或不含任何 SC 子字体时返回 error。
func loadSCFont(name string) (*sfnt.Font, error) {
	fontData, err := os.ReadFile(resolveFontPath(name))
	if err != nil {
		return nil, fmt.Errorf("读取字体文件 %s 失败: %w", name, err)
	}
	col, err := sfnt.ParseCollection(fontData)
	if err != nil {
		return nil, fmt.Errorf("解析字体合集 %s 失败: %w", name, err)
	}
	f := selectSC(col, fontStyle)
	if f == nil {
		return nil, fmt.Errorf("字体 %s 未找到简体(SC)子变体", name)
	}
	return f, nil
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

	// ---- 左侧：大号时间（纵居中）----
	const timeSize = 128.0
	timeBaseline := baselineFromTop(sectionTop+(sectionH-timeSize)/2, timeSize)
	r.text(left, timeBaseline, data.TimeStr, timeSize, color.Black, true, 0)

	// ---- 时间右侧：日期 + 农历（纵居中堆叠，在时间与天气卡片之间居中）----
	const (
		dateSize  = 34.0
		lunarSize = 30.0
		ddGap     = 6.0
	)
	// 精确测量时间文字宽度，确保日期块不与时间重叠
	r.dc.SetFontFace(r.face(timeSize, true))
	timeW, _ := r.dc.MeasureString(data.TimeStr)
	dateAreaLeft := left + timeW + 50 // 时间右缘 + 充足间距
	dateAreaRight := right - 330 - 40 // 天气卡片左缘 - 间距
	dateBlockCenter := (dateAreaLeft + dateAreaRight) / 2
	// 日期拆成「年 / 月日 / 农历」三行，居中纵向堆叠并整体纵居中。
	parts := strings.SplitN(data.DateStr, "年", 2)
	yearLine := parts[0] + "年" // "2026年"
	mdLine := parts[1]          // "8月3日"
	lunarLine := "农历 " + data.LunarStr
	blockH := dateSize + ddGap + dateSize + ddGap + lunarSize
	bStart := sectionTop + (sectionH-blockH)/2
	r.text(dateBlockCenter, baselineFromTop(bStart, dateSize), yearLine, dateSize, color.Black, true, 0.5)
	r.text(dateBlockCenter, baselineFromTop(bStart+dateSize+ddGap, dateSize), mdLine, dateSize, color.Black, true, 0.5)
	r.text(dateBlockCenter, baselineFromTop(bStart+dateSize+ddGap+dateSize+ddGap, lunarSize), lunarLine, lunarSize, color.RGBA{0x33, 0x33, 0x33, 0xff}, false, 0.5)

	// ---- 右侧天气卡片（圆角 16，4px 黑边；温度/天气右对齐，南宁在卡片左侧空白区）----
	cardW := 330.0
	cardX := right - cardW
	cardH := 220.0 // temp(90)+8+desc(34)+6+range(34)=172，+ 上下 padding；位置行单独居中
	cardY := sectionTop + (sectionH-cardH)/2

	r.dc.SetLineWidth(4)
	r.dc.SetColor(color.Black)
	r.dc.DrawRoundedRectangle(cardX, cardY, cardW, cardH, 16)
	r.dc.Stroke() // 白底已由画布背景保证

	innerRight := cardX + cardW - 30 // 右边距 30
	// 温度/天气/区间三行右对齐、整体在卡片内纵向居中（位置行单独居中，不计入此处）。
	const (
		tempSize    = 90.0
		weatherSize = 34.0
		rangeSize   = 34.0
		gapTW       = 8.0 // 温度→天气
		gapWR       = 6.0 // 天气→区间
	)
	contH := tempSize + gapTW + weatherSize + gapWR + rangeSize
	cy := cardY + (cardH-contH)/2 // 三行内容块上沿（纵向居中）

	tb := baselineFromTop(cy, tempSize)
	r.text(innerRight, tb, fmt.Sprintf("%.0f°C", data.Temp), tempSize, color.Black, true, 1)

	db2 := baselineFromTop(cy+tempSize+gapTW, weatherSize)
	r.text(innerRight, db2, data.WeatherText, weatherSize, color.Black, true, 1)

	rb := baselineFromTop(cy+tempSize+gapTW+weatherSize+gapWR, rangeSize)
	r.text(innerRight, rb, fmt.Sprintf("%.0f°C ~ %.0f°C", data.LowTemp, data.HighTemp),
		rangeSize, color.RGBA{0x44, 0x44, 0x44, 0xff}, false, 1)

	// 位置：显示在温度卡片内左侧（填补左侧空白），纵向居中
	locB := baselineFromTop(cardY+(cardH-34)/2, 34)
	r.text(cardX+20, locB, data.Location, 34, color.RGBA{0x33, 0x33, 0x33, 0xff}, true, 0)

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
		// 用 WeatherText（中文天气现象）替代 HTML 中的 emoji 图标，保证字体可渲染。
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

	// ---- 6×7 网格（标准月历 6 行；末格周日越界时手动推算日期）----
	gridTop := thBottom
	gridBottom := canvasH - padY // 一直延伸到画布底部留白处
	rowH := (gridBottom - gridTop) / 6.0

	for row := 0; row < 6; row++ {
		for c := 0; c < 7; c++ {
			// 显示列 c（周一为首）映射到数据列。
			// 数据网格以周日为首（col 0 = 周日），UI 以周一为首（col 0 = 周一）。
			//   周一~周六（c=0~5）→ 取同行数据列 c+1；
			//   周日（c=6）      → 取下一行数据列 0（下一周的周日），
			//     否则周日会显示成"本周起始周日"而非"本周结束周日"，导致 8 号后面出现 2 号。
			var cell CalendarCell
			if c < 6 {
				cell = data.Calendar[row][c+1]
			} else if row+1 < 6 {
				cell = data.Calendar[row+1][0]
			} else {
				// 最后一行(行5)周日(列6)：数据越界([6][7] 只有 42 天，index 0~41)。
				// 从 gridStart 推算第 42 天（0-based index 42 = 第 43 天），填充正确日期与农历。
				year, month, _ := baseDate.Date()
				loc := baseDate.Location()
				firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
				startOffset := int(firstOfMonth.Weekday())
				gridStart := firstOfMonth.AddDate(0, 0, -startOffset)
				actualDay := gridStart.AddDate(0, 0, 42)
				cell = CalendarCell{
					Day:            actualDay.Day(),
					Lunar:          lunarDay(actualDay.Year(), int(actualDay.Month()), actualDay.Day()),
					IsCurrentMonth: actualDay.Month() == month,
					IsToday:        false,
				}
			}
			// 最后一行的周日格留空（已是月末）
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
			numH, lunarH, g := 60.0, 31.0, 7.0
			total := numH + g + lunarH
			cy0 := y + (rowH-total)/2 // 两行整体居中
			r.text(cx, baselineFromTop(cy0, numH), strconv.Itoa(cell.Day), numH, numColor, true, 0.5)
			r.text(cx, baselineFromTop(cy0+numH+g, lunarH), cell.Lunar, lunarH, lunarColor, false, 0.5)
		}
	}
}
