//go:build !windows

package main

import (
	"bytes"
	"encoding/binary"
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
		{"1072:1448", "852:55", "136:55"},
		{"758:1024", "602:38", "96:38"},
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
case "$expected" in '?page=home&part=clock') printf 'X-Dashboard-Part: clock\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_CLOCK%:*}" "${TEST_CLOCK#*:}" >> "$headers" ;; '?page=home&part=weather') printf 'X-Dashboard-Part: weather\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_WEATHER%:*}" "${TEST_WEATHER#*:}" >> "$headers" ;; esac
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

func TestLandscapePartialOpencodeQuickRefresh(t *testing.T) {
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
if [ "$1" = -i ]; then printf 'xres: 1072\nyres: 1448\n'; else echo "$*" >> "$TEST_LOG"; fi
`)
	write(filepath.Join(bin, "curl"), `#!/bin/sh
while [ "$#" -gt 0 ]; do case "$1" in -D) headers=$2; shift 2;; -o) image=$2; shift 2;; -H|--max-time|--connect-timeout) shift 2;; *) url=$1; shift;; esac; done
n=0; [ -f "$TEST_COUNT" ] && n=$(cat "$TEST_COUNT"); n=$((n+1)); echo "$n" > "$TEST_COUNT"
case "$n" in 1) expected='?page=opencode' ;; 2|3) expected='?page=opencode&part=pageheader' ;; 4) expected='?page=opencode' ;; *) exit 1 ;; esac
revision=aaaaaaaaaaaaaaaa
case "$n" in 3|4) revision=bbbbbbbbbbbbbbbb ;; esac
case "$url" in *"screen.png$expected") ;; *) exit 1 ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: opencode\r\nX-Dashboard-Pages: 2\r\nX-Dashboard-Page-Index: 2\r\nX-Dashboard-Prev-Page: home\r\nX-Dashboard-Next-Page: home\r\nX-Dashboard-Date: 2026-11-13\r\nX-Dashboard-Hour: 2026111310\r\nX-Dashboard-Usage-Revision: %s\r\n' "$revision" > "$headers"
case "$expected" in *'part=pageheader') printf 'X-Dashboard-Part: pageheader\r\nX-Dashboard-X: 948\r\nX-Dashboard-Y: 620\r\n' >> "$headers" ;; esac
printf 'image' > "$image"
`)
	path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "draws")
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
		"KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+filepath.Join(root, "state"),
		"KINDLE_DASHBOARD_PAGE=opencode", "TEST_LOG="+logPath,
		"TEST_COUNT="+filepath.Join(root, "count"))
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
		if len(lines) >= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()
	if len(lines) != 4 {
		t.Fatalf("expected 4 paint actions, got %v; stderr:\n%s", lines, stderr.String())
	}
	if lines[0] != "-c" || !strings.Contains(lines[1], "-g") ||
		!strings.Contains(lines[2], "-x 948 -y 620") ||
		!strings.Contains(lines[3], "-x 0 -y 0") {
		t.Fatalf("unexpected paint schedule: %v", lines)
	}
}

func TestLandscapePartialHomeNewsPatch(t *testing.T) {
	for _, tc := range []struct {
		size, clock, news string
	}{
		{"1072:1448", "852:55", "36:680"},
		{"758:1024", "602:38", "25:480"},
	} {
		t.Run(tc.size, func(t *testing.T) {
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
case "$n" in 1) expected='?page=home' ;; 2|3|5) expected='?page=home&part=clock' ;; 4) expected='?page=home&part=news' ;; *) exit 1 ;; esac
revision=aaaaaaaaaaaaaaaa
case "$n" in 3|4|5) revision=bbbbbbbbbbbbbbbb ;; esac
case "$url" in *"screen.png$expected") ;; *) exit 1 ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: home\r\nX-Dashboard-Pages: 3\r\nX-Dashboard-Page-Index: 1\r\nX-Dashboard-Prev-Page: calendar\r\nX-Dashboard-Next-Page: opencode\r\nX-Dashboard-Date: 2026-11-26\r\nX-Dashboard-Hour: 2026112616\r\nX-Dashboard-Usage-Revision: %s\r\n' "$revision" > "$headers"
case "$expected" in
  '?page=home&part=clock') printf 'X-Dashboard-Part: clock\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_CLOCK%:*}" "${TEST_CLOCK#*:}" >> "$headers" ;;
  '?page=home&part=news') printf 'X-Dashboard-Part: news\r\nX-Dashboard-X: %s\r\nX-Dashboard-Y: %s\r\n' "${TEST_NEWS%:*}" "${TEST_NEWS#*:}" >> "$headers" ;;
esac
printf 'image' > "$image"
`)
			path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
			if err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(root, "draws")
			cmd := exec.Command("sh", path)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
				"KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+filepath.Join(root, "state"),
				"TEST_LOG="+logPath, "TEST_COUNT="+filepath.Join(root, "count"),
				"TEST_SIZE="+tc.size, "TEST_CLOCK="+tc.clock, "TEST_NEWS="+tc.news)
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
				if len(lines) >= 6 {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_, _ = cmd.Process.Wait()
			if len(lines) != 6 {
				t.Fatalf("expected 6 paint actions, got %v; stderr:\n%s", lines, stderr.String())
			}
			clockArgs := "-x " + strings.Replace(tc.clock, ":", " -y ", 1)
			newsArgs := "-x " + strings.Replace(tc.news, ":", " -y ", 1)
			if lines[0] != "-c" || lines[1] == "-c" ||
				!strings.Contains(lines[2], clockArgs) ||
				!strings.Contains(lines[3], clockArgs) ||
				!strings.Contains(lines[4], newsArgs) ||
				!strings.Contains(lines[5], clockArgs) {
				t.Fatalf("unexpected paint schedule: %v", lines)
			}
		})
	}
}

func TestLandscapePartialCalendarClockPatch(t *testing.T) {
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
if [ "$1" = -i ]; then printf 'xres: 1072\nyres: 1448\n'; else echo "$*" >> "$TEST_LOG"; fi
`)
	write(filepath.Join(bin, "curl"), `#!/bin/sh
while [ "$#" -gt 0 ]; do case "$1" in -D) headers=$2; shift 2;; -o) image=$2; shift 2;; -H|--max-time|--connect-timeout) shift 2;; *) url=$1; shift;; esac; done
n=0; [ -f "$TEST_COUNT" ] && n=$(cat "$TEST_COUNT"); n=$((n+1)); echo "$n" > "$TEST_COUNT"
case "$n" in 1) expected='?page=calendar' ;; 2) expected='?page=calendar&part=clock' ;; *) exit 1 ;; esac
case "$url" in *"screen.png$expected") ;; *) exit 1 ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: calendar\r\nX-Dashboard-Pages: 3\r\nX-Dashboard-Page-Index: 3\r\nX-Dashboard-Prev-Page: opencode\r\nX-Dashboard-Next-Page: home\r\nX-Dashboard-Date: 2026-11-26\r\nX-Dashboard-Hour: 2026112616\r\n' > "$headers"
case "$expected" in '?page=calendar&part=clock') printf 'X-Dashboard-Part: clock\r\nX-Dashboard-X: 852\r\nX-Dashboard-Y: 55\r\n' >> "$headers" ;; esac
printf 'image' > "$image"
`)
	path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "draws")
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
		"KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+filepath.Join(root, "state"),
		"KINDLE_DASHBOARD_PAGE=calendar", "TEST_LOG="+logPath,
		"TEST_COUNT="+filepath.Join(root, "count"))
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
		if len(lines) >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()
	if len(lines) != 3 {
		t.Fatalf("expected 3 paint actions, got %v; stderr:\n%s", lines, stderr.String())
	}
	if lines[0] != "-c" || !strings.Contains(lines[1], "-g") || !strings.Contains(lines[2], "-x 852 -y 55") {
		t.Fatalf("unexpected paint schedule: %v", lines)
	}
}

func TestLandscapePartialTapOpenPaginateReturn(t *testing.T) {
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
	write(filepath.Join(root, "landscape-ui.sh"), "landscape_ui_start() { return 0; }\nlandscape_ui_cleanup() { exit 0; }\nlandscape_sleep() { sleep \"$1\" & landscape_sleep_pid=$!; wait \"$landscape_sleep_pid\"; [ -n \"$landscape_sleep_pid\" ] && kill \"$landscape_sleep_pid\" 2>/dev/null; [ -n \"$landscape_sleep_pid\" ] && wait \"$landscape_sleep_pid\" 2>/dev/null; landscape_sleep_pid=; }\n")
	write(filepath.Join(root, "network-health.sh"), "network_keep_awake() { return 0; }\nnetwork_ensure_online() { return 0; }\n")
	write(filepath.Join(bin, "eips"), `#!/bin/sh
if [ "$1" = -i ]; then printf 'xres: 758\nyres: 1024\n'; else echo "$*" >> "$TEST_LOG"; fi
`)
	write(filepath.Join(bin, "curl"), `#!/bin/sh
headers=; image=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -D) headers=$2; shift 2 ;;
    -o) image=$2; shift 2 ;;
    -H|--max-time|--connect-timeout) shift 2 ;;
    -fsS|-s|-S|-f) shift ;;
    *) url=$1; shift ;;
  esac
done
echo "$url" >> "$TEST_URLS"
case "$url" in
  *"/api/tap"*)
    case "$url" in
      *"page=home"*"x=650&y=120"*) printf 'open abc123def456 1\n' ;;
      *"page=news_detail"*"x=700&y=80"*) printf 'home\n' ;;
      *) printf 'noop\n' ;;
    esac
    exit 0
    ;;
esac
page=home; ap=1
case "$url" in *"page=news_detail"*) page=news_detail ;; esac
case "$url" in *"&p=2"*) ap=2 ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: %s\r\nX-Dashboard-Pages: 3\r\nX-Dashboard-Page-Index: 1\r\nX-Dashboard-Prev-Page: calendar\r\nX-Dashboard-Next-Page: opencode\r\nX-Dashboard-Date: 2026-11-26\r\nX-Dashboard-Hour: 2026112617\r\nX-Dashboard-Usage-Revision: aaaaaaaaaaaaaaaa\r\n' "$page" > "$headers"
if [ "$page" = news_detail ]; then
  printf 'X-Dashboard-Article-Id: abc123def456\r\nX-Dashboard-Article-Page: %s\r\nX-Dashboard-Article-Total: 2\r\n' "$ap" >> "$headers"
fi
printf 'image' > "$image"
`)

	path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	logPath := filepath.Join(root, "draws")
	urlLog := filepath.Join(root, "urls")
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
		"KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+state,
		"KINDLE_PAGE_SWIPE=0", "TEST_LOG="+logPath, "TEST_URLS="+urlLog)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	waitLines := func(targetFile string, want int) []string {
		t.Helper()
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(targetFile)
			txt := strings.TrimSpace(string(data))
			if txt != "" {
				lines := strings.Split(txt, "\n")
				if len(lines) >= want {
					return lines
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		data, _ := os.ReadFile(targetFile)
		t.Fatalf("timeout waiting %d lines in %s, got:\n%s\nstderr:\n%s", want, targetFile, string(data), stderr.String())
		return nil
	}
	sendReq := func(req string) {
		t.Helper()
		tmp := filepath.Join(state, "page-request.tmp")
		dst := filepath.Join(state, "page-request")
		if err := os.WriteFile(tmp, []byte(req+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Process.Signal(syscall.SIGUSR1); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Initial home full render (-c, -g).
	waitLines(logPath, 2)
	// 2. Tap news item -> full open page 1 (-c, -g).
	sendReq("tap 650 120")
	waitLines(logPath, 4)
	// 3. Swipe next -> quick flip page 2 (-g -x 0 -y 0, no -c).
	sendReq("next")
	lines := waitLines(logPath, 5)
	if strings.Contains(lines[4], "-c") || !strings.Contains(lines[4], "-x 0 -y 0") {
		t.Fatalf("page 2 flip should use quick refresh without -c, got %q", lines[4])
	}
	// 4. Tap top-left -> return home (-c, -g).
	sendReq("tap 700 80")
	waitLines(logPath, 7)
	// 5. Tap blank area -> /api/tap returns noop, zero extra redraws.
	urlsBefore := len(waitLines(urlLog, 4))
	sendReq("tap 100 100")
	waitLines(urlLog, urlsBefore+1)
	time.Sleep(150 * time.Millisecond)

	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()

	finalDraws := strings.Split(strings.TrimSpace(mustReadFile(t, logPath)), "\n")
	if len(finalDraws) != 7 {
		t.Fatalf("noop tap must not trigger extra redraws, got %d draws: %v", len(finalDraws), finalDraws)
	}
	urls := mustReadFile(t, urlLog)
	if !strings.Contains(urls, "/api/tap?page=home&x=650&y=120&w=758&h=1024") ||
		!strings.Contains(urls, "/screen.png?page=news_detail&id=abc123def456&p=1") ||
		!strings.Contains(urls, "/screen.png?page=news_detail&id=abc123def456&p=2") {
		t.Fatalf("missing expected requests in:\n%s", urls)
	}
}

func TestLandscapePartialArticleAutoTimeout(t *testing.T) {
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
	write(filepath.Join(root, "landscape-ui.sh"), "landscape_ui_start() { return 0; }\nlandscape_ui_cleanup() { exit 0; }\nlandscape_sleep() { sleep 0.25; }\n")
	write(filepath.Join(root, "network-health.sh"), "network_keep_awake() { return 0; }\nnetwork_ensure_online() { return 0; }\n")
	write(filepath.Join(bin, "eips"), `#!/bin/sh
if [ "$1" = -i ]; then printf 'xres: 758\nyres: 1024\n'; else echo "$*" >> "$TEST_LOG"; fi
`)
	write(filepath.Join(bin, "curl"), `#!/bin/sh
headers=; image=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -D) headers=$2; shift 2 ;;
    -o) image=$2; shift 2 ;;
    -H|--max-time|--connect-timeout) shift 2 ;;
    -fsS|-s|-S|-f) shift ;;
    *) url=$1; shift ;;
  esac
done
case "$url" in
  *"/api/tap"*) printf 'open abc123def456 1\n'; exit 0 ;;
esac
page=home
case "$url" in *"page=news_detail"*) page=news_detail ;; esac
printf 'X-Dashboard-Layout: landscape-v1\r\nX-Dashboard-Partial-Version: 1\r\nX-Dashboard-Page: %s\r\nX-Dashboard-Pages: 3\r\nX-Dashboard-Page-Index: 1\r\nX-Dashboard-Prev-Page: calendar\r\nX-Dashboard-Next-Page: opencode\r\nX-Dashboard-Date: 2026-11-26\r\nX-Dashboard-Hour: 2026112617\r\n' "$page" > "$headers"
if [ "$page" = news_detail ]; then
  printf 'X-Dashboard-Article-Id: abc123def456\r\nX-Dashboard-Article-Page: 1\r\nX-Dashboard-Article-Total: 1\r\n' >> "$headers"
fi
printf 'image' > "$image"
`)

	path, err := filepath.Abs("kindle-client/dashboard-extension/landscape-partial.sh")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	logPath := filepath.Join(root, "draws")
	cmd := exec.Command("sh", path)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"),
		"KINDLE_DASHBOARD_DIR="+root, "KINDLE_PARTIAL_STATE="+state,
		"KINDLE_PAGE_SWIPE=0", "KINDLE_ARTICLE_TIMEOUT=1", "TEST_LOG="+logPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	waitDrawCount := func(want int) {
		t.Helper()
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(logPath)
			txt := strings.TrimSpace(string(data))
			if txt != "" && len(strings.Split(txt, "\n")) >= want {
				return
			}
			time.Sleep(40 * time.Millisecond)
		}
		data, _ := os.ReadFile(logPath)
		t.Fatalf("timeout waiting %d draws, got:\n%s", want, string(data))
	}

	waitDrawCount(2) // initial home
	_ = os.WriteFile(filepath.Join(state, "page-request"), []byte("tap 650 120\n"), 0644)
	_ = cmd.Process.Signal(syscall.SIGUSR1)
	waitDrawCount(4) // article opened
	// After >= 1s idle in news_detail, client should auto-return to home (-c, -g => total >= 6).
	waitDrawCount(6)
}

func TestTouchSwipeDetectsTapAndSwipe(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "event1")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("kindle-client/dashboard-extension/touch-swipe.sh")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	cmd := exec.Command("sh", script, fifo, state, "999999")
	cmd.Env = append(os.Environ(), "KINDLE_TAP_DEBOUNCE=0", "KINDLE_SWIPE_DEBOUNCE=0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	sendEv := func(evType, code uint16, value int32) {
		var buf [16]byte
		binary.LittleEndian.PutUint16(buf[8:10], evType)
		binary.LittleEndian.PutUint16(buf[10:12], code)
		binary.LittleEndian.PutUint32(buf[12:16], uint32(value))
		if _, err := w.Write(buf[:]); err != nil {
			t.Fatal(err)
		}
	}
	readReq := func() string {
		t.Helper()
		reqPath := filepath.Join(state, "page-request")
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if data, err := os.ReadFile(reqPath); err == nil {
				_ = os.Remove(reqPath)
				return strings.TrimSpace(string(data))
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for %s", reqPath)
		return ""
	}

	// 1. Short tap with BTN_TOUCH=1 following X/Y (must not reset start_x/start_y).
	sendEv(3, 57, 0)
	sendEv(3, 53, 520)
	sendEv(3, 54, 740)
	sendEv(1, 330, 1)
	sendEv(3, 53, 525)
	sendEv(3, 57, -1)
	sendEv(1, 330, 0)
	if got := readReq(); got != "tap 525 740" {
		t.Fatalf("tap 1 got %q, want %q", got, "tap 525 740")
	}

	// 2. Repeated tap at exact same X where evdev only emits Y.
	sendEv(3, 57, 1)
	sendEv(3, 54, 742)
	sendEv(3, 57, -1)
	if got := readReq(); got != "tap 525 742" {
		t.Fatalf("tap 2 got %q, want %q", got, "tap 525 742")
	}

	// 3. Swipe left (dy <= -180 => next).
	sendEv(3, 57, 2)
	sendEv(3, 53, 500)
	sendEv(3, 54, 800)
	sendEv(3, 54, 550)
	sendEv(3, 57, -1)
	if got := readReq(); got != "next" {
		t.Fatalf("swipe next got %q, want next", got)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
