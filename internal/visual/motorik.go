package visual

import (
	"math"
	"time"
)

var motorikColor = fieldColor{104, 132, 148}

// contributeMotorik adds the traffic-associated inertial current to the same
// density field as voices and environmental contributions. Pressure controls
// only this visual interpretation's spatial presence; it never changes tempo.
func (p *Presenter) contributeMotorik(field *densityField, now time.Time) {
	if field == nil || p.motorikPressure <= 0 {
		return
	}
	pressure := clamp(p.motorikPressure, 0, 1)
	phase := motorikPhase(now)
	// Fast attack, longer decay: a recurrent push within a continuously moving
	// current rather than a beat-synchronous on/off flash.
	impulse := math.Exp(-phase * 3.4)
	interval := 0.28
	travel := math.Mod(now.Sub(time.Unix(0, 0)).Seconds()/motorikQuarterSeconds*interval, 1)
	fragments := motorikFragmentCount(pressure)
	for index := 0; index < fragments; index++ {
		seed := stableSeed(int64(index+1)*4789 + int64(field.width*17+field.height))
		xNorm := wrap01(0.07 + float64(index)*interval + travel + impulse*0.012)
		yNorm := 0.51 + float64(index%3)*0.075 + (float64(stableByte(seed, 0))/255-0.5)*0.018
		sx := float64(field.width) * (0.045 + pressure*0.022)
		sy := float64(field.height) * (0.018 + pressure*0.008)
		strength := (0.038 + pressure*0.020) * (0.80 + impulse*0.20)
		field.addGaussian(xNorm*float64(field.width-1), yNorm*float64(field.height-1), sx, sy, strength, motorikColor)
	}
}

func motorikFragmentCount(pressure float64) int {
	return 2 + int(math.Ceil(clamp(pressure, 0, 1)*3))
}

func wrap01(value float64) float64 {
	return value - math.Floor(value)
}
