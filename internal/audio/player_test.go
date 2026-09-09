package audio

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMPVArgsUseAnIPCServerWithoutIdleMode(t *testing.T) {
	args := mpvArgs("/tmp/hamnsignal-mpv.sock", "https://example.test/live.mp3")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--input-ipc-server=/tmp/hamnsignal-mpv.sock") {
		t.Fatalf("IPC server argument missing: %q", args)
	}
	if strings.Contains(joined, "input-ipc-client") || strings.Contains(joined, "--idle=yes") {
		t.Fatalf("unsupported playback lifecycle option present: %q", args)
	}
}

func TestInitialAndDisabledStates(t *testing.T) {
	player := NewPlayer(Options{Disabled: true})
	if player.Status() != Disabled {
		t.Fatalf("initial status = %s", player.Status())
	}
	if err := player.Start(); err != ErrDisabled {
		t.Fatalf("start error = %v", err)
	}
	if err := player.Toggle(); err != ErrDisabled {
		t.Fatalf("toggle error = %v", err)
	}
	if err := player.Stop(); err != nil {
		t.Fatal(err)
	}
	if player.Status() != Disabled {
		t.Fatalf("status after stop = %s", player.Status())
	}
}

func TestMissingMPVIsUnavailableWithoutRetryLoop(t *testing.T) {
	player := NewPlayer(Options{MPVPath: filepath.Join(t.TempDir(), "missing-mpv")})
	if err := player.Start(); err == nil {
		t.Fatal("expected missing mpv error")
	}
	if player.Status() != Unavailable {
		t.Fatalf("status = %s", player.Status())
	}
}

func TestPauseResumeSendsSmallIPCCommands(t *testing.T) {
	socket := filepath.Join("/tmp", "hamnsignal-mpv-test-"+strconv.Itoa(os.Getpid())+".sock")
	_ = os.Remove(socket)
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("unix IPC sockets unavailable in this test environment: %v", err)
	}
	defer listener.Close()
	commands := make(chan []any, 2)
	go func() {
		for i := 0; i < 2; i++ {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			var packet map[string][]any
			if decodeErr := json.NewDecoder(conn).Decode(&packet); decodeErr == nil {
				commands <- packet["command"]
			}
			_ = conn.Close()
		}
	}()
	player := &Player{status: Playing, cmd: &exec.Cmd{}, socket: socket}
	if err := player.Pause(); err != nil {
		t.Fatal(err)
	}
	if player.Status() != Paused {
		t.Fatalf("status after pause = %s", player.Status())
	}
	if err := player.Resume(); err != nil {
		t.Fatal(err)
	}
	if player.Status() != Playing {
		t.Fatalf("status after resume = %s", player.Status())
	}
	for _, want := range []bool{true, false} {
		select {
		case command := <-commands:
			if len(command) != 3 || command[0] != "set_property" || command[1] != "pause" || command[2] != want {
				t.Fatalf("command = %#v", command)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for IPC command")
		}
	}
}

func TestPlayerExitDoesNotLeaveSocketDirectory(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-mpv")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	player := NewPlayer(Options{MPVPath: script})
	if err := player.Start(); err == nil {
		t.Fatal("expected fake player without IPC socket to fail")
	}
	if player.Status() != Unavailable {
		t.Fatalf("status = %s", player.Status())
	}
}
