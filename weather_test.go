package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseChinaWeather(t *testing.T) {
	now := time.Date(2026, 10, 30, 22, 30, 0, 0, time.FixedZone("CST", 8*3600))
	var b strings.Builder
	b.WriteString(`<input id="hidden_title" value="10月30日20时 多云 23/31°C"/><div id="7d" class="c7d"><ul>`)
	for i := 0; i < 5; i++ {
		d := now.AddDate(0, 0, i)
		temps := `<span>31℃</span>/<i>23℃</i>`
		if i == 0 {
			temps = `<i>23℃</i>`
		}
		fmt.Fprintf(&b, `<li class="sky skyid"><h1>%d日（预报）</h1><p class="wea">多云</p><p class="tem">%s</p></li>`, d.Day(), temps)
	}
	b.WriteString(`</ul></div><script>var observe24h_data = {"od":{"od0":"202610302200","od2":[{"od22":"26.4"}]}};</script>`)
	result, err := parseChinaWeather(b.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	if result.temp != 26.4 || result.highTemp != 31 || result.lowTemp != 23 || len(result.forecast) != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := parseChinaWeather(b.String(), now.AddDate(0, 0, 1)); err == nil {
		t.Fatal("expected stale dates to fail")
	}
	if _, err := parseChinaWeather(strings.Replace(b.String(), `"od22":"26.4"`, `"od22":"bad"`, 1), now); err == nil {
		t.Fatal("expected invalid temperature to fail")
	}
}
