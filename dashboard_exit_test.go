package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestPowerButtonFourPressesExit(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "key")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	supervisor := exec.Command("sleep", "30")
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = supervisor.Process.Kill(); _, _ = supervisor.Process.Wait() }()
	watcher := exec.Command("sh", "kindle-client/dashboard-extension/exit-button.sh", strconv.Itoa(supervisor.Process.Pid))
	watcher.Env = append(os.Environ(), "KINDLE_POWER_DEVICE="+fifo, "KINDLE_DASHBOARD_STATE="+dir)
	if err := watcher.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watcher.Process.Kill(); _, _ = watcher.Process.Wait() }()
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()
		for i := 0; i < 4; i++ {
			e := make([]byte, 16)
			binary.LittleEndian.PutUint16(e[8:], 1)
			binary.LittleEndian.PutUint16(e[10:], 116)
			binary.LittleEndian.PutUint32(e[12:], 1)
			if _, err = f.Write(e); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("input writer timed out")
	}
	exited := make(chan error, 1)
	go func() { exited <- supervisor.Wait() }()
	select {
	case err := <-exited:
		if err == nil {
			t.Fatal("expected signal termination")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("four presses did not stop supervisor")
	}
}
