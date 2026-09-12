package visual

import (
	"math"
	"sort"
	"time"
)

const (
	motorikGlyphTrailing = '╴'
	motorikGlyphLeading  = '╶'
)

type motorikGlyphCandidate struct {
	x, y  int
	score float64
	glyph rune
}

// resolveMotorikGlyphs selectively replaces ordinary Braille cells with a
// small mechanical vocabulary. The shared field has already been rasterized;
// this only changes a cell's glyph, never its resolved colour or tone.
func (p *Presenter) resolveMotorikGlyphs(canvas *Canvas, motorik, shared, river densityField, now time.Time) {
	if canvas == nil || p.motorikPressure <= 0 || motorik.width != shared.width || motorik.height != shared.height || motorik.width != river.width || motorik.height != river.height {
		return
	}

	pressure := clamp(p.motorikPressure, 0, 1)
	impulse := math.Exp(-motorikPhase(now) * 3.4)
	candidates := make([]motorikGlyphCandidate, 0)
	visibleMotorik := 0

	for y := 0; y < canvas.Height; y++ {
		for x := 0; x < canvas.Width; x++ {
			index := y*canvas.Width + x
			if canvas.dots[index] == 0 {
				continue
			}

			current := motorikCellDensity(motorik, x, y)
			if current < 0.009 {
				continue
			}
			visibleMotorik++

			left := motorikCellDensity(motorik, x-1, y)
			right := motorikCellDensity(motorik, x+1, y)
			support := (left + right) / 2
			coherence := clamp((support/(current+0.001)-0.30)/0.55, 0, 1)
			share := clamp(current/(motorikCellDensity(shared, x, y)+0.001), 0, 1)
			density := clamp((current-0.010)/0.018, 0, 1)
			phaseWeight := 0.42 + impulse*0.58
			riverFactor := motorikRiverGlyphFactor(motorikCellDensity(river, x, y))
			score := (density*0.50 + coherence*0.25 + share*0.25) * phaseWeight * (0.72 + pressure*0.28) * riverFactor
			if coherence < 0.35 || share < 0.62 || score < 0.60 {
				continue
			}

			glyph := motorikGlyphLeading
			if left > right {
				glyph = motorikGlyphTrailing
			}
			candidates = append(candidates, motorikGlyphCandidate{x: x, y: y, score: score, glyph: glyph})
		}
	}

	if visibleMotorik == 0 {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].y != candidates[j].y {
			return candidates[i].y < candidates[j].y
		}
		return candidates[i].x < candidates[j].x
	})

	limit := minInt(1+int(math.Floor(pressure*8)), int(math.Floor(float64(visibleMotorik)*0.12)))
	if limit <= 0 {
		return
	}
	if len(candidates) < limit {
		limit = len(candidates)
	}
	for _, candidate := range candidates[:limit] {
		canvas.setGlyphOverride(candidate.x, candidate.y, candidate.glyph)
	}
}

func motorikCellDensity(field densityField, cellX, cellY int) float64 {
	if cellX < 0 || cellY < 0 || cellX >= field.width/2 || cellY >= field.height/4 {
		return 0
	}

	total := 0.0
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < 2; dx++ {
			total += field.values[(cellY*4+dy)*field.width+cellX*2+dx]
		}
	}
	return total / 8
}
