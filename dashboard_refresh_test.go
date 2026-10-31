package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRefreshBoundary(t *testing.T) {
	for _, tc := range []struct{ now, interval, want string }{
		{"1000", "60", "20"},
		{"1020", "60", "60"},
		{"1000", "120", "80"},
	} {
		out, err := exec.Command("sh", "kindle-client/TRMNL.sh", "--next-refresh-delay", tc.now, tc.interval).Output()
		if err != nil || strings.TrimSpace(string(out)) != tc.want {
			t.Errorf("%+v: got %q, %v", tc, out, err)
		}
	}
	if err := exec.Command("sh", "kindle-client/TRMNL.sh", "--next-refresh-delay", "1000", "0").Run(); err == nil {
		t.Fatal("zero interval must fail")
	}
}
