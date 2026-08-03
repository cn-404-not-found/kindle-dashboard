// main.go —— 墨水屏看板后端服务（核心骨架）
//
// 平台：Linux x86_64 (Alpine LXC)，面向 Kindle 等墨水屏设备。
// 职责：HTTP 路由监听、数据结构体定义、系统时间获取与数据 Mock。
// 注意：本文件【不包含】任何 2D 画布绘制代码，绘制由同 package 下的
//       render.go 中的 RenderDashboard(data DashboardData) ([]byte, error) 实现。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// ----------------------------------------------------------------------------
// 数据结构体定义
// ----------------------------------------------------------------------------

// ForecastItem 表示 5 日预报中的单日条目。
type ForecastItem struct {
	Weekday     string  `json:"weekday"`      // 星期，如 "周一"
	WeatherText string  `json:"weather_text"` // 天气现象/图标文本，如 "多云"
	HighTemp    int     `json:"high_temp"`    // 当日最高温（℃）
	LowTemp     int     `json:"low_temp"`     // 当日最低温（℃）
	IsToday     bool    `json:"is_today"`     // 是否为今天
}

// CalendarCell 表示月历网格中的单个单元格（7 列 × 6 行中的一个）。
type CalendarCell struct {
	Day            int    `json:"day"`             // 阳历日期，如 3
	Lunar          string `json:"lunar"`           // 农历日期，如 "廿一"（待接入真实农历）
	IsCurrentMonth bool   `json:"is_current_month"` // 是否属于当前展示的月份
	IsToday        bool   `json:"is_today"`        // 是否为今天
}

// DashboardData 是所有看板数据的聚合根，作为 RenderDashboard 的输入。
type DashboardData struct {
	// 时间与日期
	TimeStr  string `json:"time_str"`  // 当前时间，如 "10:43"
	DateStr  string `json:"date_str"`  // 当前日期，如 "2026年08月03日"
	LunarStr string `json:"lunar_str"` // 农历/节气，如 "六月廿一"
	Location string `json:"location"`  // 城市名，如 "南宁"（图上展示用）

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

// ---- 位置配置（Kindle 一般固定放置，按需修改）----
const (
	cityName = "南宁"   // 图上展示的城市名
	cityLat  = 22.82    // 南宁纬度（Open-Meteo 用）
	cityLon  = 108.37   // 南宁经度
)

// wmoToText 将 WMO weather_code 映射为中文天气现象。
// 参考：https://open-meteo.com/en/docs 的 weather_code 说明。
func wmoToText(code int) string {
	switch code {
	case 0:
		return "晴"
	case 1:
		return "晴间多云"
	case 2:
		return "多云"
	case 3:
		return "阴"
	case 45, 48:
		return "雾"
	case 51, 53, 55:
		return "毛毛雨"
	case 56, 57:
		return "冻雨"
	case 61:
		return "小雨"
	case 63:
		return "中雨"
	case 65:
		return "大雨"
	case 66, 67:
		return "冻雨"
	case 71, 73, 75:
		return "雪"
	case 77:
		return "雪粒"
	case 80, 81, 82:
		return "阵雨"
	case 85, 86:
		return "阵雪"
	case 95:
		return "雷阵雨"
	case 96, 99:
		return "雷阵雨伴冰雹"
	default:
		return "未知"
	}
}

// weatherResult 聚合一次天气拉取的结果。
type weatherResult struct {
	temp        float64
	weatherText string
	highTemp    float64
	lowTemp     float64
	forecast    []ForecastItem
}

// fetchWeather 从 Open-Meteo（免 key、免费）拉取实时天气与 7 日预报。
// 成功返回 (结果, true)；网络/解析失败时返回 (空, false)，调用方回退到 Mock。
func fetchWeather() (weatherResult, bool) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.2f&longitude=%.2f"+
			"&current=temperature_2m,weather_code"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min"+
			"&timezone=Asia%%2FShanghai&forecast_days=7",
		cityLat, cityLon,
	)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("天气获取失败（网络）: %v", err)
		return weatherResult{}, false
	}
	defer resp.Body.Close()

	var w struct {
		Current struct {
			Temperature2m float64 `json:"temperature_2m"`
			WeatherCode   int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Time             []string  `json:"time"`
			WeatherCode      []int     `json:"weather_code"`
			Temperature2mMax []float64 `json:"temperature_2m_max"`
			Temperature2mMin []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		log.Printf("天气解析失败: %v", err)
		return weatherResult{}, false
	}

	res := weatherResult{
		temp:        w.Current.Temperature2m,
		weatherText: wmoToText(w.Current.WeatherCode),
	}
	if len(w.Daily.Temperature2mMax) > 0 {
		res.highTemp = w.Daily.Temperature2mMax[0]
	}
	if len(w.Daily.Temperature2mMin) > 0 {
		res.lowTemp = w.Daily.Temperature2mMin[0]
	}
	for i := 0; i < len(w.Daily.Time) && i < 5; i++ {
		res.forecast = append(res.forecast, ForecastItem{
			WeatherText: wmoToText(w.Daily.WeatherCode[i]),
			HighTemp:    int(w.Daily.Temperature2mMax[i] + 0.5),
			LowTemp:     int(w.Daily.Temperature2mMin[i] + 0.5),
			IsToday:     i == 0,
		})
	}
	return res, true
}

// mockWeather 天气拉取失败时的兜底数据，保证看板始终可渲染。
func mockWeather() weatherResult {
	return weatherResult{
		temp:        26.5,
		weatherText: "多云",
		highTemp:    31,
		lowTemp:     23,
		forecast: []ForecastItem{
			{WeatherText: "多云", HighTemp: 31, LowTemp: 23, IsToday: true},
			{WeatherText: "晴", HighTemp: 33, LowTemp: 24},
			{WeatherText: "小雨", HighTemp: 29, LowTemp: 22},
			{WeatherText: "阴", HighTemp: 30, LowTemp: 21},
			{WeatherText: "雷阵雨", HighTemp: 28, LowTemp: 20},
		},
	}
}

// fetchDashboardData 实时获取系统时间、真实天气与农历，并组装看板数据。
func fetchDashboardData() DashboardData {
	now := time.Now()

	// 时间/日期字符串通过 time.Now() 实时获取。
	timeStr := now.Format("15:04")
	dateStr := now.Format("2006年1月2日") // 不补零，便于顶部拆成「年/月日/农历」三行

	// 天气：优先真实拉取，失败回退 Mock。
	w, ok := fetchWeather()
	if !ok {
		w = mockWeather()
	}

	return DashboardData{
		// --- 时间相关 ---
		TimeStr:  timeStr,
		DateStr:  dateStr,
		LunarStr: lunarMonthDay(now.Year(), int(now.Month()), now.Day()), // 真实农历（月+日）
		Location: cityName,

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

// lunarDay 返回指定公历日期对应的农历「日」（如 "廿一"）。
// 用于月历网格中每个单元格下方的农历小字。
func lunarDay(year, month, day int) string {
	return calendar.NewSolarFromYmd(year, month, day).GetLunar().GetDayInChinese()
}

// lunarMonthDay 返回指定公历日期对应的农历「月+日」（如 "六月廿一"）。
// 用于顶部日期行旁的农历/节气提示。
func lunarMonthDay(year, month, day int) string {
	l := calendar.NewSolarFromYmd(year, month, day).GetLunar()
	return l.GetMonthInChinese() + "月" + l.GetDayInChinese()
}

// buildMonthGrid 构建当前月份的 7×6 月历网格。
// 网格从“包含本月 1 号的那个周日”开始，覆盖完整的 6 周（42 天），
// 跨月的单元格通过 IsCurrentMonth=false 标记，今天通过 IsToday=true 标记。
// 农历 Lunar 字段暂时留空，待接入真实农历计算。
func buildMonthGrid(now time.Time) [6][7]CalendarCell {
	year, month, _ := now.Date()
	loc := now.Location()

	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	startOffset := int(firstOfMonth.Weekday()) // 1 号是周几 → 前面需补几天
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

	pngBytes, err := RenderDashboard(data)
	if err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	// 墨水屏设备通常会缓存图片，禁用缓存以保证每次拉取最新画面。
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(pngBytes)
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
	http.HandleFunc("/screen.png", screenHandler)
	http.HandleFunc("/api/display", apiDisplayHandler)

	addr := ":8080"
	log.Printf("Kindle dashboard backend listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
