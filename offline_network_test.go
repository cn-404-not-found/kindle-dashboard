package main

import (
	"archive/tar"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOfflineNetworkHelpers(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"lipc-get-prop": "#!/bin/sh\necho 1\n",
		"lipc-set-prop": "#!/bin/sh\necho \"$3\"\n",
		"wpa_cli":       "#!/bin/sh\necho wpa_state=COMPLETED\n",
		"ifconfig":      "#!/bin/sh\necho 'inet addr:192.0.2.1'\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	script := `. ./kindle-client/dashboard-extension/offline-network.sh; network_wifi_state; network_wifi_set 0; network_wait_connected`
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "KINDLE_WIFI_WAIT_SECONDS=1")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "1\n0" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestOfflineFetchTurnsWiFiOffAfterSuccessfulDownload(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	calls := filepath.Join(dir, "wifi")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write("offline-network.sh", mustReadTestFile(t, "kindle-client/dashboard-extension/offline-network.sh"))
	write("server-settings.sh", "#!/bin/sh\necho http://example.test\n")
	write("lipc-set-prop", "#!/bin/sh\necho \"$3\" >> \"$WIFI_CALLS\"\n")
	write("wpa_cli", "#!/bin/sh\necho wpa_state=COMPLETED\n")
	write("ifconfig", "#!/bin/sh\necho 'inet addr:192.0.2.1'\n")
	f, err := os.Create(filepath.Join(dir, "tiles.tar"))
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	for _, name := range []string{"0.png", "1.png", "2.png", "3.png", "4.png", "5.png", "6.png", "7.png", "8.png", "9.png", "colon.png"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	write("curl", `#!/bin/sh
case "$*" in *clock-tiles.tar*) tiles=yes;; esac
while [ "$#" -gt 0 ]; do
 case "$1" in -D) headers=$2; shift;; -o) body=$2; shift;; esac
 shift
done
printf 'HTTP/1.1 200 OK\r\nX-Dashboard-Clock: local\r\nX-Dashboard-Layout: top-v2\r\nX-Dashboard-Timezone-Offset: 8\r\n\r\n' > "$headers"
if [ "$tiles" = yes ]; then cp "$TEST_TILES" "$body"; else printf png > "$body"; fi
`)
	cmd := exec.Command("sh", "kindle-client/dashboard-extension/offline-fetch.sh")
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "KINDLE_DASHBOARD_DIR="+dir, "KINDLE_OFFLINE_STATE="+state, "KINDLE_SCREEN_WIDTH=1072", "KINDLE_SCREEN_HEIGHT=1448", "WIFI_CALLS="+calls, "TEST_TILES="+filepath.Join(dir, "tiles.tar"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(calls)
		if strings.Contains(string(b), "1\n0\n") {
			ready, err := os.ReadFile(filepath.Join(state, "ready"))
			if err != nil {
				t.Fatal(err)
			}
			y, err := os.ReadFile(filepath.Join(state, strings.TrimSpace(string(ready)), "clock-y"))
			if err != nil || strings.TrimSpace(string(y)) != "69" {
				t.Fatalf("clock position = %q, %v", y, err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, _ := os.ReadFile(calls)
	t.Fatalf("Wi-Fi was not enabled and disabled: %q", b)
}

func mustReadTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClockTimezoneFollowsServerOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "offset")
	for _, tc := range []struct{ value, want string }{{"8", "UTC-8"}, {"-5", "UTC+5"}, {"0", "UTC-0"}, {"99", "UTC-8"}, {"bad", "UTC-8"}} {
		if err := os.WriteFile(path, []byte(tc.value), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "-c", `. ./kindle-client/dashboard-extension/offline-network.sh; clock_tz "$1"`, "sh", path)
		out, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(out)) != tc.want {
			t.Errorf("%q: got %q, %v", tc.value, out, err)
		}
	}
}

func TestTwoHourSlotsAlignToLocalEvenHours(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "offset")
	for _, tc := range []struct {
		offset string
		local  time.Time
		same   bool
	}{
		{"8", time.Date(2026, 11, 1, 1, 59, 59, 0, time.FixedZone("CST", 8*3600)), false},
		{"8", time.Date(2026, 11, 1, 2, 0, 0, 0, time.FixedZone("CST", 8*3600)), true},
		{"-5", time.Date(2026, 11, 1, 2, 0, 0, 0, time.FixedZone("EST", -5*3600)), true},
	} {
		if err := os.WriteFile(path, []byte(tc.offset), 0600); err != nil {
			t.Fatal(err)
		}
		slot := func(at time.Time) string {
			t.Helper()
			out, err := exec.Command("sh", "-c", `. ./kindle-client/dashboard-extension/offline-network.sh; clock_two_hour_slot "$1" "$2"`, "sh", strconv.FormatInt(at.Unix(), 10), path).Output()
			if err != nil {
				t.Fatal(err)
			}
			return strings.TrimSpace(string(out))
		}
		previous := slot(tc.local.Add(-time.Second))
		current := slot(tc.local)
		if (previous != current) != tc.same {
			t.Errorf("%s: previous=%s current=%s, expected boundary=%v", tc.local, previous, current, tc.same)
		}
	}
}
