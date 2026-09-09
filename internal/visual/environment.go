package visual

import (
	"math"
	"time"
)

var riverColor = fieldColor{74, 181, 166}
var rainColor = fieldColor{157, 205, 211}

var riverLobeX = [...]float64{0.08, 0.28, 0.49, 0.71, 0.91}

const riverTopologySeed uint32 = 0x20776b93

// contributeRiver gives the observed river a low, persistent field near the
// bottom of the viewport. Its lobe positions are seeded by the measurement and
// drift only on a slow time scale.
func (p *Presenter) contributeRiver(field *densityField, now time.Time) {
	if field == nil || !p.hasRiver {
		return
	}
	level := clamp(p.currentRiver, 0, 1)
	// Level controls the river's vertical presence. Its broad horizontal
	// topology remains one recognisable environmental entity as the level
	// changes, rather than changing shape for unrelated seed reasons.
	seed := riverTopologySeed
	phase := now.Sub(time.Unix(0, 0)).Seconds() / 45
	// Sitting slightly into the lower edge grounds the river without turning it
	// into a fixed baseline. Higher levels still rise into the viewport.
	baseY := float64(field.height) * (0.95 - level*0.12)
	sy := float64(field.height) * (0.035 + level*0.06)
	// The centres cover the lower viewport, while their seeded widths and
	// strengths retain clear gaps rather than resolving into a uniform band.
	for index := 0; index < 5; index++ {
		lobeSeed := stableSeed(int64(seed) + int64(index+1)*7919)
		baseX := riverLobeX[index] * float64(field.width)
		xDrift := math.Sin(phase+float64(stableByte(lobeSeed, 0))/40)*float64(field.width)*0.012 + p.currentWindBias*float64(field.width)*0.35
		yDrift := math.Sin(phase*0.7+float64(stableByte(lobeSeed, 1))/35)*sy*0.25 + p.currentWindVertical*float64(field.height)*0.1
		sx := float64(field.width) * (0.095 + float64(stableByte(lobeSeed, 2))/255*0.04)
		strength := 0.045 + float64(stableByte(lobeSeed, 3))/255*0.020
		field.addGaussian(baseX+xDrift, baseY+yDrift, sx, sy, strength, riverColor)
	}
}

// contributeArrival uses the event's stable seed as a temporary local field
// disturbance. It never creates a horizontal rule or a free-running object.
func (p *Presenter) contributeArrival(field *densityField, now time.Time) {
	if field == nil || p.arrival == nil {
		return
	}
	fraction := now.Sub(p.arrival.start).Seconds() / p.arrival.duration.Seconds()
	if fraction < 0 || fraction >= 1 {
		return
	}
	baseX := 0.25 + float64(stableByte(p.arrival.seed, 1))/255*0.5
	travel := (fraction - 0.5) * 0.16
	y := 0.57
	sx, sy, strength := 0.035, 0.045, 0.12
	if p.arrival.mode == "ferry" {
		y, sx, sy, strength = 0.66, 0.07, 0.075, 0.1
	}
	fade := 1 - fraction
	field.addGaussian((baseX+travel)*float64(field.width), y*float64(field.height), float64(field.width)*sx*(1+fraction*0.45), float64(field.height)*sy*(1+fraction*0.35), strength*fade, fieldColor{181, 221, 220})
}

// contributeRain derives a small, bounded set of short-lived environmental
// traces directly from precipitation and time; it keeps no particle state.
func (p *Presenter) contributeRain(field *densityField, now time.Time) {
	if field == nil || p.precipitation <= 0 {
		return
	}
	intensity := clamp(p.precipitation/5, 0, 1)
	count := int(math.Ceil(intensity * 8))
	for index := 0; index < count; index++ {
		seed := stableSeed(int64(index+1)*7919 + int64(field.width*31+field.height))
		age := math.Mod(now.Sub(time.Unix(0, 0)).Seconds()/1.2+float64(stableByte(seed, 0))/255, 1)
		x := (float64(stableByte(seed, 1))/255 + p.currentWindBias*age*1.2) * float64(field.width)
		y := age * float64(field.height)
		sx := float64(field.width) * 0.004
		sy := float64(field.height) * 0.018
		strength := (0.028 + intensity*0.018) * (1 - age*0.45)
		field.addGaussian(x, y, sx, sy, strength, rainColor)
	}
}
