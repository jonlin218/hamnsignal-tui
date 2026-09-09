// Package audio manages the optional external mpv playback process.
package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const StreamURL = "https://hamnsignal.se/live.mp3"

type Status string

const (
	Disabled    Status = "disabled"
	Unavailable Status = "unavailable"
	Starting    Status = "starting"
	Playing     Status = "playing"
	Paused      Status = "paused"
	Stopped     Status = "stopped"
	Error       Status = "error"
)

var (
	ErrDisabled    = errors.New("audio is disabled")
	ErrUnavailable = errors.New("mpv is unavailable")
)

type Options struct {
	Disabled  bool
	MPVPath   string
	StreamURL string
	OnStatus  func(Status)
}

type Player struct {
	mu       sync.Mutex
	opMu     sync.Mutex
	options  Options
	status   Status
	cmd      *exec.Cmd
	done     chan struct{}
	socket   string
	tempDir  string
	stopping bool
}

func NewPlayer(options Options) *Player {
	if options.StreamURL == "" {
		options.StreamURL = StreamURL
	}
	status := Stopped
	if options.Disabled {
		status = Disabled
	}
	return &Player{options: options, status: status}
}

func (p *Player) Status() Status { p.mu.Lock(); defer p.mu.Unlock(); return p.status }

// Start attempts one playback start. It does not retry automatically.
func (p *Player) Start() error {
	p.opMu.Lock()
	p.mu.Lock()
	if p.options.Disabled {
		p.setStatusLocked(Disabled)
		p.mu.Unlock()
		p.opMu.Unlock()
		return ErrDisabled
	}
	if p.cmd != nil && (p.status == Starting || p.status == Playing || p.status == Paused) {
		p.mu.Unlock()
		p.opMu.Unlock()
		return nil
	}
	path := p.options.MPVPath
	if path == "" {
		path, _ = exec.LookPath("mpv")
	}
	if path == "" {
		p.setStatusLocked(Unavailable)
		p.mu.Unlock()
		p.opMu.Unlock()
		return ErrUnavailable
	}
	tempDir, err := os.MkdirTemp("", "hamnsignal-mpv-")
	if err != nil {
		p.setStatusLocked(Error)
		p.mu.Unlock()
		p.opMu.Unlock()
		return fmt.Errorf("create mpv IPC directory: %w", err)
	}
	socket := filepath.Join(tempDir, "mpv.sock")
	cmd := exec.Command(path, mpvArgs(socket, p.options.StreamURL)...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tempDir)
		p.setStatusLocked(Unavailable)
		p.mu.Unlock()
		p.opMu.Unlock()
		return fmt.Errorf("start mpv: %w", err)
	}
	p.cmd, p.done, p.socket, p.tempDir, p.stopping = cmd, make(chan struct{}), socket, tempDir, false
	p.setStatusLocked(Starting)
	done := p.done
	p.mu.Unlock()
	p.opMu.Unlock()
	go p.monitor(cmd, done)
	if !waitForSocket(socket, 1500*time.Millisecond) {
		p.mu.Lock()
		stillCurrent := p.cmd == cmd
		p.mu.Unlock()
		if stillCurrent {
			_ = p.Stop()
		}
		_ = os.RemoveAll(tempDir)
		p.mu.Lock()
		if !p.options.Disabled {
			p.setStatusLocked(Unavailable)
		}
		p.mu.Unlock()
		return errors.New("mpv IPC socket did not become ready")
	}
	p.mu.Lock()
	if p.cmd == cmd {
		p.setStatusLocked(Playing)
	}
	p.mu.Unlock()
	return nil
}

// mpvArgs stays deliberately close to the known-good direct invocation. The
// IPC server is the one additional facility the application needs for pause,
// resume, and graceful cleanup.
func mpvArgs(socket, streamURL string) []string {
	return []string{"--no-video", "--really-quiet", "--input-ipc-server=" + socket, "--", streamURL}
}

func (p *Player) Toggle() error {
	switch p.Status() {
	case Playing:
		return p.Pause()
	case Paused:
		return p.Resume()
	default:
		return p.Start()
	}
}

func (p *Player) Pause() error {
	p.mu.Lock()
	if p.status == Disabled {
		p.mu.Unlock()
		return ErrDisabled
	}
	socket, active := p.socket, p.cmd != nil
	p.mu.Unlock()
	if !active {
		return p.Start()
	}
	p.opMu.Lock()
	defer p.opMu.Unlock()
	if err := sendCommand(socket, []any{"set_property", "pause", true}); err != nil {
		p.setError(err)
		return err
	}
	p.mu.Lock()
	if p.cmd != nil {
		p.setStatusLocked(Paused)
	}
	p.mu.Unlock()
	return nil
}

func (p *Player) Resume() error {
	p.mu.Lock()
	if p.status == Disabled {
		p.mu.Unlock()
		return ErrDisabled
	}
	socket, active := p.socket, p.cmd != nil
	p.mu.Unlock()
	if !active {
		return p.Start()
	}
	p.opMu.Lock()
	defer p.opMu.Unlock()
	if err := sendCommand(socket, []any{"set_property", "pause", false}); err != nil {
		p.setError(err)
		return err
	}
	p.mu.Lock()
	if p.cmd != nil {
		p.setStatusLocked(Playing)
	}
	p.mu.Unlock()
	return nil
}

// Stop asks mpv to quit, then uses a bounded graceful process signal fallback.
func (p *Player) Stop() error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	cmd, done, socket, tempDir := p.cmd, p.done, p.socket, p.tempDir
	if cmd == nil {
		if p.options.Disabled {
			p.setStatusLocked(Disabled)
		} else {
			p.setStatusLocked(Stopped)
		}
		p.mu.Unlock()
		return nil
	}
	p.stopping = true
	p.setStatusLocked(Stopped)
	p.mu.Unlock()
	_ = sendCommand(socket, []any{"quit"})
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	_ = os.RemoveAll(tempDir)
	return nil
}

func (p *Player) monitor(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	p.mu.Lock()
	tempDir := ""
	if p.cmd == cmd {
		tempDir = p.tempDir
		p.cmd = nil
		p.socket = ""
		p.tempDir = ""
		if !p.stopping {
			if err != nil {
				p.setStatusLocked(Error)
			} else {
				p.setStatusLocked(Stopped)
			}
		}
	}
	p.mu.Unlock()
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
	close(done)
}

func (p *Player) setStatusLocked(status Status) {
	if p.status == status {
		return
	}
	p.status = status
	if p.options.OnStatus != nil {
		p.options.OnStatus(status)
	}
}
func (p *Player) setError(err error) {
	p.mu.Lock()
	if p.cmd != nil && !p.stopping {
		p.setStatusLocked(Error)
	}
	p.mu.Unlock()
	_ = err
}

func waitForSocket(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("unix", path, 50*time.Millisecond); err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func sendCommand(socket string, command []any) error {
	conn, err := net.DialTimeout("unix", socket, 250*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	payload, err := json.Marshal(map[string]any{"command": command})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = conn.Write(payload)
	return err
}
