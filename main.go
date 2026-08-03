// main.go —— 墨水屏看板后端服务（核心骨架）
//
// 平台：Linux x86_64 (Alpine LXC)，面向 Kindle 等墨水屏设备。
// 职责：HTTP 路由监听、数据结构体定义、系统时间获取与数据 Mock。
// 注意：本文件【不包含】任何 2D 画布绘制代码，绘制由同 package 下的
//       render.go 中的 RenderDashboard(data DashboardData) ([]byte, error) 实现。
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
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

	// 实时天气
	Temp        float64 `json:"temp"`         // 实时温度（℃）
	WeatherText string  `json:"weather_text"` // 天气现象，如 "多云"
	HighTemp    float64 `json:"high_temp"`    // 今日最高温（℃）
	LowTemp     float64 `json:"low_temp"`     // 今日最低温（℃）

	// 5 日预报
	Forecast []ForecastItem `json:"forecast"`

	// 月历网格：7 列（周日→周六）× 6 行
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

// fetchDashboardData 实时获取系统时间并组装看板数据。
// 天气与月历的“业务值”目前为结构严密、逻辑合理的 Mock 数据，
// 已在各处标注 TODO，方便后续替换为真实 API。
func fetchDashboardData() DashboardData {
	now := time.Now()

	// 时间/日期字符串通过 time.Now() 实时获取。
	timeStr := now.Format("15:04")
	dateStr := now.Format("2006年01月02日")

	return DashboardData{
		// --- 时间相关 ---
		TimeStr:  timeStr,
		DateStr:  dateStr,
		LunarStr: "六月廿一", // TODO: 替换为真实农历/节气 API（如基于 now 计算）

		// --- 实时天气（Mock，待接入真实 API）---
		Temp:        26.5, // TODO: 替换为真实实时温度
		WeatherText: "多云", // TODO: 替换为真实天气现象
		HighTemp:    31,   // TODO: 替换为真实今日最高温
		LowTemp:     23,   // TODO: 替换为真实今日最低温

		// --- 5 日预报与月历网格 ---
		Forecast: buildForecast(now),
		Calendar: buildMonthGrid(now),
	}
}

// buildForecast 基于今天生成未来 5 日的预报（日期/星期真实，天气为 Mock）。
func buildForecast(now time.Time) []ForecastItem {
	// TODO: 替换为真实 5 日预报 API；下方 mock 数组仅为占位。
	mockTexts := []string{"多云", "晴", "小雨", "阴", "雷阵雨"}
	mockHigh := []int{31, 33, 29, 30, 28}
	mockLow := []int{23, 24, 22, 21, 20}

	items := make([]ForecastItem, 5)
	for i := 0; i < 5; i++ {
		d := now.AddDate(0, 0, i)
		items[i] = ForecastItem{
			Weekday:     weekdayNames[int(d.Weekday())],
			WeatherText: mockTexts[i],
			HighTemp:    mockHigh[i],
			LowTemp:     mockLow[i],
			IsToday:     i == 0,
		}
	}
	return items
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
				Lunar:          "", // TODO: 替换为真实农历日期
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
