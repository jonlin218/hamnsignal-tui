package visual

// attenuateMotorikByRiver lets the current meet the river as a heavier local
// medium. It uses only the river's own temporary field: voices and every other
// shared contribution remain irrelevant to this resistance decision.
func attenuateMotorikByRiver(motorik *densityField, river densityField) {
	if motorik == nil || motorik.width != river.width || motorik.height != river.height {
		return
	}
	for index, value := range motorik.values {
		factor := motorikRiverResistance(river.values[index])
		motorik.values[index] = value * factor
		motorik.red[index] *= factor
		motorik.green[index] *= factor
		motorik.blue[index] *= factor
	}
}

// motorikRiverResistance is unity below the river's faint outer tail, then
// eases smoothly to a small residual contribution in the dense river body.
func motorikRiverResistance(riverDensity float64) float64 {
	const edge, body, residual = .018, .085, .12
	if riverDensity <= edge {
		return 1
	}
	if riverDensity >= body {
		return residual
	}
	t := (riverDensity - edge) / (body - edge)
	t = t * t * (3 - 2*t)
	return 1 - (1-residual)*t
}

// motorikRiverGlyphFactor starts yielding the mechanical vocabulary before
// density resistance becomes substantial, allowing the boundary to dissolve
// into ordinary shared Braille instead of forming a hard edge.
func motorikRiverGlyphFactor(riverDensity float64) float64 {
	const edge, body = .012, .065
	if riverDensity <= edge {
		return 1
	}
	if riverDensity >= body {
		return 0
	}
	t := (riverDensity - edge) / (body - edge)
	return 1 - (t * t * (3 - 2*t))
}
