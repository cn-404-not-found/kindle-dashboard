package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDashboardPowerLifecycle(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	log := filepath.Join(dir, "calls")
	touch := filepath.Join(dir, "touch")
	if err := syscall.Mkfifo(touch, 0600); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	setter := write("set", "#!/bin/sh\necho \"$2 $3\" >> \"$TEST_CALLS\"\n")
	getter := write("get", "#!/bin/sh\ncase \"$2\" in flIntensity) echo 19;; preventScreenSaver) echo 0;; esac\n")
	dashboard := write("dashboard.sh", "#!/bin/sh\nsleep 20 & wait $!\n")
	_ = dashboard
	source, err := os.ReadFile("kindle-client/dashboard-extension/touch-light.sh")
	if err != nil {
		t.Fatal(err)
	}
	write("touch-light.sh", string(source))
	cmd := exec.Command("/bin/sh", "kindle-client/dashboard-extension/power.sh")
	cmd.Env = append(os.Environ(), "KINDLE_DASHBOARD_DIR="+dir, "KINDLE_DASHBOARD_STATE="+state, "KINDLE_TOUCH_DEVICE="+touch, "KINDLE_LIPC_SET="+setter, "KINDLE_LIPC_GET="+getter, "KINDLE_DASHBOARD_LOG="+filepath.Join(dir, "debug"), "KINDLE_TOUCH_TIMEOUT=1", "TEST_CALLS="+log)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	waitFor := func(want string, count int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			b, _ := os.ReadFile(log)
			if strings.Count(string(b), want) >= count {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		b, _ := os.ReadFile(log)
		t.Fatalf("missing %q (%d): %s", want, count, b)
	}
	waitFor("flIntensity 0", 1)
	go func() {
		f, err := os.OpenFile(touch, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		defer f.Close()
		event := make([]byte, 16)
		binary.LittleEndian.PutUint16(event[8:], 3)
		_, _ = f.Write(event)
	}()
	waitFor("flIntensity 12", 1)
	waitFor("flIntensity 0", 2)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	waitFor("flIntensity 19", 1)
	waitFor("preventScreenSaver 0", 1)
}
