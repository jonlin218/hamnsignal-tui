package visual

import (
	"math"
	"testing"
	"time"
)

func TestMotorikClockPreservesVisualPhaseAndBoundaries(t *testing.T) {
	now := time.Unix(1234, 567000000)
	want := math.Mod(now.Sub(time.Unix(0, 0)).Seconds()/motorikQuarterSeconds, 1)
	if got := motorikPhase(now); math.Abs(got-want) > 1e-12 {
		t.Fatalf("visual phase changed during clock extraction: got %.15f want %.15f", got, want)
	}
	index := MotorikQuarterIndex(now)
	boundary := MotorikQuarterBoundary(index)
	next := NextMotorikQuarterBoundary(now)
	if boundary.After(now) || !next.After(now) || next != MotorikQuarterBoundary(index+1) {
		t.Fatalf("unexpected boundaries: current=%s next=%s now=%s", boundary, next, now)
	}
	if got, want := MotorikQuarterDuration(), time.Duration(math.Round(60.0/152.0*float64(time.Second))); got != want {
		t.Fatalf("quarter duration=%s want %s", got, want)
	}
}
