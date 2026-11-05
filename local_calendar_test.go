package main

import (
	"testing"
	"time"
)

func TestNextSolarTermAcrossYear(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	for _, tc := range []struct {
		date, name, wantDate string
		days                 int
	}{
		{"2026-11-05", "立冬", "2026-11-07", 2},
		{"2026-12-31", "小寒", "2027-01-05", 5},
	} {
		now, err := time.ParseInLocation("2006-01-02", tc.date, loc)
		if err != nil {
			t.Fatal(err)
		}
		name, date, days := nextSolarTerm(now)
		if name != tc.name || date.Format("2006-01-02") != tc.wantDate || days != tc.days {
			t.Errorf("%s: got %s %s %d", tc.date, name, date.Format("2006-01-02"), days)
		}
	}
}
