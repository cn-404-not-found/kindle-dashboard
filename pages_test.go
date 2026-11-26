package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestResolveDashboardPage(t *testing.T) {
	home, ok := resolveDashboardPage("")
	if !ok || home.Name != dashboardPageHome || home.Index != 1 || home.Total != 3 || home.Next != dashboardPageOpencode || home.Prev != dashboardPageCalendar {
		t.Fatalf("home = %+v ok=%v", home, ok)
	}
	opencode, ok := resolveDashboardPage(dashboardPageOpencode)
	if !ok || opencode.Index != 2 || opencode.Total != 3 || opencode.Prev != dashboardPageHome || opencode.Next != dashboardPageCalendar {
		t.Fatalf("opencode = %+v ok=%v", opencode, ok)
	}
	calendar, ok := resolveDashboardPage(dashboardPageCalendar)
	if !ok || calendar.Index != 3 || calendar.Total != 3 || calendar.Prev != dashboardPageOpencode || calendar.Next != dashboardPageHome {
		t.Fatalf("calendar = %+v ok=%v", calendar, ok)
	}
	if _, ok := resolveDashboardPage("missing"); ok {
		t.Fatal("unknown page must be rejected")
	}
}

func TestDashboardPageSupportsPartial(t *testing.T) {
	for _, part := range []string{"clock", "weather", "news"} {
		if !dashboardPageSupportsPartial(dashboardPageHome, part) {
			t.Fatalf("home must support %q patches", part)
		}
	}
	if dashboardPageSupportsPartial(dashboardPageHome, "pageheader") {
		t.Fatal("home must reject pageheader patches")
	}
	for _, part := range []string{"clock", "weather"} {
		if !dashboardPageSupportsPartial(dashboardPageCalendar, part) {
			t.Fatalf("calendar must support %q patches", part)
		}
	}
	for _, part := range []string{"news", "pageheader"} {
		if dashboardPageSupportsPartial(dashboardPageCalendar, part) {
			t.Fatalf("calendar must reject %q patches", part)
		}
	}
	if !dashboardPageSupportsPartial(dashboardPageOpencode, "pageheader") {
		t.Fatal("opencode page must support pageheader patches")
	}
	for _, part := range []string{"clock", "weather", "news", ""} {
		if dashboardPageSupportsPartial(dashboardPageOpencode, part) {
			t.Fatalf("opencode page must reject %q patches", part)
		}
	}
}

func TestOpencodeHeaderClock(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 11, 23, 14, 5, 0, 0, loc)
	clock, date := opencodeHeaderClock(now)
	if clock != "14:05" || date != "11月23日 周一" {
		t.Fatalf("clock = %q, date = %q", clock, date)
	}
}

func TestOpencodeIntervalLabel(t *testing.T) {
	previous := opencodePollInterval
	defer func() { opencodePollInterval = previous }()
	for _, tc := range []struct {
		interval time.Duration
		want     string
	}{
		{60 * time.Second, "每分钟"},
		{5 * time.Minute, "每 5 分钟"},
		{90 * time.Second, "每 90 秒"},
		{30 * time.Second, "每分钟"},
	} {
		opencodePollInterval = tc.interval
		if got := opencodeIntervalLabel(); got != tc.want {
			t.Fatalf("interval %v: %q, want %q", tc.interval, got, tc.want)
		}
	}
}

func TestOpencodeRowTops(t *testing.T) {
	for count, want := range map[int][]float64{
		1: {340},
		2: {210, 540},
		3: {172, 436, 700},
	} {
		got := opencodeRowTops(count)
		if len(got) != len(want) {
			t.Fatalf("count %d: %v", count, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("count %d: %v, want %v", count, got, want)
			}
		}
	}
	if got := opencodeRowTops(0); got != nil {
		t.Fatalf("zero rows = %v", got)
	}
}

func TestOpencodeResetLabel(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, loc)
	tests := []struct {
		reset time.Time
		want  string
	}{
		{time.Time{}, "重置时间未知"},
		{now.Add(-time.Minute), "即将重置"},
		{now.Add(2 * time.Hour), "12:00 重置"},
		{now.AddDate(0, 0, 3), "周四 10:00 重置"},
		{now.AddDate(0, 0, 20), "12月13日 10:00 重置"},
	}
	for _, tc := range tests {
		if got := opencodeResetLabel(tc.reset, now); got != tc.want {
			t.Fatalf("reset %v: %q, want %q", tc.reset, got, tc.want)
		}
	}
}

func TestRenderOpencodeDashboard(t *testing.T) {
	if _, err := loadCalendarFont(fontFile); err != nil {
		t.Skip("Sarasa TTC unavailable")
	}
	now := time.Date(2026, 11, 23, 12, 35, 0, 0, time.FixedZone("UTC+8", 8*3600))
	usage := OpencodeUsage{
		Available: true,
		FetchedAt: now,
		Windows: []OpencodeWindow{
			{Key: "rolling", Label: "5 小时窗口", Percent: 42, ResetAt: now.Add(2 * time.Hour), Status: "ok"},
			{Key: "weekly", Label: "本 周", Percent: 78, ResetAt: now.AddDate(0, 0, 1), Status: "ok"},
			{Key: "monthly", Label: "本 月", Percent: 94, ResetAt: now.AddDate(0, 0, 8), Status: "ok"},
		},
	}
	for _, tc := range []struct {
		name  string
		usage OpencodeUsage
	}{
		{"with data", usage},
		{"without data", OpencodeUsage{Err: "未配置 OPENCODE_GO_API_KEY"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := renderOpencodeDashboard(tc.usage, now, 2, 2)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != image.Rect(0, 0, 1448, 1072) {
				t.Fatalf("bounds = %v", img.Bounds())
			}
			black := 0
			for y := 0; y < img.Bounds().Dy(); y += 7 {
				for x := 0; x < img.Bounds().Dx(); x += 7 {
					if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
						black++
					}
				}
			}
			if black == 0 {
				t.Fatal("rendered page is blank")
			}
		})
	}
}
