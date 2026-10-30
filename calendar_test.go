package main

import (
	"testing"
	"time"
)

func TestMonthGridSundayFirst(t *testing.T) {
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	grid := buildMonthGrid(now)
	if grid[0][0].Day != 26 || grid[0][0].IsCurrentMonth || grid[0][6].Day != 1 || !grid[0][6].IsToday {
		t.Fatalf("incorrect first week: %+v", grid[0])
	}
	count := 0
	for _, row := range grid {
		for _, cell := range row {
			if cell.IsCurrentMonth {
				count++
			}
		}
	}
	if count != 30 {
		t.Fatalf("expected 30 days, got %d", count)
	}
}
func TestCalendarLabels(t *testing.T) {
	for _, tc := range []struct {
		year, month, day int
		want             string
	}{{2026, 2, 17, "春节"}, {2026, 11, 7, "立冬"}, {2026, 10, 10, "九月"}} {
		if got := lunarDay(tc.year, tc.month, tc.day); got != tc.want {
			t.Errorf("%d-%d-%d: got %q, want %q", tc.year, tc.month, tc.day, got, tc.want)
		}
	}
}
func TestExpiredForecastNotRelabeled(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 1, 0, 0, time.UTC)
	w := weatherResult{forecastDate: now.AddDate(0, 0, -1), forecast: []ForecastItem{{WeatherText: "晴"}}}
	if len(buildForecast(now, w)) != 0 {
		t.Fatal("expired forecast relabeled as today")
	}
}
