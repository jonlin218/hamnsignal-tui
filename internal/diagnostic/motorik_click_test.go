package diagnostic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jonlin218/hamnsignal-tui/internal/visual"
)

func TestNextBoundaryUsesSharedMotorikClockAndSkipsMissedPulses(t *testing.T) {
	now := visual.MotorikQuarterBoundary(100).Add(20 * time.Millisecond)
	next := NextBoundary(now)
	if want := visual.MotorikQuarterBoundary(101); next != want {
		t.Fatalf("next=%s want %s", next, want)
	}
	delayed := visual.MotorikQuarterBoundary(100).Add(3*visual.MotorikQuarterDuration() + 20*time.Millisecond)
	if got, want := NextBoundary(delayed), visual.MotorikQuarterBoundary(104); got != want {
		t.Fatalf("delayed scheduler replayed or misaligned pulses: got %s want %s", got, want)
	}
}

func TestClickWAVContainsBoundedReferenceTrack(t *testing.T) {
	file, err := os.CreateTemp("", "hamnsignal-click-test-*.wav")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := writeClickWAV(file, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 44 {
		t.Fatalf("click WAV was not written: info=%v err=%v", info, err)
	}
}

func TestClickLifecycleStopsAndCleansTemporaryTrack(t *testing.T) {
	click := NewMotorikClick()
	click.commandPath = "/usr/bin/false"
	click.chunk = time.Second
	if err := click.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	click.mu.Lock()
	path := click.path
	click.mu.Unlock()
	if path == "" {
		t.Fatal("click did not create a temporary track")
	}
	click.Stop()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary click track remained after shutdown: %v", err)
	}
}
