//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLandscapeUIRestoresWindowManager(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(strconv.FormatBool(forced), func(t *testing.T) {
			wm := exec.Command("sleep", "30")
			if err := wm.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = wm.Process.Signal(syscall.SIGCONT); _ = wm.Process.Kill(); _, _ = wm.Process.Wait() }()
			dir, err := filepath.Abs("kindle-client/dashboard-extension")
			if err != nil {
				t.Fatal(err)
			}
			state := t.TempDir()
			program := `ps() { printf '%s awesome\n' "$TEST_WM_PID"; }; . "$KINDLE_DASHBOARD_DIR/landscape-ui.sh"; trap landscape_ui_cleanup TERM INT HUP; landscape_ui_start || exit 2; echo ready > "$TMP_DIR/ready"; while :; do landscape_sleep 30; done`
			owner := exec.Command("sh", "-c", program)
			owner.Env = append(os.Environ(), "KINDLE_DASHBOARD_DIR="+dir, "TMP_DIR="+state, "TEST_WM_PID="+strconv.Itoa(wm.Process.Pid))
			if err := owner.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = owner.Process.Kill(); _, _ = owner.Process.Wait() }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(state, "ready")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("client failed to pause window manager")
				}
				time.Sleep(50 * time.Millisecond)
			}
			status := func() string {
				out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(wm.Process.Pid)).Output()
				return strings.TrimSpace(string(out))
			}
			if !strings.HasPrefix(status(), "T") {
				t.Fatalf("window manager was not paused: %s", status())
			}
			signal := syscall.SIGTERM
			if forced {
				signal = syscall.SIGKILL
			}
			if err := owner.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			_, _ = owner.Process.Wait()
			deadline = time.Now().Add(8 * time.Second)
			for strings.HasPrefix(status(), "T") {
				if time.Now().After(deadline) {
					t.Fatal("window manager was not resumed")
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}
