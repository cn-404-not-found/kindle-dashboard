package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const settingsScript = "kindle-client/dashboard-extension/server-settings.sh"

func TestServerAddressNormalization(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		valid       bool
	}{
		{"192.168.10.103:8080", "http://192.168.10.103:8080", true},
		{"https://weather.example.com:443/", "https://weather.example.com:443", true},
		{"http://10.0.0.2:65536", "", false}, {"http://10.0.0.999", "", false},
		{"http://good.example/evil", "", false}, {"http://good.example;reboot", "", false},
		{"http://-bad.example", "", false}, {"http://user@host", "", false},
	} {
		out, err := exec.Command("sh", settingsScript, "--normalize", tc.input).Output()
		if (err == nil) != tc.valid || (tc.valid && strings.TrimSpace(string(out)) != tc.want) {
			t.Errorf("%q: got %q err %v", tc.input, out, err)
		}
	}
}
func TestServerAddressSavedWithoutExecutingConfiguration(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("sh", settingsScript)
	cmd.Env = append(os.Environ(), "KINDLE_DASHBOARD_DIR="+dir, "TERM=xterm")
	cmd.Stdin = strings.NewReader("192.168.10.103:8080\ny\n\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("save: %v %s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(dir, "server-url.txt"))
	if err != nil || string(b) != "http://192.168.10.103:8080\n" {
		t.Fatalf("saved %q, %v", b, err)
	}
	cmd = exec.Command("sh", settingsScript, "--get")
	cmd.Env = append(os.Environ(), "KINDLE_DASHBOARD_DIR="+dir)
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "http://192.168.10.103:8080" {
		t.Fatalf("get: %q %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server-url.txt"), []byte("http://192.168.10.254:9000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("sh", settingsScript, "--get")
	cmd.Env = append(os.Environ(), "KINDLE_DASHBOARD_DIR="+dir)
	out, err = cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "http://192.168.10.254:9000" {
		t.Fatalf("updated get: %q %v", out, err)
	}
}
