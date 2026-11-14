package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 常用城市代码：当城市列表暂时不可用时作为备份。
var weatherCityIDs = map[string]string{"南宁": "101300101", "北京": "101010100", "上海": "101020100", "广州": "101280101", "深圳": "101280601", "杭州": "101210101", "成都": "101270101", "武汉": "101200101", "西安": "101110101", "重庆": "101040100"}

type weatherResult struct {
	temp                   float64
	weatherText            string
	highTemp, lowTemp      float64
	forecast               []ForecastItem
	observedAt, forecastAt time.Time
	fetchedAt              time.Time
	forecastDate           time.Time
}

var weatherCache struct {
	sync.Mutex
	value              weatherResult
	updated, attempted time.Time
	city               string
	valid              bool
	failed             bool
}
var weatherHTTPClient = &http.Client{Timeout: 10 * time.Second}

// weatherSlot 返回当前时间最近一次半点，用于统一后台和请求触发的更新判断。
func weatherSlot(now time.Time) time.Time {
	slot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 30, 0, 0, now.Location())
	if now.Before(slot) {
		return slot.Add(-time.Hour)
	}
	return slot
}

// cachedWeather 在半点后更新缓存；失败时保留旧数据，每分钟最多重试一次。
func cachedWeather(now time.Time) (weatherResult, string, bool) {
	weatherCache.Lock()
	defer weatherCache.Unlock()
	cityKey := cfg.CityName + ":" + cfg.CityID
	if weatherCache.city != cityKey {
		weatherCache.valid = false
		weatherCache.failed = false
		weatherCache.attempted = time.Time{}
		weatherCache.city = cityKey
	}
	if !weatherCache.attempted.IsZero() && !weatherCache.attempted.Before(weatherSlot(now)) && now.Sub(weatherCache.attempted) < time.Minute {
		if weatherCache.valid {
			if weatherCache.failed {
				return weatherCache.value, "数据未更新", true
			}
			return weatherCache.value, "", true
		}
		return weatherResult{weatherText: "天气暂不可用"}, "天气获取失败", false
	}
	if weatherCache.valid && !weatherCache.failed && !weatherCache.updated.Before(weatherSlot(now)) {
		return weatherCache.value, "", true
	}
	weatherCache.attempted = now
	w, err := fetchChinaWeather(now)
	if err != nil {
		log.Printf("中国天气网获取失败: %v", err)
		weatherCache.failed = true
		if weatherCache.valid {
			return weatherCache.value, "数据未更新", true
		}
		return weatherResult{weatherText: "天气暂不可用"}, "天气获取失败", false
	}
	w.fetchedAt = now
	weatherCache.value, weatherCache.updated, weatherCache.valid, weatherCache.failed = w, now, true, false
	return w, "", true
}

// runWeatherRefresh 在启动时获取天气，之后每分钟检查半点更新和失败重试。
func runWeatherRefresh() {
	for {
		now := time.Now().In(appLoc)
		cachedWeather(now)
		// 对齐分钟边界，确保无需客户端请求也能在半点启动获取。
		now = time.Now().In(appLoc)
		time.Sleep(now.Truncate(time.Minute).Add(time.Minute).Sub(now))
	}
}

var cityIDPattern = regexp.MustCompile(`^\d{9}$`)

func fetchChinaWeather(now time.Time) (weatherResult, error) {
	id := cfg.CityID
	if id == "" {
		var err error
		id, err = weatherCities.resolve(cfg.CityName)
		if err != nil {
			return weatherResult{}, err
		}
	}
	if !cityIDPattern.MatchString(id) {
		return weatherResult{}, fmt.Errorf("城市 %q 的代码无效", cfg.CityName)
	}
	url := "https://www.weather.com.cn/weather/" + id + ".shtml"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return weatherResult{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KindleDashboard/1.0)")
	resp, err := weatherHTTPClient.Do(req)
	if err != nil {
		return weatherResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return weatherResult{}, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return weatherResult{}, err
	}
	return parseChinaWeather(string(body), now)
}

var (
	forecastSection = regexp.MustCompile(`(?s)<div id="7d"[^>]*>(.*?)</ul>`)
	forecastItem    = regexp.MustCompile(`(?s)<li\s+class="sky[^\"]*"[^>]*>(.*?)</li>`)
	weatherPattern  = regexp.MustCompile(`(?s)<p\s+[^>]*class="wea"[^>]*>(.*?)</p>`)
	tempPattern     = regexp.MustCompile(`(?s)<p\s+class="tem"[^>]*>(.*?)</p>`)
	numberPattern   = regexp.MustCompile(`-?\d+(?:\.\d+)?`)
	tagPattern      = regexp.MustCompile(`<[^>]+>`)
	titlePattern    = regexp.MustCompile(`id="hidden_title"\s+value="([^"]+)"`)
	observePattern  = regexp.MustCompile(`(?m)var\s+observe24h_data\s*=\s*(\{[^\n;]+\})\s*;`)
	dayPattern      = regexp.MustCompile(`<h1>\s*(\d+)日`)
)

func parseChinaWeather(page string, now time.Time) (weatherResult, error) {
	section := forecastSection.FindStringSubmatch(page)
	if len(section) < 2 {
		return weatherResult{}, fmt.Errorf("未找到七日预报")
	}
	blocks := forecastItem.FindAllStringSubmatch(section[1], -1)
	if len(blocks) < 5 {
		return weatherResult{}, fmt.Errorf("预报不足五天")
	}
	// 页面在午夜后可能仍从前一天开始；从今天起最多保留六天。
	start := -1
	for i, block := range blocks {
		day := dayPattern.FindStringSubmatch(block[1])
		if len(day) == 2 && day[1] == strconv.Itoa(now.Day()) {
			start = i
			break
		}
	}
	if start < 0 || len(blocks)-start < 5 {
		return weatherResult{}, fmt.Errorf("未找到从今天开始的五日预报")
	}
	var out weatherResult
	out.forecastDate = now
	end := start + 6
	if end > len(blocks) {
		end = len(blocks)
	}
	for i, block := range blocks[start:end] {
		day := dayPattern.FindStringSubmatch(block[1])
		if len(day) < 2 || day[1] != strconv.Itoa(now.AddDate(0, 0, i).Day()) {
			return weatherResult{}, fmt.Errorf("预报日期与当前日期不匹配")
		}
		desc := weatherPattern.FindStringSubmatch(block[1])
		temps := tempPattern.FindStringSubmatch(block[1])
		if len(desc) < 2 || len(temps) < 2 {
			return weatherResult{}, fmt.Errorf("预报内容不完整")
		}
		text := strings.TrimSpace(html.UnescapeString(tagPattern.ReplaceAllString(desc[1], "")))
		nums := numberPattern.FindAllString(tagPattern.ReplaceAllString(temps[1], " "), -1)
		if start+i == 0 && len(nums) < 2 {
			title := titlePattern.FindStringSubmatch(page)
			if len(title) > 1 {
				re := regexp.MustCompile(`(-?\d+)\s*/\s*(-?\d+)\s*°C`)
				m := re.FindStringSubmatch(title[1])
				if len(m) == 3 {
					nums = []string{m[2], m[1]}
				}
			}
		}
		if text == "" || len(nums) < 2 {
			return weatherResult{}, fmt.Errorf("预报温度或天气缺失")
		}
		high, e1 := strconv.ParseFloat(nums[0], 64)
		low, e2 := strconv.ParseFloat(nums[1], 64)
		if e1 != nil || e2 != nil || high < low {
			return weatherResult{}, fmt.Errorf("预报温度无效")
		}
		out.forecast = append(out.forecast, ForecastItem{WeatherText: text, HighTemp: int(math.Round(high)), LowTemp: int(math.Round(low)), IsToday: i == 0})
	}
	observation := observePattern.FindStringSubmatch(page)
	if len(observation) < 2 {
		return weatherResult{}, fmt.Errorf("未找到气温实况")
	}
	var obs struct {
		OD struct {
			Time  string `json:"od0"`
			Items []struct {
				Temp string `json:"od22"`
			} `json:"od2"`
		} `json:"od"`
	}
	if err := json.Unmarshal([]byte(observation[1]), &obs); err != nil || len(obs.OD.Items) == 0 {
		return weatherResult{}, fmt.Errorf("气温实况解析失败")
	}
	observedAt, err := time.ParseInLocation("200601021504", obs.OD.Time, now.Location())
	if err != nil || now.Sub(observedAt) > 3*time.Hour || observedAt.Sub(now) > time.Hour {
		return weatherResult{}, fmt.Errorf("气温实况时间无效或过期")
	}
	out.observedAt = observedAt
	updatePattern := regexp.MustCompile(`id="fc_24h_internal_update_time"\s+value="(\d{10,12})"`)
	if match := updatePattern.FindStringSubmatch(page); len(match) == 2 {
		stamp := match[1]
		layout := "2006010215"
		if len(stamp) == 12 {
			layout = "200601021504"
		}
		out.forecastAt, _ = time.ParseInLocation(layout, stamp, now.Location())
	}
	out.temp, err = strconv.ParseFloat(obs.OD.Items[0].Temp, 64)
	if err != nil || out.temp < -80 || out.temp > 60 {
		return weatherResult{}, fmt.Errorf("气温实况无效")
	}
	out.weatherText = out.forecast[0].WeatherText
	out.highTemp = float64(out.forecast[0].HighTemp)
	out.lowTemp = float64(out.forecast[0].LowTemp)
	return out, nil
}
