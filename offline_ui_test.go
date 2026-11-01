package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOfflineUIGuardResumesAfterOwnerDies(t *testing.T) {
	owner := exec.Command("sleep", "30")
	wm := exec.Command("sleep", "30")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Process.Kill(); _, _ = owner.Process.Wait() }()
	if err := wm.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wm.Process.Signal(syscall.SIGCONT); _ = wm.Process.Kill(); _, _ = wm.Process.Wait() }()
	if err := wm.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	guard := exec.Command("sh", "kindle-client/dashboard-extension/offline-ui-guard.sh", strconv.Itoa(owner.Process.Pid), strconv.Itoa(wm.Process.Pid))
	if err := guard.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = guard.Process.Kill(); _, _ = guard.Process.Wait() }()
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_, _ = owner.Process.Wait()
	done := make(chan error, 1)
	go func() { done <- guard.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("recovery guard timed out")
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(wm.Process.Pid)).Output()
	if err != nil || strings.HasPrefix(strings.TrimSpace(string(out)), "T") {
		t.Fatalf("window manager still stopped: %q, %v", out, err)
	}
}
