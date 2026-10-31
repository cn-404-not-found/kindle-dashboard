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
	cmd.Stdin = strings.NewReader("1\n192.168.10.30\n255.255.255.0\n192.168.10.1\n1.1.1.1\ny\n\n6\n")
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
