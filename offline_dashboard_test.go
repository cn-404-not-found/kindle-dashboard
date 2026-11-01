package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOfflineClockDrawsOriginalTilesAtFixedPositions(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	bundle := filepath.Join(state, "bundle-1")
	log := filepath.Join(dir, "draws")
	if err := os.MkdirAll(bundle, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ready", "bundle-1/background.png"} {
		p := filepath.Join(state, name)
		value := "png"
		if name == "ready" {
			value = "bundle-1\n"
		}
		if err := os.WriteFile(p, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write("offline-network.sh", mustReadTestFile(t, "kindle-client/dashboard-extension/offline-network.sh"))
	write("offline-fetch.sh", "#!/bin/sh\nsleep 30 & wait $!\n")
	write("lipc-get-prop", "#!/bin/sh\necho 1\n")
	write("lipc-set-prop", "#!/bin/sh\nexit 0\n")
	write("date", "#!/bin/sh\ncase \"$1\" in +%s) echo 59;; +%H:%M) echo 18:58;; esac\n")
	write("eips", "#!/bin/sh\nif [ \"$1\" = -i ]; then echo 'xres: 1072 yres: 1448'; else echo \"$*\" >> \"$DRAW_LOG\"; fi\n")
	cmd := exec.Command("sh", "kindle-client/dashboard-extension/offline-dashboard.sh")
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "KINDLE_DASHBOARD_DIR="+dir, "KINDLE_OFFLINE_STATE="+state, "DRAW_LOG="+log)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(log)
		s := string(b)
		if strings.Contains(s, "5.png -x 237 -y 110") && strings.Contains(s, "8.png -x 301 -y 110") {
			if !strings.Contains(s, "colon.png -x 173 -y 110") {
				t.Fatal(s)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	t.Fatalf("missing clock tiles: %s", b)
}
