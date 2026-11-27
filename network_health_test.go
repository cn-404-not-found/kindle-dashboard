package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// networkStubs installs fake Kindle tools (lipc, wpa_cli, ifconfig) in a temp
// PATH so network-health.sh can be exercised without a device.
type networkStubs struct {
	dir   string
	state string
	calls string
}

func newNetworkStubs(t *testing.T, radio, wpa string) *networkStubs {
	t.Helper()
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	if err := os.MkdirAll(state, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"lipc-get-prop": `#!/bin/sh
case "$2" in
  wirelessEnable) cat "$WIFI_STATE/radio" ;;
  *) exit 1 ;;
esac
`,
		"lipc-set-prop": `#!/bin/sh
echo "$2 $3" >> "$WIFI_CALLS"
if [ "$2" = wirelessEnable ]; then echo "$3" > "$WIFI_STATE/radio"; fi
`,
		"wpa_cli": `#!/bin/sh
case "$*" in
  *reassociate*)
    echo reassociate >> "$WIFI_CALLS"
    if [ "${WIFI_REASSOCIATE_RESTORES:-0}" = 1 ]; then echo COMPLETED > "$WIFI_STATE/wpa"; fi
    ;;
  *) echo "wpa_state=$(cat "$WIFI_STATE/wpa")" ;;
esac
`,
		"ifconfig": `#!/bin/sh
if [ "$(cat "$WIFI_STATE/wpa")" = COMPLETED ]; then echo 'inet addr:192.0.2.1'; fi
`,
		// Keep the recovery test fast; only the script under test reaches it.
		"sleep": "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]string{"radio": radio, "wpa": wpa} {
		if err := os.WriteFile(filepath.Join(state, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return &networkStubs{dir: dir, state: state, calls: filepath.Join(dir, "calls")}
}

func (s *networkStubs) run(t *testing.T, script string, extraEnv ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "-c", ". ./kindle-client/dashboard-extension/network-health.sh; "+script)
	cmd.Env = append(os.Environ(),
		"PATH="+s.dir+":"+os.Getenv("PATH"),
		"WIFI_STATE="+s.state,
		"WIFI_CALLS="+s.calls,
		"KINDLE_WIFI_RECOVER_SECONDS=2",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s *networkStubs) callLog(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(s.calls)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(b)
}

func TestNetworkHealthHealthyLinkKeepsAwake(t *testing.T) {
	s := newNetworkStubs(t, "1", "COMPLETED")
	out, err := s.run(t, "network_keep_awake; network_online")
	if err != nil {
		t.Fatalf("network_online failed on a healthy link: %v (%s)", err, out)
	}
	if got := s.callLog(t); !strings.Contains(got, "preventScreenSaver 1") {
		t.Fatalf("screensaver block not re-asserted: %q", got)
	}
}

func TestNetworkEnsureOnlineReassociates(t *testing.T) {
	s := newNetworkStubs(t, "1", "DISCONNECTED")
	out, err := s.run(t, "network_ensure_online", "WIFI_REASSOCIATE_RESTORES=1")
	if err != nil {
		t.Fatalf("reassociate recovery failed: %v (%s)", err, out)
	}
	calls := s.callLog(t)
	if !strings.Contains(calls, "reassociate") {
		t.Fatalf("reassociate was not attempted: %q", calls)
	}
	if strings.Contains(calls, "wirelessEnable 0") {
		t.Fatalf("radio toggled although reassociate sufficed: %q", calls)
	}
}

func TestNetworkEnsureOnlineTogglesRadio(t *testing.T) {
	s := newNetworkStubs(t, "0", "DISCONNECTED")
	out, err := s.run(t, "network_ensure_online")
	if err == nil {
		t.Fatalf("expected failure when the link never returns: %s", out)
	}
	calls := s.callLog(t)
	for _, want := range []string{"wirelessEnable 1", "reassociate", "wirelessEnable 0"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in recovery log: %q", want, calls)
		}
	}
}

func TestNetworkEnsureOnlineAppliesStaticIP(t *testing.T) {
	s := newNetworkStubs(t, "1", "COMPLETED")
	ifconfig := `#!/bin/sh
if [ -f "$WIFI_STATE/ip_ready" ]; then echo 'inet addr:192.0.2.10'; fi
`
	if err := os.WriteFile(filepath.Join(s.dir, "ifconfig"), []byte(ifconfig), 0755); err != nil {
		t.Fatal(err)
	}
	netDir := filepath.Join(s.dir, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(netDir, "mode"), []byte("static\n"), 0644); err != nil {
		t.Fatal(err)
	}
	autoScript := `#!/bin/sh
echo "auto:$1" >> "$WIFI_CALLS"
if [ "$1" = --tick ]; then
  : > "$WIFI_STATE/ip_ready"
fi
`
	if err := os.WriteFile(filepath.Join(netDir, "auto.sh"), []byte(autoScript), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := s.run(t, "network_ensure_online", "KINDLE_NET_CONFIG_DIR="+netDir)
	if err != nil {
		t.Fatalf("static IP recovery failed: %v (%s)", err, out)
	}
	calls := s.callLog(t)
	for _, want := range []string{"auto:--ensure", "auto:--tick"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in recovery log: %q", want, calls)
		}
	}
	if strings.Contains(calls, "wirelessEnable 0") {
		t.Fatalf("radio toggled although auto.sh restored static IP: %q", calls)
	}
}

func TestNetworkForceRecoverAndEventLog(t *testing.T) {
	s := newNetworkStubs(t, "1", "COMPLETED")
	netDir := filepath.Join(s.dir, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(netDir, "mode"), []byte("static\n"), 0644); err != nil {
		t.Fatal(err)
	}
	autoScript := `#!/bin/sh
echo "auto:$1" >> "$WIFI_CALLS"
`
	if err := os.WriteFile(filepath.Join(netDir, "auto.sh"), []byte(autoScript), 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(s.dir, "logs", "wifi-events.log")

	out, err := s.run(t, "network_force_recover 3",
		"KINDLE_NET_CONFIG_DIR="+netDir,
		"KINDLE_WIFI_EVENT_LOG="+logPath,
	)
	if err != nil {
		t.Fatalf("soft force recover (streak=3) failed: %v (%s)", err, out)
	}
	calls := s.callLog(t)
	for _, want := range []string{"auto:--tick", "enable 1", "ensureConnection wifi", "reassociate"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q after force recover 3: %q", want, calls)
		}
	}
	if strings.Contains(calls, "wirelessEnable 0") {
		t.Fatalf("streak=3 should soft-recover without radio cycle when connected: %q", calls)
	}

	out, err = s.run(t, "network_force_recover 6",
		"KINDLE_NET_CONFIG_DIR="+netDir,
		"KINDLE_WIFI_EVENT_LOG="+logPath,
	)
	if err != nil {
		t.Fatalf("hard force recover (streak=6) failed: %v (%s)", err, out)
	}
	calls = s.callLog(t)
	for _, want := range []string{"wirelessEnable 0", "enable 0"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q after force recover 6: %q", want, calls)
		}
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	logText := string(raw)
	for _, want := range []string{
		"Consecutive fetch failures (3); forcing Wi-Fi recovery.",
		"Escalating to radio toggle (streak=6).",
		"Forced Wi-Fi recovery completed.",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("missing %q in wifi event log:\n%s", want, logText)
		}
	}

	var seed strings.Builder
	for i := 0; i < 225; i++ {
		seed.WriteString("[2026-11-27 00:00:00] old line\n")
	}
	if err := os.WriteFile(logPath, []byte(seed.String()), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := s.run(t, "network_log 'trim check'", "KINDLE_WIFI_EVENT_LOG="+logPath); err != nil {
		t.Fatalf("network_log failed: %v (%s)", err, out)
	}
	trimmed, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(trimmed)), "\n")
	if len(lines) > 200 {
		t.Fatalf("event log exceeded 200-line cap: got %d lines", len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "trim check") {
		t.Fatalf("trimmed log lost latest event: %q", lines[len(lines)-1])
	}
}
