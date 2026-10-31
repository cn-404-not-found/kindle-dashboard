package main

import (
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRefreshBoundary(t *testing.T) {
	for _, tc := range []struct{ now, offset, interval, want string }{
		{"1000", "0", "60", "20"},
		{"1020", "0", "60", "60"},
		{"1000", "10", "60", "10"},
		{"1000", "-10", "60", "30"},
		{"1000", "0", "120", "80"},
	} {
		out, err := exec.Command("sh", "kindle-client/TRMNL.sh", "--next-refresh-delay", tc.now, tc.offset, tc.interval).Output()
		if err != nil || strings.TrimSpace(string(out)) != tc.want {
			t.Errorf("%+v: got %q, %v", tc, out, err)
		}
	}
	if err := exec.Command("sh", "kindle-client/TRMNL.sh", "--next-refresh-delay", "1000", "0", "0").Run(); err == nil {
		t.Fatal("zero interval must fail")
	}
}

func TestDisplayProvidesCurrentServerTime(t *testing.T) {
	before := time.Now().Unix()
	w := httptest.NewRecorder()
	apiDisplayHandler(w, httptest.NewRequest("GET", "/api/display", nil))
	var response displayResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ServerTime < before || response.ServerTime > time.Now().Unix() || response.RefreshRate != 60 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("clock response must not be cached")
	}
}
