package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLookupCityID(t *testing.T) {
	data := cityData{
		"广西": {"南宁": {"南宁": {AreaID: "101300101"}, "兴宁": {AreaID: "101300102"}}},
		"江苏": {"苏州": {"苏州": {AreaID: "101190401"}}, "泰州": {"泰州": {AreaID: "101191201"}}},
		"浙江": {"台州": {"台州": {AreaID: "101210601"}}, "苏州": {"苏州": {AreaID: "101210602"}}},
	}
	for _, tc := range []struct {
		name, id string
		wantErr  bool
	}{{"南宁", "101300101", false}, {"南宁市", "101300101", false}, {"广西/南宁", "101300101", false}, {"兴宁", "101300102", false}, {"江苏/苏州", "101190401", false}, {"苏州", "", true}, {"不存在", "", true}} {
		got, err := lookupCityID(data, tc.name)
		if (err != nil) != tc.wantErr || got != tc.id {
			t.Errorf("%q: got %q, err %v", tc.name, got, err)
		}
	}
}
func TestCityResolverFetchAndCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Referer") != "https://www.weather.com.cn/" {
			t.Error("missing referer")
		}
		fmt.Fprint(w, `var city_data = {"江苏":{"苏州":{"苏州":{"AREAID":"101190401","NAMECN":"苏州"}}}};`)
	}))
	defer server.Close()
	resolver := &cityResolver{client: server.Client(), url: server.URL}
	for i := 0; i < 2; i++ {
		id, err := resolver.resolve("苏州")
		if err != nil || id != "101190401" {
			t.Fatalf("id %s error %v", id, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected cached list, got %d calls", calls.Load())
	}
}
func TestCityResolverFailsSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "fail", http.StatusForbidden) }))
	defer server.Close()
	resolver := &cityResolver{client: server.Client(), url: server.URL}
	if id, err := resolver.resolve("南宁"); err != nil || id != "101300101" {
		t.Fatalf("fallback %s %v", id, err)
	}
	if _, err := resolver.resolve("苏州"); err == nil || !strings.Contains(err.Error(), "城市列表") {
		t.Fatalf("expected error, got %v", err)
	}
}
