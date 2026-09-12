package visual

import "time"

var radiationColor = fieldColor{178, 168, 146}

var radiationLobes = [...]struct {
	x, y, width, depth, strength float64
}{
	{.14, .015, .17, .92, .90},
	{.46, .035, .23, 1.08, 1.00},
	{.81, .010, .14, .78, .82},
}

// contributeRadiation adds a shallow, static upper atmospheric veil. Its
// geometry is intentionally unlike the river: three incomplete, airy lobes
// with a soft lower edge rather than a broad lower mass.
func (p *Presenter) contributeRadiation(field *densityField, _ time.Time) {
	if field == nil || p.radiation <= 0 {
		return
	}
	radiation := clamp(p.radiation, 0, 1)
	depth := float64(field.height) * (.018 + radiation*.045)
	strength := .026 + radiation*.045
	bodyWeight := .62
	bodyY := .030 + radiation*.030
	for _, lobe := range radiationLobes {
		// A narrower cap preserves soft, deterministic breaks at the extreme
		// upper edge. The broader body begins slightly lower, so the three
		// atmospheric masses may still blend beneath that edge.
		field.addGaussian(
			lobe.x*float64(field.width),
			(-.012+lobe.y*.45)*float64(field.height),
			lobe.width*.45*float64(field.width),
			depth*lobe.depth*.48,
			strength*lobe.strength*.78,
			radiationColor,
		)
		field.addGaussian(
			lobe.x*float64(field.width),
			(bodyY+lobe.y)*float64(field.height),
			lobe.width*float64(field.width),
			depth*lobe.depth*.62,
			strength*lobe.strength*bodyWeight,
			radiationColor,
		)
	}
}
