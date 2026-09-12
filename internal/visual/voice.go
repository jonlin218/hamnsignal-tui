package visual

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
)

const (
	minFrequency = 55.0
	maxFrequency = 500.0
)

type voiceVisual struct {
	voice        hamnsignal.Voice
	seed         uint32
	releaseStart time.Time
	releasing    bool
}
type voiceProfile struct{ horizontalExtent, verticalExtent, density, coherence float64 }
type arrivalGesture struct {
	start    time.Time
	duration time.Duration
	mode     string
	seed     uint32
}

// Presenter is UI-owned derived state. It never mutates hamnsignal.State.
type Presenter struct {
	voices                          map[int64]voiceVisual
	lastSync                        time.Time
	currentRiver, targetRiver       float64
	currentWindBias, targetWindBias float64
	currentWindVertical             float64
	targetWindVertical              float64
	hasRiver                        bool
	precipitation                   float64
	motorikPressure                 float64
	fixture                         VisualFixture
	lastArrivalAt                   int64
	arrival                         *arrivalGesture
}

func NewPresenter() *Presenter {
	return &Presenter{voices: map[int64]voiceVisual{}, currentRiver: 0.5, targetRiver: 0.5}
}

// SetFixture applies development-only visual overrides without modifying the
// live Hamnsignal state or the snapshot supplied to the presenter.
func (p *Presenter) SetFixture(fixture VisualFixture) { p.fixture = fixture.clone() }

// Sync consumes an application snapshot and updates voice lifecycle timing and
// slowly changing environmental parameters.
func (p *Presenter) Sync(snapshot hamnsignal.StateSnapshot, now time.Time) {
	if p.voices == nil {
		p.voices = map[int64]voiceVisual{}
	}
	p.updateEnvironment(snapshot, now)
	p.updateArrival(snapshot, now)
	seen := make(map[int64]bool, len(snapshot.Voices))
	for id, voice := range snapshot.Voices {
		seen[id] = true
		entry, exists := p.voices[id]
		if !exists || (entry.releasing && !voice.Released) {
			entry = voiceVisual{voice: voice, seed: stableSeed(id)}
		} else {
			entry.voice = voice
		}
		if voice.Released {
			if !entry.releasing {
				entry.releaseStart, entry.releasing = now, true
			}
			if now.Sub(entry.releaseStart) >= releaseDuration(voice) {
				delete(p.voices, id)
				continue
			}
		} else {
			entry.releasing = false
			entry.releaseStart = time.Time{}
		}
		p.voices[id] = entry
	}
	for id := range p.voices {
		if !seen[id] {
			delete(p.voices, id)
		}
	}
	if p.arrival != nil && now.Sub(p.arrival.start) >= p.arrival.duration {
		p.arrival = nil
	}
}

// Render draws current derived voices and any current arrival gesture onto a
// fresh canvas. Every dot is caused by a voice or a real arrival event.
func (p *Presenter) Render(snapshot hamnsignal.StateSnapshot, width, height int, now time.Time) string {
	return p.render(snapshot, width, height, now, false)
}

// RenderANSI uses the restrained cool palette when the terminal supports it.
// Geometry and density are identical to Render.
func (p *Presenter) RenderANSI(snapshot hamnsignal.StateSnapshot, width, height int, now time.Time) string {
	return p.render(snapshot, width, height, now, true)
}

func (p *Presenter) render(snapshot hamnsignal.StateSnapshot, width, height int, now time.Time, colour bool) string {
	if width < 2 || height < 1 {
		return ""
	}
	p.Sync(snapshot, now)
	canvas := NewCanvas(width, height)
	field := newDensityField(width, height)
	voiceField := newDensityField(width, height)
	riverField := newDensityField(width, height)
	motorikField := newDensityField(width, height)
	ids := make([]int64, 0, len(p.voices))
	for id := range p.voices {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		voiceField.clear()
		p.contributeVoice(&voiceField, p.voices[id], now)
		field.add(voiceField)
	}
	p.contributeRiver(&riverField, now)
	field.add(riverField)
	p.contributeRain(&field, now)
	p.contributeArrival(&field, now)
	p.contributeMotorik(&motorikField, now)
	attenuateMotorikByRiver(&motorikField, riverField)
	field.add(motorikField)
	field.rasterize(&canvas)
	p.resolveMotorikGlyphs(&canvas, motorikField, field, riverField, now)
	if colour {
		return canvas.RenderANSI()
	}
	return canvas.Render()
}

func (p *Presenter) contributeVoice(field *densityField, entry voiceVisual, now time.Time) {
	fields := entry.voice.Fields
	freq := minFrequency
	if fields.Freq != nil {
		freq = float64(*fields.Freq)
	}
	yNorm := clamp(frequencyPosition(freq)+(0.5-p.currentRiver)*0.12, 0, 1)
	pan := 0.0
	if fields.Pan != nil {
		pan = float64(*fields.Pan)
	}
	xNorm := clamp(panPosition(pan)+p.currentWindBias, 0, 1)
	x := xNorm * float64(field.width-1)
	y := yNorm * float64(field.height-1)
	amp := 0.0
	if fields.Amp != nil {
		amp = float64(*fields.Amp)
	}
	energy := amplitudeEnergy(amp)
	brightness := 0.5
	if fields.Brightness != nil {
		brightness = clamp(float64(*fields.Brightness), 0, 1)
	}
	stability := 0.75
	if fields.Stability != nil {
		stability = clamp(float64(*fields.Stability), 0, 1)
	}
	profile := styleFor(fields.Layer, fields.Source)
	releaseFraction := 1.0
	if entry.releasing {
		releaseFraction = 1 - now.Sub(entry.releaseStart).Seconds()/releaseDuration(entry.voice).Seconds()
		if releaseFraction < 0 {
			releaseFraction = 0
		}
	}
	if releaseFraction <= 0 {
		return
	}
	scale := clamp(float64(field.width/2+field.height/4)/150, 0.75, 3.0)
	sx := profile.horizontalExtent * scale
	sy := profile.verticalExtent * scale * (1.15 - stability*0.25)
	// Quiet voices still occupy space, while amplitude determines the field
	// strength. Precipitation adds only a restrained edge texture modulation.
	strength := (0.006 + energy*0.12) * profile.density * releaseFraction
	strength *= 0.9 + brightness*0.1
	strength *= 1 + clamp(p.precipitation/10, 0, 1)*0.04
	color := voiceColorWithBrightness(voiceColor(entry.seed), brightness, releaseFraction)
	topology := topologyForVoice(profile, entry.seed, stability)
	for _, lobe := range topology.envelopes {
		field.addEnvelopeGaussian(
			x+lobe.offsetX*sx,
			y+lobe.offsetY*sy,
			sx*lobe.scaleX,
			sy*lobe.scaleY,
			strength*lobe.strength,
			color,
		)
	}
	for _, lobe := range topology.bodies {
		field.addGaussian(x+lobe.offsetX*sx, y+lobe.offsetY*sy, sx*lobe.scaleX, sy*lobe.scaleY, strength*lobe.strength*1.7, color)
	}
	for _, lobe := range topology.cores {
		field.addGaussian(x+lobe.offsetX*sx, y+lobe.offsetY*sy, sx*lobe.scaleX, sy*lobe.scaleY, strength*lobe.strength*2.4, color)
	}
	for _, valley := range topology.valleys {
		field.addGaussian(x+valley.offsetX*sx, y+valley.offsetY*sy, sx*valley.scaleX, sy*valley.scaleY, -strength*valley.strength, color)
	}
}

func styleFor(layer, source *string) voiceProfile {
	if source != nil && strings.EqualFold(*source, "counterpoint") {
		return voiceProfile{28, 10, 0.9, 0.55}
	}
	if layer == nil {
		return voiceProfile{18, 7, 0.8, 0.7}
	}
	switch strings.ToLower(*layer) {
	case "cantus":
		return voiceProfile{20, 15, 1.05, 0.9}
	case "cantus_support":
		return voiceProfile{14, 7, 0.6, 0.8}
	default:
		return voiceProfile{40, 11, 0.9, 0.85}
	}
}

func (p *Presenter) updateEnvironment(snapshot hamnsignal.StateSnapshot, now time.Time) {
	river := 0.5
	p.hasRiver = false
	if value, ok := snapshot.River["river_level"]; ok && value.Fields.NormalizedValue != nil {
		river = clamp(float64(*value.Fields.NormalizedValue), 0, 1)
		p.hasRiver = true
	}
	windBias := 0.0
	windVertical := 0.0
	if direction, ok := snapshot.Weather["wind_direction"]; ok && direction.Fields.RawValue != nil {
		speed := 0.0
		if value, exists := snapshot.Weather["wind_speed"]; exists && value.Fields.RawValue != nil {
			speed = clamp(float64(*value.Fields.RawValue)/15, 0, 1)
		}
		angle := float64(*direction.Fields.RawValue) * math.Pi / 180
		windBias = math.Sin(angle) * speed * 0.05
		windVertical = -math.Cos(angle) * speed * 0.012
	}
	p.targetRiver, p.targetWindBias, p.targetWindVertical = river, windBias, windVertical
	if value, ok := snapshot.Weather["precipitation"]; ok && value.Fields.RawValue != nil {
		p.precipitation = clamp(float64(*value.Fields.RawValue), 0, 10)
	} else {
		p.precipitation = 0
	}
	if p.fixture.Precipitation != nil {
		p.precipitation = clamp(*p.fixture.Precipitation, 0, 10)
	}
	p.motorikPressure = 0
	if value, ok := snapshot.Traffic["e45_queue"]; ok && value.Fields.NormalizedValue != nil {
		p.motorikPressure = clamp(float64(*value.Fields.NormalizedValue), 0, 1)
	}
	if p.fixture.TrafficPressure != nil {
		p.motorikPressure = clamp(*p.fixture.TrafficPressure, 0, 1)
	}
	if p.lastSync.IsZero() {
		p.currentRiver, p.currentWindBias, p.currentWindVertical = p.targetRiver, p.targetWindBias, p.targetWindVertical
	} else {
		factor := clamp(now.Sub(p.lastSync).Seconds()*2, 0, 1)
		p.currentRiver += (p.targetRiver - p.currentRiver) * factor
		p.currentWindBias += (p.targetWindBias - p.currentWindBias) * factor
		p.currentWindVertical += (p.targetWindVertical - p.currentWindVertical) * factor
	}
	p.lastSync = now
}

func (p *Presenter) updateArrival(snapshot hamnsignal.StateSnapshot, now time.Time) {
	if snapshot.LastArrival == nil || snapshot.LastArrival.At == 0 || snapshot.LastArrival.At == p.lastArrivalAt {
		return
	}
	p.lastArrivalAt = snapshot.LastArrival.At
	mode := ""
	if snapshot.LastArrival.Fields.Mode != nil {
		mode = strings.ToLower(*snapshot.LastArrival.Fields.Mode)
	}
	duration := 1800 * time.Millisecond
	if mode == "ferry" {
		duration = 3200 * time.Millisecond
	}
	p.arrival = &arrivalGesture{start: now, duration: duration, mode: mode, seed: stableSeed(snapshot.LastArrival.At)}
}

// frequencyPosition maps 55–500 Hz logarithmically to bottom–top.
func frequencyPosition(freq float64) float64 {
	if freq < minFrequency {
		freq = minFrequency
	}
	if freq > maxFrequency {
		freq = maxFrequency
	}
	return 1 - math.Log(freq/minFrequency)/math.Log(maxFrequency/minFrequency)
}
func panPosition(pan float64) float64 { return (clamp(pan, -1, 1) + 1) / 2 }

// Amplitude is observed around 0.001–0.1 on the live stream, so logarithmic scaling makes quiet voices delicate.
func amplitudeEnergy(amplitude float64) float64 {
	if amplitude <= 0.001 {
		return 0
	}
	if amplitude >= 0.1 {
		return 1
	}
	return (math.Log10(amplitude) - math.Log10(0.001)) / (math.Log10(0.1) - math.Log10(0.001))
}
func releaseDuration(voice hamnsignal.Voice) time.Duration {
	seconds := 2.0
	if voice.Fields.Release != nil {
		seconds = float64(*voice.Fields.Release)
	}
	if seconds < 0.25 {
		seconds = 0.25
	}
	if seconds > 20 {
		seconds = 20
	}
	return time.Duration(seconds * float64(time.Second))
}
func stableSeed(id int64) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte{byte(id), byte(id >> 8), byte(id >> 16), byte(id >> 24), byte(id >> 32), byte(id >> 40), byte(id >> 48), byte(id >> 56)})
	return h.Sum32()
}
func stableByte(seed uint32, index int) uint8 { return uint8((seed >> uint((index%4)*8)) & 0xff) }
func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
