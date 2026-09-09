package visual

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
)

func TestBrailleMappingAndCombiningDots(t *testing.T) {
	canvas := NewCanvas(1, 1)
	canvas.Set(0, 0)
	canvas.Set(1, 0)
	canvas.Set(0, 3)
	canvas.Set(1, 3)
	if got := canvas.Render(); got != "⣉" {
		t.Fatalf("got %q", got)
	}
	canvas.Set(-1, 0)
	canvas.Set(2, 4)
	if strings.Count(canvas.Render(), "\n") != 0 {
		t.Fatal("unexpected rows")
	}
}

func TestCanvasTinyAndZeroSafe(t *testing.T) {
	for _, canvas := range []Canvas{NewCanvas(0, 0), NewCanvas(0, 2), NewCanvas(2, 0), NewCanvas(-1, -1)} {
		canvas.Set(0, 0)
		if canvas.Render() != "" {
			t.Fatalf("non-empty tiny canvas: %q", canvas.Render())
		}
	}
}

func TestCoolPaletteIsRestrainedAndMonotonic(t *testing.T) {
	canvas := NewCanvas(2, 1)
	canvas.SetTone(0, 0, 40)
	canvas.SetTone(2, 0, 220)
	plain := canvas.Render()
	if strings.Contains(plain, "\x1b[") {
		t.Fatal("plain canvas unexpectedly contains ANSI escape codes")
	}
	coloured := canvas.render(true)
	if !strings.Contains(coloured, "\x1b[38;2;") {
		t.Fatal("coloured canvas did not emit ANSI foreground colour")
	}
	lowR, lowG, lowB := coolTone(40)
	highR, highG, highB := coolTone(220)
	if !(lowR < highR && lowG < highG && lowB < highB) || highR > 240 || highG > 250 || highB > 255 {
		t.Fatalf("unexpected palette range: low=(%d,%d,%d) high=(%d,%d,%d)", lowR, lowG, lowB, highR, highG, highB)
	}
}

func TestDensityFieldFalloffAndOverlap(t *testing.T) {
	field := newDensityField(80, 24)
	field.addGaussian(80, 48, 30, 12, 0.4, fieldColor{100, 180, 220})
	center := field.value(80, 48)
	edge := field.value(0, 48)
	if center <= edge {
		t.Fatalf("unexpected Gaussian falloff: center=%v edge=%v", center, edge)
	}
	first := center
	field.addGaussian(80, 48, 30, 12, 0.4, fieldColor{180, 120, 210})
	if field.value(80, 48) <= first {
		t.Fatal("overlapping voices did not accumulate")
	}
}

func TestDensityFieldRasterizationAndBlankBackground(t *testing.T) {
	blank := newDensityField(4, 2)
	canvas := NewCanvas(4, 2)
	blank.rasterize(&canvas)
	for _, line := range strings.Split(canvas.Render(), "\n") {
		if strings.Trim(line, "⠀") != "" {
			t.Fatalf("blank field rendered marks: %q", canvas.Render())
		}
	}
	field := newDensityField(4, 2)
	field.addGaussian(4, 4, 2, 2, 1, fieldColor{100, 180, 220})
	field.rasterize(&canvas)
	if strings.Trim(canvas.Render(), "⠀\n") == "" {
		t.Fatal("non-zero field produced no Braille occupancy")
	}
}

func TestRainIsBoundedDeterministicAndWindAware(t *testing.T) {
	now := time.Unix(300, 0)
	dry := NewPresenter()
	dry.precipitation = 0
	blank := newDensityField(200, 60)
	dry.contributeRain(&blank, now)
	if totalFieldEnergy(blank) != 0 {
		t.Fatal("dry state created rain")
	}
	light, heavy := NewPresenter(), NewPresenter()
	light.precipitation, heavy.precipitation = .5, 5
	first, repeat, strong := newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60)
	light.contributeRain(&first, now)
	light.contributeRain(&repeat, now)
	heavy.contributeRain(&strong, now)
	if totalFieldEnergy(first) == 0 || totalFieldEnergy(strong) <= totalFieldEnergy(first) {
		t.Fatal("rain did not scale")
	}
	if !reflect.DeepEqual(first, repeat) {
		t.Fatal("rain is not deterministic")
	}
	wind := NewPresenter()
	wind.precipitation, wind.currentWindBias = 5, .05
	shifted := newDensityField(200, 60)
	wind.contributeRain(&shifted, now)
	if reflect.DeepEqual(shifted, strong) {
		t.Fatal("wind did not deflect rain")
	}
	for _, size := range [][2]int{{1, 1}, {80, 24}, {120, 35}} {
		field := newDensityField(size[0], size[1])
		heavy.contributeRain(&field, now)
	}
}

func TestBusyVoiceFixtureDiagnosesEnvelopeAccumulation(t *testing.T) {
	now := time.Unix(200, 0)
	presenter := NewPresenter()
	presenter.Sync(busyVoiceSnapshot(), now)
	combined, envelopes, bodies, cores, withoutValleys := newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60)
	individualVisible := make([]bool, combined.width*combined.height)
	for _, entry := range presenter.voices {
		voice, envelope, body, core := newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60), newDensityField(200, 60)
		contributeVoicePartsForTest(presenter, &voice, &envelope, &body, &core, entry, now, true)
		contributeVoicePartsForTest(presenter, &withoutValleys, nil, nil, nil, entry, now, false)
		combined.add(voice)
		envelopes.add(envelope)
		bodies.add(body)
		cores.add(core)
		for index := range individualVisible {
			individualVisible[index] = individualVisible[index] || visibleAt(voice, index%voice.width, index/voice.width)
		}
	}
	combinedVisible := visibleCount(combined)
	weakOnly := 0
	for index := range individualVisible {
		if visibleAt(combined, index%combined.width, index/combined.width) && !individualVisible[index] {
			weakOnly++
		}
	}
	t.Logf("busy sparse-envelope fixture: combined=%d (%.1f%%), envelope=%d, body=%d, core=%d, weak-combined-only=%d (%.1f%% of combined), valley delta=%d", combinedVisible, percent(combinedVisible, len(combined.values)), visibleCount(envelopes), visibleCount(bodies), visibleCount(cores), weakOnly, percent(weakOnly, combinedVisible), visibleCount(withoutValleys)-combinedVisible)
	if weakOnly > combinedVisible*3/10 {
		t.Fatalf("weak envelope accumulation remained excessive: %d of %d", weakOnly, combinedVisible)
	}
}

func busyVoiceSnapshot() hamnsignal.StateSnapshot {
	voices := map[int64]hamnsignal.Voice{}
	for index, spec := range []struct {
		freq, pan, amp float64
		layer, source  string
	}{
		{90, -0.72, .030, "body", ""}, {115, -0.48, .026, "body", ""}, {145, -0.22, .032, "cantus", ""}, {175, 0.02, .028, "body", "counterpoint"}, {205, 0.25, .030, "body", ""}, {235, 0.48, .027, "cantus_support", ""}, {270, 0.70, .031, "body", ""}, {315, -0.60, .025, "cantus", ""}, {360, 0.12, .029, "body", ""}, {410, 0.58, .024, "cantus_support", ""},
	} {
		id := hamnsignal.Int64(index + 100)
		freq, pan, amp := hamnsignal.Number(spec.freq), hamnsignal.Number(spec.pan), hamnsignal.Number(spec.amp)
		layer, source := spec.layer, spec.source
		fields := hamnsignal.VoiceFields{NodeID: &id, Freq: &freq, Pan: &pan, Amp: &amp, Layer: &layer}
		if source != "" {
			fields.Source = &source
		}
		voices[int64(id)] = hamnsignal.Voice{Fields: fields}
	}
	return hamnsignal.StateSnapshot{Voices: voices}
}

func contributeVoicePartsForTest(p *Presenter, all, envelope, body, core *densityField, entry voiceVisual, now time.Time, valleys bool) {
	fields, profile := entry.voice.Fields, styleFor(entry.voice.Fields.Layer, entry.voice.Fields.Source)
	freq, pan, amp := 55.0, 0.0, 0.0
	if fields.Freq != nil {
		freq = float64(*fields.Freq)
	}
	if fields.Pan != nil {
		pan = float64(*fields.Pan)
	}
	if fields.Amp != nil {
		amp = float64(*fields.Amp)
	}
	brightness, stability := 0.5, 0.75
	if fields.Brightness != nil {
		brightness = clamp(float64(*fields.Brightness), 0, 1)
	}
	if fields.Stability != nil {
		stability = clamp(float64(*fields.Stability), 0, 1)
	}
	x, y := clamp(panPosition(pan)+p.currentWindBias, 0, 1)*float64(all.width-1), clamp(frequencyPosition(freq)+(0.5-p.currentRiver)*0.12, 0, 1)*float64(all.height-1)
	scale := clamp(float64(all.width/2+all.height/4)/150, .75, 3)
	sx, sy := profile.horizontalExtent*scale, profile.verticalExtent*scale*(1.15-stability*.25)
	strength := (0.006 + amplitudeEnergy(amp)*.12) * profile.density * (.9 + brightness*.1) * (1 + clamp(p.precipitation/10, 0, 1)*.04)
	color, topology := voiceColorWithBrightness(voiceColor(entry.seed), brightness, 1), topologyForVoice(profile, entry.seed, stability)
	add := func(target *densityField, lobe fieldLobe, factor float64) {
		if target != nil {
			target.addGaussian(x+lobe.offsetX*sx, y+lobe.offsetY*sy, sx*lobe.scaleX, sy*lobe.scaleY, strength*lobe.strength*factor, color)
		}
	}
	addEnvelope := func(target *densityField, lobe fieldLobe) {
		if target != nil {
			target.addEnvelopeGaussian(x+lobe.offsetX*sx, y+lobe.offsetY*sy, sx*lobe.scaleX, sy*lobe.scaleY, strength*lobe.strength, color)
		}
	}
	for _, lobe := range topology.envelopes {
		addEnvelope(all, lobe)
		addEnvelope(envelope, lobe)
	}
	for _, lobe := range topology.bodies {
		add(all, lobe, 1.7)
		add(body, lobe, 1.7)
	}
	for _, lobe := range topology.cores {
		add(all, lobe, 2.4)
		add(core, lobe, 2.4)
	}
	if valleys {
		for _, lobe := range topology.valleys {
			add(all, lobe, -1)
		}
	}
}

func visibleAt(field densityField, x, y int) bool {
	value := field.value(x, y)
	return value > .022 && value >= .035+float64((x%2+(y%4)*2)%5)*.02
}
func visibleCount(field densityField) int {
	count := 0
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			if visibleAt(field, x, y) {
				count++
			}
		}
	}
	return count
}
func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

func TestVoiceLobesAreStableAndKeepPrimaryCentre(t *testing.T) {
	body := styleFor(stringPtr("body"), nil)
	first := voiceLobes(body, stableSeed(42), 0.4)
	second := voiceLobes(body, stableSeed(42), 0.4)
	if len(first) != len(second) || len(first) < 3 || first[0].offsetX != 0 || first[0].offsetY != 0 {
		t.Fatalf("unexpected body lobe plan: %#v", first)
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatal("lobe plan is not deterministic")
		}
		if math.Abs(first[index].offsetX) > 1 || math.Abs(first[index].offsetY) > 1 {
			t.Fatalf("secondary lobe escaped voice bounds: %#v", first[index])
		}
	}
}

func TestLayerLobeTendenciesAndStability(t *testing.T) {
	body := voiceLobes(styleFor(stringPtr("body"), nil), stableSeed(1), 0.5)
	cantus := voiceLobes(styleFor(stringPtr("cantus"), nil), stableSeed(1), 0.5)
	support := voiceLobes(styleFor(stringPtr("cantus_support"), nil), stableSeed(1), 0.5)
	counterpoint := voiceLobes(styleFor(stringPtr("body"), stringPtr("counterpoint")), stableSeed(1), 0.5)
	if len(body) <= len(cantus) || len(support) != len(cantus) || len(counterpoint) <= len(cantus) {
		t.Fatalf("unexpected layer lobe counts: body=%d cantus=%d support=%d counterpoint=%d", len(body), len(cantus), len(support), len(counterpoint))
	}
	stable := voiceLobes(styleFor(stringPtr("body"), nil), stableSeed(7), 1)
	unstable := voiceLobes(styleFor(stringPtr("body"), nil), stableSeed(7), 0)
	if math.Abs(stable[1].offsetX)+math.Abs(stable[1].offsetY) >= math.Abs(unstable[1].offsetX)+math.Abs(unstable[1].offsetY) {
		t.Fatal("stability did not tighten lobe coherence")
	}
}

func TestArrivalGestureIsAContainedFieldDisturbance(t *testing.T) {
	presenter := NewPresenter()
	presenter.arrival = &arrivalGesture{start: time.Unix(60, 0), duration: 2 * time.Second, mode: "tram"}
	field := newDensityField(200, 60)
	presenter.contributeArrival(&field, time.Unix(60, 0).Add(time.Second))
	canvas := NewCanvas(200, 60)
	field.rasterize(&canvas)
	if width := nonBlankWidth(canvas.Render()); width > 24 || width == 0 {
		t.Fatalf("arrival field has invalid width: %d cells", width)
	}
}

func TestVoiceColoursAreDeterministicAndDistributed(t *testing.T) {
	if voiceColor(stableSeed(42)) != voiceColor(stableSeed(42)) {
		t.Fatal("voice colour changed for the same identity")
	}
	seen := map[fieldColor]bool{}
	for id := int64(1); id <= 12; id++ {
		seen[voiceColor(stableSeed(id))] = true
	}
	if len(seen) < 4 {
		t.Fatalf("voice identities did not distribute across palette: %d", len(seen))
	}
}

func TestBrightnessAndReleasePreserveVoiceHue(t *testing.T) {
	base := voiceColor(stableSeed(8))
	dim := voiceColorWithBrightness(base, 0, 1)
	bright := voiceColorWithBrightness(base, 1, 1)
	released := voiceColorWithBrightness(base, 1, 0.4)
	if colourLuminance(bright) <= colourLuminance(dim) || colourLuminance(released) >= colourLuminance(bright) {
		t.Fatalf("brightness/release luminance invalid: dim=%v bright=%v released=%v", dim, bright, released)
	}
	if dim.r == 0 || dim.g == 0 || dim.b == 0 {
		t.Fatal("active colour fell below visibility floor")
	}
}

func TestColouredFieldsBlendInsteadOfOverwriting(t *testing.T) {
	field := newDensityField(10, 4)
	blue, rose := fieldColor{100, 150, 220}, fieldColor{210, 130, 150}
	field.addGaussian(10, 8, 4, 3, 0.2, blue)
	field.addGaussian(10, 8, 4, 3, 0.2, rose)
	mixed := field.color(10, 8)
	if mixed.r <= blue.r || mixed.b <= rose.b || mixed == blue || mixed == rose {
		t.Fatalf("overlap did not blend colours: %v", mixed)
	}
	if field.value(10, 8) <= 0.2 {
		t.Fatal("overlap did not accumulate energy")
	}
}

func TestStrongColourOverlapLightensOnlyAtPeak(t *testing.T) {
	field := newDensityField(10, 4)
	base := fieldColor{100, 150, 220}
	field.addGaussian(10, 8, 4, 3, 0.08, base)
	faint := field.color(10, 8)
	field.addGaussian(10, 8, 4, 3, 0.8, base)
	peak := field.color(10, 8)
	if colourLuminance(peak) <= colourLuminance(faint) {
		t.Fatalf("strong overlap did not lighten: faint=%v peak=%v", faint, peak)
	}
}

func TestVoiceTopologyHasNestedCoreBodyAndEnvelope(t *testing.T) {
	profile := styleFor(stringPtr("body"), nil)
	topology := topologyForVoice(profile, stableSeed(42), 0.7)
	if len(topology.envelopes) == 0 || len(topology.bodies) == 0 || len(topology.cores) != 1 || len(topology.valleys) != 2 {
		t.Fatalf("unexpected topology: %#v", topology)
	}
	envelope, body, core := topology.envelopes[0], topology.bodies[0], topology.cores[0]
	if !(envelope.scaleX > body.scaleX && body.scaleX > core.scaleX && envelope.scaleY > body.scaleY && body.scaleY > core.scaleY) {
		t.Fatalf("topology is not nested: envelope=%#v body=%#v core=%#v", envelope, body, core)
	}
	if core.offsetX != 0 || core.offsetY != 0 {
		t.Fatalf("core left voice centre: %#v", core)
	}
}

func TestVoiceTopologyTuningStrengthensInternalSeparation(t *testing.T) {
	topology := topologyForVoice(styleFor(stringPtr("body"), nil), stableSeed(42), 0.7)
	primary := topology.envelopes[0]
	if got := topology.bodies[0].strength / primary.strength; math.Abs(got-0.52) > 1e-9 {
		t.Fatalf("primary body strength = %v, want 0.52", got)
	}
	if got := topology.cores[0].strength / primary.strength; math.Abs(got-0.37) > 1e-9 {
		t.Fatalf("primary core strength = %v, want 0.37", got)
	}
	for _, valley := range topology.valleys {
		if valley.strength < 0.07 || valley.strength > 0.10 {
			t.Fatalf("valley strength outside tuned range: %v", valley.strength)
		}
	}
}

func TestTopologyAndValleysAreStableAndNonNegative(t *testing.T) {
	profile := styleFor(stringPtr("body"), nil)
	first := topologyForVoice(profile, stableSeed(9), 0.2)
	second := topologyForVoice(profile, stableSeed(9), 0.2)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("topology changed between frames")
	}
	field := newDensityField(20, 8)
	color := voiceColor(stableSeed(9))
	field.addGaussian(20, 16, 8, 5, 0.05, color)
	field.addGaussian(20, 16, 8, 5, -1, color)
	for _, value := range field.values {
		if value < 0 {
			t.Fatalf("attenuation produced negative energy: %v", value)
		}
	}
}

func TestStabilityChangesTopologyWithoutChangingIdentity(t *testing.T) {
	profile := styleFor(stringPtr("body"), nil)
	stable := topologyForVoice(profile, stableSeed(22), 1)
	unstable := topologyForVoice(profile, stableSeed(22), 0)
	stableDistance := math.Abs(stable.valleys[0].offsetX) + math.Abs(stable.valleys[0].offsetY)
	unstableDistance := math.Abs(unstable.valleys[0].offsetX) + math.Abs(unstable.valleys[0].offsetY)
	if stableDistance >= unstableDistance {
		t.Fatal("stability did not keep topology more coherent")
	}
	if voiceColor(stableSeed(22)) != voiceColor(stableSeed(22)) {
		t.Fatal("topology change altered voice identity colour")
	}
}

func TestMappingsClampAndFrequencyIsLogarithmic(t *testing.T) {
	if frequencyPosition(55) != 1 || frequencyPosition(500) != 0 || frequencyPosition(1) != 1 || frequencyPosition(1000) != 0 {
		t.Fatal("frequency bounds not clamped")
	}
	if !(frequencyPosition(110)-frequencyPosition(220) > 0) {
		t.Fatal("frequency should rise upward")
	}
	if panPosition(-2) != 0 || panPosition(2) != 1 || panPosition(0) != 0.5 {
		t.Fatal("pan bounds not clamped")
	}
	if amplitudeEnergy(0) != 0 || amplitudeEnergy(1) != 1 || !(amplitudeEnergy(0.03) > amplitudeEnergy(0.003)) {
		t.Fatal("amplitude scaling invalid")
	}
}

func TestReleaseLifecycleEventuallyCleansUp(t *testing.T) {
	release := hamnsignal.Number(1)
	node := hamnsignal.Int64(42)
	state := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{42: {Fields: hamnsignal.VoiceFields{NodeID: &node, Freq: numberPtr(220), Amp: numberPtr(0.02), Release: &release}}}}
	presenter := NewPresenter()
	start := time.Unix(100, 0)
	if presenter.Render(state, 20, 10, start) == "" {
		t.Fatal("active voice did not render")
	}
	state.Voices[42] = hamnsignal.Voice{Released: true, Fields: state.Voices[42].Fields}
	presenter.Render(state, 20, 10, start.Add(500*time.Millisecond))
	if len(presenter.voices) != 1 {
		t.Fatal("released voice disappeared too early")
	}
	presenter.Render(state, 20, 10, start.Add(2*time.Second))
	if len(presenter.voices) != 0 {
		t.Fatal("released voice was not cleaned up")
	}
}

func TestStableVoiceCharacteristics(t *testing.T) {
	if stableSeed(12) != stableSeed(12) || stableSeed(12) == stableSeed(13) {
		t.Fatal("unstable or colliding seed")
	}
}

func TestVoiceIdentityProfilesAndCounterpointPrecedence(t *testing.T) {
	body, cantus, support, counterpoint := "body", "cantus", "cantus_support", "counterpoint"
	if styleFor(&body, nil).horizontalExtent <= styleFor(&cantus, nil).horizontalExtent {
		t.Fatal("body should be more horizontal than cantus")
	}
	if styleFor(&support, nil).density >= styleFor(&cantus, nil).density {
		t.Fatal("cantus support should be subordinate")
	}
	if styleFor(&body, &counterpoint).coherence == styleFor(&body, nil).coherence {
		t.Fatal("counterpoint source did not override body style")
	}
}

func TestStabilityAndBrightnessInfluenceVoiceTexture(t *testing.T) {
	node := hamnsignal.Int64(8)
	freq := hamnsignal.Number(220)
	amp := hamnsignal.Number(0.05)
	stable, unstable := hamnsignal.Number(1), hamnsignal.Number(0)
	bright, dim := hamnsignal.Number(1), hamnsignal.Number(0)
	base := hamnsignal.VoiceFields{NodeID: &node, Freq: &freq, Amp: &amp}
	stableState := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{8: {Fields: base}}}
	base.Stability, base.Brightness = &stable, &bright
	stableState.Voices[8] = hamnsignal.Voice{Fields: base}
	stableRender := NewPresenter().Render(stableState, 40, 15, time.Unix(10, 0))
	base.Stability, base.Brightness = &unstable, &dim
	unstableState := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{8: {Fields: base}}}
	unstableRender := NewPresenter().Render(unstableState, 40, 15, time.Unix(10, 0))
	if stableRender == unstableRender {
		t.Fatal("stability/brightness had no visible influence")
	}
}

func TestEnvironmentalInfluencesAndNeutralDefaults(t *testing.T) {
	now := time.Unix(20, 0)
	riverKey := "river_level"
	river := hamnsignal.Number(0.9)
	state := hamnsignal.StateSnapshot{River: map[string]hamnsignal.EnvironmentalValue{riverKey: {Fields: hamnsignal.EnvironmentalFields{Key: &riverKey, NormalizedValue: &river}}}}
	presenter := NewPresenter()
	presenter.Sync(state, now)
	if presenter.currentRiver != 0.9 {
		t.Fatalf("river = %v", presenter.currentRiver)
	}
	presenter.Sync(hamnsignal.StateSnapshot{}, now.Add(100*time.Millisecond))
	if presenter.targetRiver != 0.5 {
		t.Fatalf("missing river was not neutral: %v", presenter.targetRiver)
	}
	directionKey, speedKey := "wind_direction", "wind_speed"
	direction, speed := hamnsignal.Number(90), hamnsignal.Number(10)
	weather := map[string]hamnsignal.EnvironmentalValue{directionKey: {Fields: hamnsignal.EnvironmentalFields{Key: &directionKey, RawValue: &direction}}, speedKey: {Fields: hamnsignal.EnvironmentalFields{Key: &speedKey, RawValue: &speed}}}
	presenter.Sync(hamnsignal.StateSnapshot{Weather: weather}, now.Add(time.Second))
	if presenter.targetWindBias <= 0 {
		t.Fatalf("east wind did not produce positive bias: %v", presenter.targetWindBias)
	}
}

func TestRiverFieldIsAbsentWithoutObservedRiver(t *testing.T) {
	presenter := NewPresenter()
	presenter.Sync(hamnsignal.StateSnapshot{}, time.Unix(70, 0))
	field := newDensityField(80, 24)
	presenter.contributeRiver(&field, time.Unix(70, 0))
	if totalFieldEnergy(field) != 0 {
		t.Fatalf("missing river created field energy: %v", totalFieldEnergy(field))
	}
}

func TestRiverFieldIsDeterministicBottomAnchoredAndLevelSensitive(t *testing.T) {
	now := time.Unix(80, 0)
	low, high := NewPresenter(), NewPresenter()
	low.hasRiver, low.currentRiver = true, 0.1
	high.hasRiver, high.currentRiver = true, 0.9
	lowField, highField := newDensityField(120, 35), newDensityField(120, 35)
	low.contributeRiver(&lowField, now)
	high.contributeRiver(&highField, now)
	if totalFieldEnergy(lowField) == 0 || totalFieldEnergy(highField) == 0 {
		t.Fatal("river field did not render")
	}
	if fieldCentroidY(lowField) < float64(lowField.height)*0.7 {
		t.Fatalf("low river escaped its lower region: y=%v", fieldCentroidY(lowField))
	}
	if fieldCentroidY(highField) >= fieldCentroidY(lowField) {
		t.Fatalf("higher river did not rise: low=%v high=%v", fieldCentroidY(lowField), fieldCentroidY(highField))
	}
	repeat := newDensityField(120, 35)
	high.contributeRiver(&repeat, now)
	if !reflect.DeepEqual(highField, repeat) {
		t.Fatal("river topology was not deterministic")
	}
}

func TestRiverLevelMappingIsMonotonicAcrossRepresentativeLevels(t *testing.T) {
	now := time.Unix(82, 0)
	levels := []float64{0, 0.1, 0.25, 0.3, 0.5, 0.7, 0.75, 0.9, 1}
	previousTop := math.MaxInt
	baselineOccupied := -1
	for _, level := range levels {
		presenter := NewPresenter()
		presenter.hasRiver, presenter.currentRiver = true, level
		field := newDensityField(200, 60)
		presenter.contributeRiver(&field, now)
		top, bottom := fieldVerticalExtent(field, 0.045)
		depth := bottom - top + 1
		occupied := occupiedFieldColumns(field, 0.070)
		if top > previousTop {
			t.Fatalf("level %.2f did not rise monotonically: top=%d; previous top=%d", level, top, previousTop)
		}
		if bottom < field.height*9/10 {
			t.Fatalf("level %.2f lost lower-edge contact: bottom=%d", level, bottom)
		}
		if baselineOccupied < 0 {
			baselineOccupied = occupied
		} else if math.Abs(float64(occupied-baselineOccupied)) > 2 {
			t.Fatalf("level %.2f changed the horizontal footprint: %d, baseline %d", level, occupied, baselineOccupied)
		}
		t.Logf("level %.2f: top=%d (terminal row %d), bottom=%d, depth=%d, occupied columns=%d/%d", level, top, top/4, bottom, depth, occupied, field.width)
		previousTop = top
	}
}

func TestRiverDriftAndSmallViewportsRemainSubordinate(t *testing.T) {
	presenter := NewPresenter()
	presenter.hasRiver, presenter.currentRiver = true, 0.55
	start := time.Unix(85, 0)
	first, later := newDensityField(200, 60), newDensityField(200, 60)
	presenter.contributeRiver(&first, start)
	presenter.contributeRiver(&later, start.Add(10*time.Second))
	if drift := math.Abs(fieldCentroidX(later) - fieldCentroidX(first)); drift > float64(first.width)*0.03 {
		t.Fatalf("slow river drift became a shared translation: %v", drift)
	}
	for _, size := range [][2]int{{80, 24}, {120, 35}} {
		field := newDensityField(size[0], size[1])
		presenter.contributeRiver(&field, start)
		if totalFieldEnergy(field) == 0 || fieldCentroidY(field) < float64(field.height)*0.68 {
			t.Fatalf("river is unsafe or too high at %v", size)
		}
	}
}

func TestWindVectorIsBoundedAndPrecipitationRemainsLocal(t *testing.T) {
	presenter := NewPresenter()
	directionKey, speedKey := "wind_direction", "wind_speed"
	direction, speed := hamnsignal.Number(90), hamnsignal.Number(15)
	weather := map[string]hamnsignal.EnvironmentalValue{
		directionKey: {Fields: hamnsignal.EnvironmentalFields{Key: &directionKey, RawValue: &direction}},
		speedKey:     {Fields: hamnsignal.EnvironmentalFields{Key: &speedKey, RawValue: &speed}},
	}
	presenter.Sync(hamnsignal.StateSnapshot{Weather: weather}, time.Unix(90, 0))
	if presenter.currentWindBias <= 0 || math.Abs(presenter.currentWindVertical) > 0.0121 {
		t.Fatalf("unexpected bounded east wind vector: x=%v y=%v", presenter.currentWindBias, presenter.currentWindVertical)
	}
	if presenter.precipitation != 0 {
		t.Fatal("missing precipitation was not neutral")
	}
}

func TestArrivalFieldPreservesModeDurationAndFades(t *testing.T) {
	now := time.Unix(100, 0)
	tram := NewPresenter()
	tram.arrival = &arrivalGesture{start: now, duration: 1800 * time.Millisecond, mode: "tram", seed: stableSeed(1)}
	ferry := NewPresenter()
	ferry.arrival = &arrivalGesture{start: now, duration: 3200 * time.Millisecond, mode: "ferry", seed: stableSeed(1)}
	tramField, ferryField := newDensityField(120, 35), newDensityField(120, 35)
	tram.contributeArrival(&tramField, now.Add(500*time.Millisecond))
	ferry.contributeArrival(&ferryField, now.Add(500*time.Millisecond))
	if nonZeroFieldWidth(ferryField) <= nonZeroFieldWidth(tramField) {
		t.Fatal("ferry disturbance was not broader")
	}
	expired := newDensityField(120, 35)
	tram.contributeArrival(&expired, now.Add(2*time.Second))
	if totalFieldEnergy(expired) != 0 {
		t.Fatal("expired arrival remained visible")
	}
}

func TestRiverFieldIsEnvironmentalAndNotUniform(t *testing.T) {
	presenter := NewPresenter()
	presenter.hasRiver, presenter.currentRiver = true, 0.55
	field := newDensityField(120, 35)
	presenter.contributeRiver(&field, time.Unix(110, 0))
	if width := fieldWidthAbove(field, 0.045); width < field.width/2 || width >= field.width {
		t.Fatalf("river should span a substantial but incomplete range, got %d of %d", width, field.width)
	}
	if occupied := occupiedFieldColumns(field, 0.070); occupied < field.width/2 || occupied > field.width*4/5 {
		t.Fatalf("river should visibly occupy 50-80%% of the lower width, got %d of %d", occupied, field.width)
	}
	if riverLobeX[0] > 0.1 || riverLobeX[len(riverLobeX)-1] < 0.9 {
		t.Fatalf("river lobe centres are not distributed widely: %#v", riverLobeX)
	}
	colour := field.color(field.width/2, int(fieldCentroidY(field)))
	if colour.g <= colour.r || colour.g <= colour.b || colour.g < 120 {
		t.Fatalf("river colour escaped its blue-green family: %#v", colour)
	}
	for _, value := range field.values {
		if value < 0 {
			t.Fatalf("river created negative density: %v", value)
		}
	}
}

func TestWeatherMotionIsAbsentOrBoundedAndPrecipitationIsLocal(t *testing.T) {
	now := time.Unix(120, 0)
	neutral := NewPresenter()
	neutral.Sync(hamnsignal.StateSnapshot{}, now)
	if neutral.currentWindBias != 0 || neutral.currentWindVertical != 0 {
		t.Fatalf("missing weather moved environment: x=%v y=%v", neutral.currentWindBias, neutral.currentWindVertical)
	}
	zeroWind := NewPresenter()
	directionKey, speedKey := "wind_direction", "wind_speed"
	direction, speed := hamnsignal.Number(45), hamnsignal.Number(0)
	zeroWind.Sync(hamnsignal.StateSnapshot{Weather: map[string]hamnsignal.EnvironmentalValue{
		directionKey: {Fields: hamnsignal.EnvironmentalFields{RawValue: &direction}},
		speedKey:     {Fields: hamnsignal.EnvironmentalFields{RawValue: &speed}},
	}}, now)
	if zeroWind.currentWindBias != 0 || zeroWind.currentWindVertical != 0 {
		t.Fatal("zero wind deformed environment")
	}
	strongWind := NewPresenter()
	direction, speed = hamnsignal.Number(180), hamnsignal.Number(1000)
	strongWind.Sync(hamnsignal.StateSnapshot{Weather: map[string]hamnsignal.EnvironmentalValue{
		directionKey: {Fields: hamnsignal.EnvironmentalFields{RawValue: &direction}},
		speedKey:     {Fields: hamnsignal.EnvironmentalFields{RawValue: &speed}},
	}}, now)
	if math.Abs(strongWind.currentWindBias) > 0.0501 || math.Abs(strongWind.currentWindVertical) > 0.0121 || strongWind.currentWindVertical < 0 {
		t.Fatalf("wind vector was not safely clamped: x=%v y=%v", strongWind.currentWindBias, strongWind.currentWindVertical)
	}

	voice := testVoiceSnapshot()
	plain := NewPresenter().Render(voice, 80, 24, now)
	precipitation := hamnsignal.Number(10)
	voice.Weather = map[string]hamnsignal.EnvironmentalValue{"precipitation": {Fields: hamnsignal.EnvironmentalFields{RawValue: &precipitation}}}
	textured := NewPresenter().Render(voice, 80, 24, now)
	if plain == textured {
		t.Fatal("positive precipitation did not make its local, deterministic influence visible")
	}
}

func TestEnvironmentalCompositionIsAdditiveAndResizeSafe(t *testing.T) {
	now := time.Unix(130, 0)
	state := testVoiceSnapshot()
	riverKey, river := "river_level", hamnsignal.Number(0.6)
	state.River = map[string]hamnsignal.EnvironmentalValue{riverKey: {Fields: hamnsignal.EnvironmentalFields{NormalizedValue: &river}}}
	voiceOnly := NewPresenter()
	voiceOnly.Sync(testVoiceSnapshot(), now)
	voiceField := newDensityField(120, 35)
	for _, entry := range voiceOnly.voices {
		voiceOnly.contributeVoice(&voiceField, entry, now)
	}
	withEnvironment := NewPresenter()
	withEnvironment.Sync(state, now)
	combined := cloneDensityField(voiceField)
	withEnvironment.contributeRiver(&combined, now)
	if totalFieldEnergy(combined) <= totalFieldEnergy(voiceField) {
		t.Fatal("river did not add to the shared field")
	}
	for _, size := range [][2]int{{0, 0}, {1, 1}, {80, 24}, {120, 35}, {200, 60}} {
		if got := NewPresenter().Render(state, size[0], size[1], now); size[0] < 2 && got != "" {
			t.Fatalf("tiny viewport %v unexpectedly rendered %q", size, got)
		}
	}
	if state.River[riverKey].Fields.NormalizedValue == nil || *state.River[riverKey].Fields.NormalizedValue != river {
		t.Fatal("rendering mutated the central state")
	}
}

func TestArrivalPlacementIsDeterministicAndDoesNotCreateALine(t *testing.T) {
	now := time.Unix(140, 0)
	presenter := NewPresenter()
	presenter.arrival = &arrivalGesture{start: now, duration: 1800 * time.Millisecond, mode: "tram", seed: stableSeed(55)}
	first, second := newDensityField(120, 35), newDensityField(120, 35)
	presenter.contributeArrival(&first, now.Add(400*time.Millisecond))
	presenter.contributeArrival(&second, now.Add(400*time.Millisecond))
	if !reflect.DeepEqual(first, second) {
		t.Fatal("arrival placement changed for the same event and time")
	}
	if width := fieldWidthAbove(first, 0.03); width == 0 || width >= first.width/2 {
		t.Fatalf("tram disturbance is not sufficiently local: %d", width)
	}
	for _, value := range first.values {
		if value < 0 {
			t.Fatalf("arrival created negative density: %v", value)
		}
	}
}

func testVoiceSnapshot() hamnsignal.StateSnapshot {
	node, freq, amp := hamnsignal.Int64(99), hamnsignal.Number(220), hamnsignal.Number(0.04)
	return hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{99: {Fields: hamnsignal.VoiceFields{NodeID: &node, Freq: &freq, Amp: &amp}}}}
}

func totalFieldEnergy(field densityField) float64 {
	total := 0.0
	for _, value := range field.values {
		total += value
	}
	return total
}

func fieldCentroidY(field densityField) float64 {
	total, weighted := 0.0, 0.0
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			value := field.value(x, y)
			total += value
			weighted += float64(y) * value
		}
	}
	return weighted / total
}

func fieldCentroidX(field densityField) float64 {
	total, weighted := 0.0, 0.0
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			value := field.value(x, y)
			total += value
			weighted += float64(x) * value
		}
	}
	return weighted / total
}

func nonZeroFieldWidth(field densityField) int {
	min, max := field.width, -1
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			if field.value(x, y) > 0.001 {
				min, max = minInt(min, x), maxInt(max, x)
			}
		}
	}
	if max < min {
		return 0
	}
	return max - min + 1
}

func fieldWidthAbove(field densityField, threshold float64) int {
	min, max := field.width, -1
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			if field.value(x, y) > threshold {
				min, max = minInt(min, x), maxInt(max, x)
			}
		}
	}
	if max < min {
		return 0
	}
	return max - min + 1
}

func occupiedFieldColumns(field densityField, threshold float64) int {
	occupied := 0
	for x := 0; x < field.width; x++ {
		for y := 0; y < field.height; y++ {
			if field.value(x, y) > threshold {
				occupied++
				break
			}
		}
	}
	return occupied
}

func fieldVerticalExtent(field densityField, threshold float64) (top, bottom int) {
	top, bottom = field.height, -1
	for y := 0; y < field.height; y++ {
		for x := 0; x < field.width; x++ {
			if field.value(x, y) > threshold {
				top, bottom = minInt(top, y), maxInt(bottom, y)
			}
		}
	}
	return top, bottom
}

func cloneDensityField(field densityField) densityField {
	clone := newDensityField(field.width/2, field.height/4)
	copy(clone.values, field.values)
	copy(clone.red, field.red)
	copy(clone.green, field.green)
	copy(clone.blue, field.blue)
	return clone
}

func TestPrecipitationBelongsToVoicesAndArrivalExpires(t *testing.T) {
	precipKey := "precipitation"
	precipitation := hamnsignal.Number(10)
	weather := map[string]hamnsignal.EnvironmentalValue{precipKey: {Fields: hamnsignal.EnvironmentalFields{Key: &precipKey, RawValue: &precipitation}}}
	noVoice := hamnsignal.StateSnapshot{Weather: weather}
	presenter := NewPresenter()
	blank := presenter.Render(noVoice, 30, 10, time.Unix(30, 0))
	if strings.Trim(blank, "⠀\n") == "" {
		t.Fatal("positive precipitation did not create rain without a voice")
	}
	node, mode, line := hamnsignal.Int64(10), "tram", "3"
	site := "central"
	freq, amp := hamnsignal.Number(220), hamnsignal.Number(0.03)
	voice := hamnsignal.Voice{Fields: hamnsignal.VoiceFields{NodeID: &node, Freq: &freq, Amp: &amp}}
	arrival := &hamnsignal.Arrival{At: 1, Fields: hamnsignal.ArrivalFields{Mode: &mode, Line: &line, Site: &site}}
	withVoice := hamnsignal.StateSnapshot{Weather: weather, Voices: map[int64]hamnsignal.Voice{10: voice}, LastArrival: arrival}
	start := time.Unix(30, 0)
	rendered := presenter.Render(withVoice, 30, 10, start)
	if strings.Trim(rendered, "⠀\n") == "" || presenter.arrival == nil {
		t.Fatal("voice or arrival did not render")
	}
	presenter.Sync(withVoice, start.Add(2*time.Second))
	if presenter.arrival != nil {
		t.Fatal("tram arrival did not expire")
	}
	mode = "ferry"
	arrival.At = 2
	presenter.Sync(withVoice, start.Add(3*time.Second))
	if presenter.arrival == nil || presenter.arrival.duration <= 2*time.Second {
		t.Fatal("ferry gesture was not slower")
	}
}

func TestLargeCanvasIncreasesVoiceEnvelopeWithoutChangingCause(t *testing.T) {
	node, freq, amp := hamnsignal.Int64(77), hamnsignal.Number(220), hamnsignal.Number(0.04)
	state := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{77: {Fields: hamnsignal.VoiceFields{NodeID: &node, Freq: &freq, Amp: &amp}}}}
	now := time.Unix(50, 0)
	small := NewPresenter().Render(state, 40, 15, now)
	large := NewPresenter().Render(state, 200, 60, now)
	if nonBlankWidth(large) <= nonBlankWidth(small) {
		t.Fatalf("large envelope did not grow: small=%d large=%d", nonBlankWidth(small), nonBlankWidth(large))
	}
}

func nonBlankWidth(rendered string) int {
	min, max := 1<<30, -1
	for _, line := range strings.Split(rendered, "\n") {
		for index, character := range []rune(line) {
			if character != '⠀' {
				if index < min {
					min = index
				}
				if index > max {
					max = index
				}
			}
		}
	}
	if max < min {
		return 0
	}
	return max - min + 1
}

func numberPtr(value hamnsignal.Number) *hamnsignal.Number { return &value }
func stringPtr(value string) *string                       { return &value }

func colourLuminance(color fieldColor) float64 {
	return 0.2126*float64(color.r) + 0.7152*float64(color.g) + 0.0722*float64(color.b)
}
