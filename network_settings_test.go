package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkSettingsValidation(t *testing.T) {
	tests := []struct {
		ip, mask, gateway, dns string
		valid                  bool
	}{
		{"192.168.10.30", "255.255.255.0", "192.168.10.1", "1.1.1.1", true},
		{"10.0.0.2", "255.255.254.0", "10.0.0.1", "8.8.8.8", true},
		{"192.168.1.2", "255.255.255.0", "192.168.2.1", "8.8.8.8", false},
		{"192.168.1.0", "255.255.255.0", "192.168.1.1", "8.8.8.8", false},
		{"192.168.1.999", "255.255.255.0", "192.168.1.1", "8.8.8.8", false},
		{"192.168.1.2", "255.0.255.0", "192.168.1.1", "8.8.8.8", false},
		{"192.168.1.2", "255.255.255.0", "192.168.1.1", "8.8.8.8;touch /tmp/bad", false},
	}
	for _, tt := range tests {
		err := exec.Command("sh", "kindle-client/network-settings/network.sh", "--validate", tt.ip, tt.mask, tt.gateway, tt.dns).Run()
		if (err == nil) != tt.valid {
			t.Errorf("%+v: error %v", tt, err)
		}
	}
}

// TestNetworkSettingsSave confirms that entering settings does not modify the host network.
func TestNetworkSettingsSave(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("sh", "kindle-client/network-settings/network.sh")
	cmd.Env = append(os.Environ(), "KINDLE_NET_CONFIG_DIR="+dir, "TERM=xterm")
	cmd.Stdin = strings.NewReader("1\n192.168.10.30\n255.255.255.0\n192.168.10.1\n1.1.1.1\ny\n\n8\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("menu failed: %v: %s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(dir, "network.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "192.168.10.30\n255.255.255.0\n192.168.10.1\n1.1.1.1\n" {
		t.Fatalf("unexpected saved settings: %q", b)
	}
}

func TestNetworkSettingsPingTarget(t *testing.T) {
	for _, tt := range []struct {
		target string
		valid  bool
	}{{"192.168.10.254", true}, {"example.com", true}, {"-c", false}, {"abc;reboot", false}, {"", false}} {
		err := exec.Command("sh", "kindle-client/network-settings/network.sh", "--validate-target", tt.target).Run()
		if (err == nil) != tt.valid {
			t.Errorf("target %q: %v", tt.target, err)
		}
	}
}

func TestAutomaticStaticModeOnlyAppliesToSelectedWiFi(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("kindle-client/network-settings/network.sh")
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"network.sh": string(script), "mode": "static\n", "ssid": "target wifi\n", "network.conf": "192.168.10.126\n255.255.255.0\n192.168.10.254\n192.168.10.254\n"} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	logpath := filepath.Join(dir, "calls")
	mocks := map[string]string{
		"wpa_cli":  "#!/bin/sh\nprintf 'wpa_state=COMPLETED\\nssid=%s\\n' \"$TEST_SSID\"\n",
		"ifconfig": "#!/bin/sh\nif [ $# -eq 1 ]; then echo 'inet addr:192.168.10.128 Mask:255.255.255.0'; else echo ifconfig >> \"$TEST_LOG\"; fi\n",
		"route":    "#!/bin/sh\necho '0.0.0.0 192.168.10.1 0.0.0.0 UG 0 0 0 wlan0'\n",
		"ip":       "#!/bin/sh\necho ip >> \"$TEST_LOG\"\n",
	}
	for name, body := range mocks {
		if err = os.WriteFile(filepath.Join(bin, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		ssid string
		want bool
	}{{"other wifi", false}, {"target wifi", true}} {
		os.Remove(logpath)
		cmd := exec.Command("sh", "kindle-client/network-settings/auto.sh", "--tick")
		cmd.Env = append(os.Environ(), "KINDLE_NET_CONFIG_DIR="+dir, "KINDLE_NET_PATH="+bin+":/usr/bin:/bin", "KINDLE_NET_RESOLV_PATH="+filepath.Join(dir, "resolv.conf"), "TEST_SSID="+tc.ssid, "TEST_LOG="+logpath)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v: %s", tc.ssid, e, out)
		}
		b, e := os.ReadFile(logpath)
		if tc.want && (e != nil || !strings.Contains(string(b), "ifconfig") || !strings.Contains(string(b), "ip")) {
			t.Fatalf("missing network updates: %q, %v", b, e)
		}
		if !tc.want && e == nil {
			t.Fatalf("changed a different network: %s", b)
		}
	}
}

func TestRootMountModeUsesOvermountedRoot(t *testing.T) {
	for _, tc := range []struct{ mounts, want string }{{"rootfs / rootfs rw 0 0\n/dev/root / ext3 ro,noatime 0 0\n", "ro"}, {"rootfs / rootfs rw 0 0\n/dev/root / ext3 rw,noatime 0 0\n", "rw"}} {
		cmd := exec.Command("sh", "kindle-client/network-settings/network.sh", "--root-mount-mode")
		cmd.Stdin = strings.NewReader(tc.mounts)
		out, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(out)) != tc.want {
			t.Errorf("mounts %q: got %q, err %v", tc.mounts, out, err)
		}
	}
}
