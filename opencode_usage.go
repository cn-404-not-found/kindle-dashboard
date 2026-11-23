// opencode_usage.go —— OpenCode Go 订阅用量采集与缓存
//
// OpenCode Go 没有公开文档化的配额接口，这里使用控制台/CLI 同款的
// GET /zen/go/v1/usage（需要具备 usage read 权限的 oc_sk_... 密钥）。
// 接口返回的是三个限额窗口的百分比，不是费用明细。
//
// 设计要点：
//   - 只由服务端轮询，密钥不进 Kindle、不进 URL；
//   - 轮询失败时保留上一次成功数据，并把错误写进快照供页面显示；
//   - 响应格式未文档化，解析保持宽松：带/不带 usage 前缀、ISO/epoch/秒数重置时间都接受。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	opencodeDefaultUsageURL    = "https://opencode.ai/zen/go/v1/usage"
	opencodeMaxResponseBytes   = 64 << 10
	opencodeDefaultInterval    = 60 * time.Second
	opencodeDefaultHTTPTimeout = 12 * time.Second
)

// opencodePollInterval 是当前生效的轮询间隔，仅由启动流程写入，
// 供页面底部说明文字读取。
var opencodePollInterval = opencodeDefaultInterval

// OpencodeWindow 表示一个限额窗口（5 小时滚动 / 本周 / 本月）。
type OpencodeWindow struct {
	Key     string    `json:"key"`     // rolling / weekly / monthly
	Label   string    `json:"label"`   // 页面展示名称
	Percent int       `json:"percent"` // 已用百分比，0~100
	ResetAt time.Time `json:"reset_at"`
	Status  string    `json:"status"`
}

// OpencodeUsage 是页面渲染所需的用量快照。
type OpencodeUsage struct {
	Available bool             `json:"available"`
	FetchedAt time.Time        `json:"fetched_at"`
	Windows   []OpencodeWindow `json:"windows"`
	Err       string           `json:"error,omitempty"`
}

// opencodeUsageStore 保护后台刷新与 HTTP 渲染之间的并发访问。
type opencodeUsageStore struct {
	mu    sync.RWMutex
	usage OpencodeUsage
}

var opencodeUsage opencodeUsageStore

// snapshot 返回当前快照的深拷贝，避免调用方修改共享切片。
func (s *opencodeUsageStore) snapshot() OpencodeUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.usage
	out.Windows = append([]OpencodeWindow(nil), s.usage.Windows...)
	return out
}

// replace 记录一次成功抓取。
func (s *opencodeUsageStore) replace(u OpencodeUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage = u
}

// fail 记录一次失败；已有成功数据时保留数据，只更新错误信息。
func (s *opencodeUsageStore) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage.Err = err.Error()
	if s.usage.FetchedAt.IsZero() {
		s.usage.Available = false
	}
}

// opencodeUsageConfig 读取环境变量配置。
// interval 小于 60 秒时回退默认值，避免误配置造成频繁请求。
func opencodeUsageConfig() (endpoint, apiKey string, interval time.Duration) {
	endpoint = strings.TrimSpace(os.Getenv("OPENCODE_GO_USAGE_URL"))
	if endpoint == "" {
		endpoint = opencodeDefaultUsageURL
	}
	apiKey = strings.TrimSpace(os.Getenv("OPENCODE_GO_API_KEY"))
	interval = opencodeDefaultInterval
	if raw := strings.TrimSpace(os.Getenv("OPENCODE_USAGE_INTERVAL")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 60 {
			interval = time.Duration(seconds) * time.Second
		} else {
			log.Printf("OPENCODE_USAGE_INTERVAL 无效（%q），使用默认 60 秒", raw)
		}
	}
	return endpoint, apiKey, interval
}

// startOpencodeUsageRefresh 启动后台轮询。未配置密钥时只记录状态，不发起请求。
func startOpencodeUsageRefresh(ctx context.Context) {
	endpoint, apiKey, interval := opencodeUsageConfig()
	opencodePollInterval = interval
	if apiKey == "" {
		opencodeUsage.fail(fmt.Errorf("未配置 OPENCODE_GO_API_KEY"))
		log.Printf("OpenCode Go 用量未配置：设置 OPENCODE_GO_API_KEY 后才会轮询")
		return
	}
	refresh := func() {
		usage, err := fetchOpencodeUsage(ctx, endpoint, apiKey)
		if err != nil {
			opencodeUsage.fail(err)
			log.Printf("OpenCode Go 用量刷新失败: %v", err)
			return
		}
		opencodeUsage.replace(usage)
	}
	go func() {
		// 首次抓取放到后台，避免上游网络慢时阻塞 HTTP 服务启动。
		refresh()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh()
			}
		}
	}()
}

// fetchOpencodeUsage 请求官方用量接口并解析响应。
// 响应体上限 64 KiB，且不跟随重定向，避免密钥被带到其他主机。
func fetchOpencodeUsage(ctx context.Context, endpoint, apiKey string) (OpencodeUsage, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return OpencodeUsage{}, fmt.Errorf("OPENCODE_GO_USAGE_URL 非法")
	}
	reqCtx, cancel := context.WithTimeout(ctx, opencodeDefaultHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return OpencodeUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: opencodeDefaultHTTPTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return OpencodeUsage{}, fmt.Errorf("请求用量接口失败: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return OpencodeUsage{}, fmt.Errorf("密钥无效或没有 usage 权限（HTTP %d）", resp.StatusCode)
	case http.StatusTooManyRequests:
		return OpencodeUsage{}, fmt.Errorf("用量接口限流（HTTP 429）")
	default:
		return OpencodeUsage{}, fmt.Errorf("用量接口返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, opencodeMaxResponseBytes+1))
	if err != nil {
		return OpencodeUsage{}, fmt.Errorf("读取用量响应失败: %w", err)
	}
	if len(body) > opencodeMaxResponseBytes {
		return OpencodeUsage{}, fmt.Errorf("用量响应超过 %d 字节", opencodeMaxResponseBytes)
	}
	return parseOpencodeUsage(body, time.Now())
}

// parseOpencodeUsage 解析用量响应。
// 兼容 {"usage":{...}} 与裸对象两种顶层结构；没有可用窗口时返回错误。
func parseOpencodeUsage(body []byte, now time.Time) (OpencodeUsage, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return OpencodeUsage{}, fmt.Errorf("解析用量响应失败: %w", err)
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return OpencodeUsage{}, fmt.Errorf("用量响应不是 JSON 对象")
	}
	if inner, ok := obj["usage"].(map[string]any); ok {
		obj = inner
	}
	defs := []struct {
		key   string
		label string
	}{
		{"rolling", "5 小时窗口"},
		{"weekly", "本 周"},
		{"monthly", "本 月"},
	}
	windows := make([]OpencodeWindow, 0, len(defs))
	for _, def := range defs {
		raw, ok := obj[def.key]
		if !ok {
			continue
		}
		window, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		percent, ok := opencodePercent(window)
		if !ok {
			continue
		}
		windows = append(windows, OpencodeWindow{
			Key:     def.key,
			Label:   def.label,
			Percent: percent,
			ResetAt: opencodeResetAt(window, now),
			Status:  opencodeString(window["status"]),
		})
	}
	if len(windows) == 0 {
		return OpencodeUsage{}, fmt.Errorf("用量响应中没有可识别的窗口")
	}
	return OpencodeUsage{Available: true, FetchedAt: now, Windows: windows}, nil
}

// opencodePercent 读取百分比；缺少 percent 时尝试用 used/limit 推算。
func opencodePercent(window map[string]any) (int, bool) {
	for _, key := range []string{"percent", "used_percent", "usage_percent"} {
		if value, ok := opencodeNumber(window[key]); ok {
			return int(math.Round(clampFloat(value, 0, 100))), true
		}
	}
	used, usedOK := opencodeNumber(window["used"])
	limit, limitOK := opencodeNumber(window["limit"])
	if usedOK && limitOK && limit > 0 {
		return int(math.Round(clampFloat(used/limit*100, 0, 100))), true
	}
	return 0, false
}

// opencodeResetAt 依次尝试 resetsAt/resets_at 与 resetsInSeconds/resets_in_seconds。
func opencodeResetAt(window map[string]any, now time.Time) time.Time {
	for _, key := range []string{"resetsAt", "resets_at"} {
		switch value := window[key].(type) {
		case string:
			if t, ok := parseOpencodeTime(value); ok {
				return t
			}
		default:
			if number, ok := opencodeNumber(value); ok {
				return opencodeEpoch(number)
			}
		}
	}
	for _, key := range []string{"resetsInSeconds", "resets_in_seconds"} {
		if seconds, ok := opencodeNumber(window[key]); ok && seconds >= 0 {
			return now.Add(time.Duration(seconds) * time.Second)
		}
	}
	return time.Time{}
}

// parseOpencodeTime 解析常见的 ISO-8601 写法。
func parseOpencodeTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// opencodeEpoch 把秒/毫秒时间戳转换为时间。
func opencodeEpoch(value float64) time.Time {
	if math.Abs(value) > 1e12 {
		return time.UnixMilli(int64(value))
	}
	return time.Unix(int64(value), 0)
}

// opencodeNumber 同时接受 json.Number 与 float64。
func opencodeNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		number, err := v.Float64()
		return number, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// opencodeString 只接受字符串值。
func opencodeString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// clampFloat 将数值限制在 [min, max]。
func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
