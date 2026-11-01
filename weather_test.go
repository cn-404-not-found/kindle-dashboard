package main

import (
	"fmt"
	"io"
	"net/http"
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

func TestParseChinaWeatherAfterMidnight(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 1, 1, 30, 0, 0, loc)
	var b strings.Builder
	b.WriteString(`<input id="hidden_title" value="10月31日20时 多云 23/30°C"/><div id="7d" class="c7d"><ul>`)
	for i := -1; i < 6; i++ {
		d := now.AddDate(0, 0, i)
		fmt.Fprintf(&b, `<li class="sky skyid"><h1>%d日（预报）</h1><p class="wea">多云</p><p class="tem"><span>29℃</span>/<i>22℃</i></p></li>`, d.Day())
	}
	b.WriteString(`</ul></div><script>var observe24h_data = {"od":{"od0":"202611010100","od2":[{"od22":"25.4"}]}};</script>`)
	result, err := parseChinaWeather(b.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.forecast) != 5 || !result.forecast[0].IsToday || result.forecast[0].HighTemp != 29 || result.temp != 25.4 {
		t.Fatalf("unexpected result: %+v", result)
	}
	bad := strings.Replace(b.String(), `<h1>2日`, `<h1>3日`, 1)
	if _, err := parseChinaWeather(bad, now); err == nil {
		t.Fatal("expected nonconsecutive dates to fail")
	}
}

type weatherRoundTripFunc func(*http.Request) (*http.Response, error)

func (f weatherRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeatherLastSuccessfulFetchSurvivesCacheAndFailure(t *testing.T) {
	oldCfg, oldClient := cfg, weatherHTTPClient
	defer func() {
		cfg = oldCfg
		weatherHTTPClient = oldClient
		weatherCache.Lock()
		weatherCache.valid = false
		weatherCache.city = ""
		weatherCache.attempted = time.Time{}
		weatherCache.Unlock()
	}()
	cfg.CityName = "南宁"
	cfg.CityID = "101300101"
	weatherCache.Lock()
	weatherCache.valid = false
	weatherCache.city = ""
	weatherCache.attempted = time.Time{}
	weatherCache.Unlock()
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.FixedZone("CST", 8*3600))
	var page strings.Builder
	page.WriteString(`<div id="7d"><ul>`)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&page, `<li class="sky"><h1>%d日</h1><p class="wea">多云</p><p class="tem"><span>30℃</span><i>22℃</i></p></li>`, now.AddDate(0, 0, i).Day())
	}
	page.WriteString(`</ul></div><script>var observe24h_data = {"od":{"od0":"202611010900","od2":[{"od22":"25"}]}};</script>`)
	calls := 0
	fail := false
	weatherHTTPClient = &http.Client{Transport: weatherRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if fail {
			return nil, fmt.Errorf("offline")
		}
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(page.String())), Header: make(http.Header)}, nil
	})}
	first, status, ok := cachedWeather(now)
	if !ok || status != "" || !first.fetchedAt.Equal(now) {
		t.Fatalf("first: %+v %q %v", first, status, ok)
	}
	cached, _, ok := cachedWeather(now.Add(5 * time.Minute))
	if !ok || !cached.fetchedAt.Equal(now) || calls != 1 {
		t.Fatalf("cache: %+v calls %d", cached, calls)
	}
	fail = true
	stale, status, ok := cachedWeather(now.Add(31 * time.Minute))
	if !ok || status == "" || !stale.fetchedAt.Equal(now) || calls != 2 {
		t.Fatalf("stale: %+v %q %v calls %d", stale, status, ok, calls)
	}
}
