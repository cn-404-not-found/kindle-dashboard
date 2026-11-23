//go:build !windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLandscapePartialClientSchedule(t *testing.T) {
	for _, tc := range []struct{ size, clock, weather string }{
		{"1072:1448", "852:55", "182:55"},
		{"758:1024", "602:38", "128:38"},
	} {
		t.Run(tc.size, func(t *testing.T) { testLandscapePartialClientSchedule(t, tc.size, tc.clock, tc.weather) })
	}
}

func testLandscapePartialClientSchedule(t *testing.T, size, clock, weather string) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "server-settings.sh"), "#!/bin/sh\necho http://example.test\n")
	write(filepath.Join(root, "landscape-ui.sh"), "landscape_ui_start() { return 0; }\nlandscape_ui_cleanup() { exit 0; }\nlandscape_sleep() { sleep 0.05; }\n")
	write(filepath.Join(root, "network-health.sh"), "network_keep_awake() { return 0; }\nnetwork_ensure_online() { return 0; }\n")
	write(filepath.Join(bin, "eips"), `#!/bin/sh
if [ "$1" = -i ]; then printf 'xres: %s\nyres: %s\n' "${TEST_SIZE%:*}" "${TEST_SIZE#*:}"; else echo "$*" >> "$TEST_LOG"; fi
`)
	write(filepath.Join(bin, "curl"), `#!/bin/sh
while [ "$#" -gt 0 ]; do case "$1" in -D) headers=$2; shift 2;; -o) image=$2; shift 2;; -H|--max-time|--connect-timeout) shift 2;; *) url=$1; shift;; esac; done
n=0; [ -f "$TEST_COUNT" ] && n=$(cat "$TEST_COUNT"); n=$((n+1)); echo "$n" > "$TEST_COUNT"
case "$n" in 1) expected='?page=home' ;; 2|3|5) expected='?page=home&part=clock' ;; 4) expected='?page=home&part=weather' ;; 6) expected='?page=home' ;; *) exit 1 ;; esac
case "$url" in *"screen.png$expected") ;; *) exit 1 ;; esac
day=2026-11-13; hour=2026111310
case "$n" in 3|4) hour=2026111311 ;; 5|6) day=2026-11-14; hour=2026111400 ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: home\r\nX-Dashboard-Pages: 2\r\nX-Dashboard-Page-Index: 1\r\nX-Dashboard-Prev-Page: opencode\r\nX-Dashboard-Next-Page: opencode\r\nX-Dashboard-Date: %s\r\nX-Dashboard-Hour: %s\r\n' "$day" "$hour" > "$headers"
case "$expected" in '?part=clock') printf 'X-Dashboard-Part: clock\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_CLOCK%:*}" "${TEST_CLOCK#*:}" >> "$headers" ;; '?part=weather') printf 'X-Dashboard-Part: weather\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_WEATHER%:*}" "${TEST_WEATHER#*:}" >> "$headers" ;; esac
printf 'image' > "$image"
`)
	path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "draws")
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+filepath.Join(root, "state"), "TEST_LOG="+logPath, "TEST_COUNT="+filepath.Join(root, "count"), "TEST_SIZE="+size, "TEST_CLOCK="+clock, "TEST_WEATHER="+weather)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	deadline := time.Now().Add(8 * time.Second)
	var lines []string
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(logPath)
		lines = strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 7 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()
	if len(lines) != 7 {
		t.Fatalf("expected 7 paint actions, got %v; stderr:\n%s", lines, stderr.String())
	}
	clockArgs := "-x " + strings.Replace(clock, ":", " -y ", 1)
	weatherArgs := "-x " + strings.Replace(weather, ":", " -y ", 1)
	if lines[0] != "-c" || lines[1] == "-c" || !strings.Contains(lines[2], clockArgs) || !strings.Contains(lines[3], clockArgs) || !strings.Contains(lines[4], weatherArgs) || lines[5] != "-c" || lines[6] == "-c" {
		t.Fatalf("unexpected paint schedule: %v", lines)
	}
}
