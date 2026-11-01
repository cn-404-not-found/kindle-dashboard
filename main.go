// main.go —— 墨水屏看板后端服务（核心骨架）
//
// 平台：Linux x86_64 (Alpine LXC)，面向 Kindle 等墨水屏设备。
// 职责：HTTP 路由监听、数据结构体定义、系统时间获取与数据 Mock。
// 注意：本文件【不包含】任何 2D 画布绘制代码，绘制由同 package 下的
//
//	render.go 中的 RenderDashboard(data DashboardData) ([]byte, error) 实现。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	xdraw "golang.org/x/image/draw"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// ----------------------------------------------------------------------------
// 数据结构体定义
// ----------------------------------------------------------------------------

// ForecastItem 表示 5 日预报中的单日条目。
type ForecastItem struct {
	Weekday     string `json:"weekday"`      // 星期，如 "周一"
	WeatherText string `json:"weather_text"` // 天气现象/图标文本，如 "多云"
	HighTemp    int    `json:"high_temp"`    // 当日最高温（℃）
	LowTemp     int    `json:"low_temp"`     // 当日最低温（℃）
	IsToday     bool   `json:"is_today"`     // 是否为今天
}

// CalendarCell 表示月历网格中的单个单元格（7 列 × 6 行中的一个）。
type CalendarCell struct {
	Day            int    `json:"day"`              // 阳历日期，如 3
	Lunar          string `json:"lunar"`            // 农历日期，如 "廿一"（待接入真实农历）
	IsCurrentMonth bool   `json:"is_current_month"` // 是否属于当前展示的月份
	IsToday        bool   `json:"is_today"`         // 是否为今天
}

// DashboardData 是所有看板数据的聚合根，作为 RenderDashboard 的输入。
type DashboardData struct {
	// 时间与日期
	TimeStr          string `json:"time_str"`  // 当前时间，如 "10:43"
	DateStr          string `json:"date_str"`  // 当前日期，如 "2026年08月03日"
	LunarStr         string `json:"lunar_str"` // 农历/节气，如 "六月廿一"
	ObservedAt       time.Time
	ForecastAt       time.Time
	WeatherStatus    string `json:"weather_status"`
	WeatherAvailable bool   `json:"weather_available"`
	Location         string `json:"location"` // 城市名，如 "南宁"（图上展示用）

	// 实时天气
	Temp        float64 `json:"temp"`         // 实时温度（℃）
	WeatherText string  `json:"weather_text"` // 天气现象，如 "多云"
	HighTemp    float64 `json:"high_temp"`    // 今日最高温（℃）
	LowTemp     float64 `json:"low_temp"`     // 今日最低温（℃）

	// 5 日预报
	Forecast []ForecastItem `json:"forecast"`

	// 月历网格：7 列（周日→周六）× 6 行（42 天；末格周日由渲染层推算）
	Calendar [6][7]CalendarCell `json:"calendar"`
}

// displayResponse 对应 GET /api/display 的 JSON 响应体。
type displayResponse struct {
	ImageURL    string `json:"image_url"`
	Filename    string `json:"filename"`
	RefreshRate int    `json:"refresh_rate"`
}

// ----------------------------------------------------------------------------
// 数据获取与 Mock 逻辑
// ----------------------------------------------------------------------------

// weekdayNames 中文星期名称，索引与 time.Weekday 对齐（0=周日）。
var weekdayNames = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// ----------------------------------------------------------------------------
// 运行时配置（可选 config.json，位于可执行文件同目录）
// ----------------------------------------------------------------------------

// serverConfig 聚合可通过配置文件覆盖的运行时参数。
// 默认值与历史硬编码一致，保证不提供配置文件时行为完全不变。
// 注意：经纬度不在此处配置——它们由代码按 CityName 查 cityCoords 表得到；
// 配置文件只需写 port、city_name 与 timezone_offset 三个字段（后两者有合理默认）。
type serverConfig struct {
	CityID         string `json:"city_id"`
	Port           int    `json:"port"`            // 监听端口（默认 8080）
	CityName       string `json:"city_name"`       // 图上展示的城市名（默认 南宁）；经纬度按表查得
	TimezoneOffset int    `json:"timezone_offset"` // 相对 UTC 的小时偏移（默认 8，即中国 UTC+8）；决定看板显示时间，不依赖宿主时区
}

// cfg 为全局运行时配置，由 main 启动时 loadConfig 填充。
var cfg serverConfig

// appLoc 为根据配置时区偏移构造的固定时区，看板所有时间/日期均以此为准。
var appLoc *time.Location

// defaultConfig 返回内置默认配置。
func defaultConfig() serverConfig {
	return serverConfig{
		Port:           8080,
		CityName:       "南宁",
		TimezoneOffset: 8, // 中国 UTC+8
	}
}

// cityCoords 内置「城市名 → 经纬度」查表，避免用户在配置文件中手写经纬度。
// 未收录的城市将回退到默认值（南宁）。如需新增城市，在此追加一行即可。
var cityCoords = map[string][2]float64{
	"南宁":   {22.82, 108.37},
	"北京":   {39.90, 116.40},
	"上海":   {31.23, 121.47},
	"广州":   {23.13, 113.26},
	"深圳":   {22.54, 114.06},
	"杭州":   {30.27, 120.15},
	"成都":   {30.57, 104.07},
	"武汉":   {30.59, 114.31},
	"西安":   {34.34, 108.94},
	"重庆":   {29.56, 106.55},
	"南京":   {32.06, 118.80},
	"天津":   {39.13, 117.20},
	"苏州":   {31.30, 120.62},
	"长沙":   {28.23, 112.94},
	"沈阳":   {41.80, 123.43},
	"青岛":   {36.07, 120.38},
	"郑州":   {34.75, 113.62},
	"大连":   {38.91, 121.61},
	"厦门":   {24.48, 118.09},
	"昆明":   {25.04, 102.71},
	"哈尔滨":  {45.80, 126.53},
	"济南":   {36.65, 117.00},
	"福州":   {26.07, 119.30},
	"贵阳":   {26.65, 106.63},
	"太原":   {37.87, 112.55},
	"兰州":   {36.06, 103.83},
	"海口":   {20.04, 110.20},
	"拉萨":   {29.65, 91.13},
	"乌鲁木齐": {43.83, 87.62},
	"呼和浩特": {40.84, 111.75},
	"银川":   {38.49, 106.23},
	"西宁":   {36.62, 101.78},
	"石家庄":  {38.04, 114.51},
	"南昌":   {28.68, 115.86},
	"合肥":   {31.82, 117.23},
	"宁波":   {29.87, 121.55},
	"无锡":   {31.49, 120.31},
	"常州":   {31.81, 119.97},
	"东莞":   {23.02, 113.75},
	"佛山":   {23.02, 113.12},
	"珠海":   {22.27, 113.58},
	"香港":   {22.32, 114.17},
	"澳门":   {22.20, 113.54},
	"台北":   {25.03, 121.57},
}

// resolveCityCoords 按城市名查表返回经纬度（lat, lon）。
// 命中返回 (lat, lon, true)；未收录返回默认城市（南宁）坐标并报警告。
func resolveCityCoords(name string) (lat, lon float64, ok bool) {
	if c, found := cityCoords[name]; found {
		return c[0], c[1], true
	}
	log.Printf("未收录城市 %q，天气经纬度回退到默认（南宁）", name)
	return cityCoords["南宁"][0], cityCoords["南宁"][1], false
}

// loadConfig 读取 exe 同目录（回退工作目录）的 config.json。
// 文件缺失时返回默认配置；解析失败或端口非法时同样回退默认，确保启动不中断。
// 仅 JSON 中显式出现的字段会覆盖默认值，未出现的保留默认。
func loadConfig() serverConfig {
	c := defaultConfig()
	data, err := os.ReadFile(resolveFontPath("config.json"))
	if err == nil {
		if err := json.Unmarshal(data, &c); err != nil {
			log.Printf("配置文件解析失败，使用默认配置: %v", err)
			c = defaultConfig()
		}
	} else if !os.IsNotExist(err) {
		log.Printf("读取配置文件失败，使用默认配置: %v", err)
	}
	if c.Port <= 0 || c.Port > 65535 {
		log.Printf("配置端口 %d 非法，回退默认 8080", c.Port)
		c.Port = defaultConfig().Port
	}
	if c.TimezoneOffset < -12 || c.TimezoneOffset > 14 {
		log.Printf("配置时区偏移 %d 非法，回退默认 8（UTC+8）", c.TimezoneOffset)
		c.TimezoneOffset = defaultConfig().TimezoneOffset
	}
	return applyEnvironment(c)
}

// applyEnvironment 允许部署环境覆盖 JSON，非法值保留已验证的文件或默认值。
func applyEnvironment(c serverConfig) serverConfig {
	if value, ok := os.LookupEnv("CITY_NAME"); ok && strings.TrimSpace(value) != "" {
		c.CityName = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("CITY_ID"); ok {
		value = strings.TrimSpace(value)
		if value == "" || cityIDPattern.MatchString(value) {
			c.CityID = value
		} else {
			log.Printf("环境变量 CITY_ID 无效，保留已有配置")
		}
	}
	if value, ok := os.LookupEnv("TIMEZONE_OFFSET"); ok && strings.TrimSpace(value) != "" {
		offset, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || offset < -12 || offset > 14 {
			log.Printf("环境变量 TIMEZONE_OFFSET 无效，保留已有配置")
		} else {
			c.TimezoneOffset = offset
		}
	}
	return c
}

// fetchDashboardData 实时获取系统时间、真实天气与农历，并组装看板数据。
func fetchDashboardData() DashboardData {
	// 以配置时区（默认 UTC+8）为准取当前时刻，不依赖宿主系统时区设置。
	now := time.Now().In(appLoc)

	// 时间/日期字符串通过 time.Now() 实时获取。
	timeStr := now.Format("15:04")
	dateStr := now.Format("2006年1月2日") // 不补零，便于顶部拆成「年/月日/农历」三行

	// 天气：优先真实拉取，失败回退 Mock。
	w, status, ok := cachedWeather(now)

	return DashboardData{
		// --- 时间相关 ---
		TimeStr:          timeStr,
		DateStr:          dateStr,
		LunarStr:         lunarMonthDay(now.Year(), int(now.Month()), now.Day()), // 真实农历（月+日）
		Location:         cfg.CityName,
		WeatherStatus:    status,
		ObservedAt:       w.observedAt,
		ForecastAt:       w.forecastAt,
		WeatherAvailable: ok,

		// --- 实时天气（Open-Meteo 真实数据，失败回退 Mock）---
		Temp:        w.temp,
		WeatherText: w.weatherText,
		HighTemp:    w.highTemp,
		LowTemp:     w.lowTemp,

		// --- 5 日预报与月历网格 ---
		Forecast: buildForecast(now, w),
		Calendar: buildMonthGrid(now),
	}
}

// buildForecast 基于今天生成未来 5 日的预报：星期/日期由 now 推导（真实），
// 天气现象与最高/最低温取自真实拉取的 w.forecast（失败时为 Mock 兜底）。
func buildForecast(now time.Time, w weatherResult) []ForecastItem {
	if !w.forecastDate.IsZero() && (w.forecastDate.Year() != now.Year() || w.forecastDate.YearDay() != now.YearDay()) {
		return nil
	}
	n := len(w.forecast)
	if n > 5 {
		n = 5
	}
	items := make([]ForecastItem, n)
	for i := 0; i < n; i++ {
		d := now.AddDate(0, 0, i)
		items[i] = ForecastItem{
			Weekday:     weekdayNames[int(d.Weekday())],
			WeatherText: w.forecast[i].WeatherText,
			HighTemp:    w.forecast[i].HighTemp,
			LowTemp:     w.forecast[i].LowTemp,
			IsToday:     i == 0,
		}
	}
	return items
}

var solarFestivals = map[string]string{"01-01": "元旦", "05-01": "劳动节", "10-01": "国庆节"}

// lunarDay 返回指定公历日期对应的农历「日」（如 "廿一"）。
// 用于月历网格中每个单元格下方的农历小字。
func lunarDay(year, month, day int) string {
	solar := calendar.NewSolarFromYmd(year, month, day)
	lunar := solar.GetLunar()
	if festivals := lunar.GetFestivals(); festivals.Len() > 0 {
		return festivals.Front().Value.(string)
	}
	if festival, ok := solarFestivals[fmt.Sprintf("%02d-%02d", month, day)]; ok {
		return festival
	}
	if term := lunar.GetJieQi(); term != "" {
		return term
	}
	if lunar.GetDayInChinese() == "初一" {
		return lunar.GetMonthInChinese() + "月"
	}
	return lunar.GetDayInChinese()
}

// lunarMonthDay 返回指定公历日期对应的农历「月+日」（如 "六月廿一"）。
// 用于顶部日期行旁的农历/节气提示。
func lunarMonthDay(year, month, day int) string {
	l := calendar.NewSolarFromYmd(year, month, day).GetLunar()
	return l.GetMonthInChinese() + "月" + l.GetDayInChinese()
}

// buildMonthGrid 构建当前月份的 7×6 月历网格。
// 网格从包含本月 1 号的周一开始，覆盖完整的 6 周（42 天），
// 跨月的单元格通过 IsCurrentMonth=false 标记，今天通过 IsToday=true 标记。
// 节日、节气及农历月份显示在日期下方。
func buildMonthGrid(now time.Time) [6][7]CalendarCell {
	year, month, _ := now.Date()
	loc := now.Location()

	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	startOffset := (int(firstOfMonth.Weekday()) + 6) % 7 // 1 号是周几 → 前面需补几天
	gridStart := firstOfMonth.AddDate(0, 0, -startOffset)

	var grid [6][7]CalendarCell
	cursor := gridStart
	for row := 0; row < 6; row++ {
		for col := 0; col < 7; col++ {
			cell := CalendarCell{
				Day:            cursor.Day(),
				Lunar:          lunarDay(cursor.Year(), int(cursor.Month()), cursor.Day()), // 真实农历日期
				IsCurrentMonth: cursor.Month() == month,
				IsToday: cursor.Year() == now.Year() &&
					cursor.Month() == now.Month() &&
					cursor.Day() == now.Day(),
			}
			grid[row][col] = cell
			cursor = cursor.AddDate(0, 0, 1)
		}
	}
	return grid
}

// ----------------------------------------------------------------------------
// HTTP 接口实现
// ----------------------------------------------------------------------------

// screenHandler 处理 GET /screen.png：
// 每次请求动态获取数据 → 调用 RenderDashboard 生成 PNG → 直接返回字节流。
func screenHandler(w http.ResponseWriter, r *http.Request) {
	data := fetchDashboardData()

	localClock := r.URL.Query().Get("clock") == "local"
	pngBytes, err := renderDashboard(data, localClock)
	if err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 适配设备帧缓冲尺寸：trmnl-kindle 客户端会在请求头上报 png-width/png-height
	// （即 eips -i 得到的屏幕分辨率）。服务端把基准图缩放/居中到正好匹配设备尺寸，
	// 保持纵横比、多余区域填白，避免 eips 把图拉伸填充而变形。
	// 若客户端未上报或上报非法，则回退到基准 1072×1448 原图。
	if wStr := r.Header.Get("png-width"); wStr != "" {
		if hStr := r.Header.Get("png-height"); hStr != "" {
			if dw, e1 := strconv.Atoi(wStr); e1 == nil && dw > 0 {
				if dh, e2 := strconv.Atoi(hStr); e2 == nil && dh > 0 {
					if out, e3 := fitToDevice(pngBytes, dw, dh); e3 != nil {
						log.Printf("适配设备分辨率失败，回退原图: %v", e3)
					} else {
						pngBytes = out
					}
				}
			}
		}
	}

	// eips 在部分 Kindle 固件上会将 RGB PNG 横向展开；统一编码为 8 位灰度。
	pngBytes, err = encodeGrayscalePNG(pngBytes)
	if err != nil {
		http.Error(w, "grayscale conversion failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if localClock {
		w.Header().Set("X-Dashboard-Clock", "local")
	}
	w.Header().Set("Content-Type", "image/png")
	// 墨水屏设备通常会缓存图片，禁用缓存以保证每次拉取最新画面。
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(pngBytes)
}

// encodeGrayscalePNG 将 PNG 转换为单通道灰度，兼容 Kindle 的 eips 显示。
func encodeGrayscalePNG(src []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	gray := image.NewGray(img.Bounds())
	draw.Draw(gray, gray.Bounds(), img, img.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, gray); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fitToDevice 将基准 PNG（1072×1448）缩放/居中适配到设备帧缓冲尺寸 devW×devH，
// 保持纵横比（contain），超出区域填白。这样 eips 能以 1:1 铺满屏幕、不再拉伸变形。
//
// 特殊处理：Kindle 等墨水屏设备的 framebuffer 可能是横屏存储（如 1448×1072），
// 但用户竖屏握持。当检测到 devW > devH（横屏帧缓冲）且源图为竖屏（sw < sh）时，
// 自动将源图顺时针旋转 90° 后再适配，确保竖屏内容在横屏 framebuffer 上正确显示。
func fitToDevice(src []byte, devW, devH int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	sb := img.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, fmt.Errorf("源图尺寸非法 %dx%d", sw, sh)
	}

	// 自动旋转：横屏 framebuffer + 竖屏源图 → 顺时针转 90°
	if devW > devH && sw < sh {
		img = rotate90CW(img, sw, sh)
		sw, sh = sh, sw // 旋转后宽高互换
	}

	dst := image.NewRGBA(image.Rect(0, 0, devW, devH))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)

	scale := math.Min(float64(devW)/float64(sw), float64(devH)/float64(sh))
	nw, nh := int(math.Round(float64(sw)*scale)), int(math.Round(float64(sh)*scale))
	if nw <= 0 || nh <= 0 {
		return nil, fmt.Errorf("缩放后尺寸过小 %dx%d", nw, nh)
	}
	scaled := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.BiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)

	ox, oy := (devW-nw)/2, (devH-nh)/2
	draw.Draw(dst, image.Rect(ox, oy, ox+nw, oy+nh), scaled, image.Point{}, draw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// rotate90CW 将图像顺时针旋转 90 度。
// 输出尺寸为 (h, w)，像素映射：src(x,y) → dst(h-1-y, x)
func rotate90CW(img image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, img.At(x, y))
		}
	}
	return dst
}

// apiDisplayHandler 处理 GET /api/display：返回前端轮询所需的元信息。
func apiDisplayHandler(w http.ResponseWriter, r *http.Request) {
	resp := displayResponse{
		ImageURL:    "/screen.png",
		Filename:    "dashboard.png",
		RefreshRate: 60, // 建议刷新间隔（秒）
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "encode failed: "+err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	cfg = loadConfig()
	// 根据配置时区偏移构造固定时区（中国默认 UTC+8）；看板时间以配置为准，不受宿主时区影响。
	appLoc = time.FixedZone(fmt.Sprintf("UTC%+d", cfg.TimezoneOffset), cfg.TimezoneOffset*3600)
	http.HandleFunc("/screen.png", screenHandler)
	http.HandleFunc("/api/display", apiDisplayHandler)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Kindle dashboard backend listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
