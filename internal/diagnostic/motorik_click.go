// Package diagnostic contains development-only inspection aids.
package diagnostic

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/jonlin218/hamnsignal-tui/internal/visual"
)

var ErrClickUnavailable = errors.New("afplay is unavailable")

const clickSampleRate = 48000

// MotorikClick plays a short generated reference track through macOS afplay.
// One lightweight player process carries many pulses, avoiding per-quarter
// process churn; every replacement track is re-anchored to the shared clock.
type MotorikClick struct {
	commandPath string
	chunk       time.Duration
	mu          sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
	path        string
}

func NewMotorikClick() *MotorikClick { return &MotorikClick{chunk: 20 * time.Second} }

func (c *MotorikClick) Start(parent context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return nil
	}
	path := c.commandPath
	if path == "" {
		path, _ = exec.LookPath("afplay")
	}
	if path == "" {
		return ErrClickUnavailable
	}
	file, err := os.CreateTemp("", "hamnsignal-motorik-click-*.wav")
	if err != nil {
		return err
	}
	if err := writeClickWAV(file, c.chunk); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel, c.done, c.path = cancel, make(chan struct{}), file.Name()
	go c.run(ctx, path, file.Name())
	return nil
}

func (c *MotorikClick) run(ctx context.Context, commandPath, path string) {
	defer func() {
		_ = os.Remove(path)
		c.mu.Lock()
		c.cancel, c.path = nil, ""
		close(c.done)
		c.mu.Unlock()
	}()
	for {
		next := NextBoundary(time.Now())
		wait := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			wait.Stop()
			return
		case <-wait.C:
		}
		cmd := exec.CommandContext(ctx, commandPath, "-v", "0.25", path)
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Start(); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
	}
}

// Stop cancels the scheduler and waits for its active afplay process to exit.
func (c *MotorikClick) Stop() {
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// NextBoundary intentionally delegates to the visual clock, so a delayed
// scheduler skips historical beats and always targets the next future pulse.
func NextBoundary(now time.Time) time.Time { return visual.NextMotorikQuarterBoundary(now) }

func writeClickWAV(file *os.File, duration time.Duration) error {
	samples := int(math.Ceil(duration.Seconds() * clickSampleRate))
	dataSize := samples * 2
	header := make([]byte, 44)
	copy(header[0:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+dataSize))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], clickSampleRate)
	binary.LittleEndian.PutUint32(header[28:], clickSampleRate*2)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(dataSize))
	if _, err := file.Write(header); err != nil {
		return err
	}
	pcm := make([]byte, dataSize)
	clickSamples := int(clickSampleRate * .004)
	for quarter := 0; ; quarter++ {
		start := int(math.Round(float64(quarter) * visual.MotorikQuarterDuration().Seconds() * clickSampleRate))
		if start >= samples {
			break
		}
		for offset := 0; offset < clickSamples && start+offset < samples; offset++ {
			envelope := math.Exp(-float64(offset) / float64(clickSamples) * 7)
			value := int16(math.Round(math.Sin(2*math.Pi*1700*float64(offset)/clickSampleRate) * envelope * 9000))
			binary.LittleEndian.PutUint16(pcm[(start+offset)*2:], uint16(value))
		}
	}
	_, err := file.Write(pcm)
	return err
}
