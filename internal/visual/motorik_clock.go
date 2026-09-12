package visual

import (
	"math"
	"time"
)

const motorikBPM = 152.0

const motorikQuarterSeconds = 60 / motorikBPM

var motorikEpoch = time.Unix(0, 0)

// MotorikQuarterDuration is the fixed quarter-note duration shared by the
// inertial current and development-only timing references.
func MotorikQuarterDuration() time.Duration {
	return time.Duration(math.Round(motorikQuarterSeconds * float64(time.Second)))
}

// motorikPhase deliberately retains the original elapsed-time calculation so
// extracting the clock does not alter the established visual timing.
func motorikPhase(now time.Time) float64 {
	elapsed := now.Sub(motorikEpoch).Seconds()
	return math.Mod(elapsed/motorikQuarterSeconds, 1)
}

// MotorikQuarterIndex returns the absolute quarter containing now.
func MotorikQuarterIndex(now time.Time) int64 {
	return int64(math.Floor(now.Sub(motorikEpoch).Seconds() / motorikQuarterSeconds))
}

// MotorikQuarterBoundary returns the absolute boundary for quarter index.
func MotorikQuarterBoundary(index int64) time.Time {
	return motorikEpoch.Add(time.Duration(math.Round(float64(index) * motorikQuarterSeconds * float64(time.Second))))
}

// NextMotorikQuarterBoundary returns the next strictly future motorik pulse.
func NextMotorikQuarterBoundary(now time.Time) time.Time {
	return MotorikQuarterBoundary(MotorikQuarterIndex(now) + 1)
}
