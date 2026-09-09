package visual

import "math"

// densityField is the virtual 2x4-per-cell raster used by VISUAL. Values are
// deliberately accumulated before rasterization so nearby voices reinforce
// one another instead of being normalized independently.
type densityField struct {
	width, height    int
	values           []float64
	red, green, blue []float64
}

type fieldColor struct{ r, g, b uint8 }

// voicePalette is intentionally finite and restrained. Hue identifies a
// voice, never a musical parameter or layer.
var voicePalette = []fieldColor{
	{96, 206, 210},  // turquoise cyan
	{116, 158, 225}, // blue
	{165, 133, 216}, // violet
	{190, 130, 193}, // purple
	{214, 139, 158}, // dusty rose
	{205, 129, 120}, // muted red
	{224, 174, 102}, // warm amber
	{211, 204, 105}, // yellow
	{155, 205, 113}, // yellow-green
	{95, 194, 165},  // cool green
}

func voiceColor(seed uint32) fieldColor {
	return voicePalette[int(seed%uint32(len(voicePalette)))]
}

func voiceColorWithBrightness(base fieldColor, brightness, release float64) fieldColor {
	clarity := 0.78 + clamp(brightness, 0, 1)*0.22
	clarity *= 0.78 + clamp(release, 0, 1)*0.22
	return fieldColor{
		r: uint8(math.Round(float64(base.r) * clarity)),
		g: uint8(math.Round(float64(base.g) * clarity)),
		b: uint8(math.Round(float64(base.b) * clarity)),
	}
}

type fieldLobe struct {
	offsetX, offsetY float64
	scaleX, scaleY   float64
	strength         float64
}

type voiceTopology struct {
	envelopes, bodies, cores, valleys []fieldLobe
}

// topology retains the existing lobe plan as each voice's broad envelope,
// then adds nested field components at the same stable locations. The core is
// still a Gaussian field, never a separately drawn marker.
func topologyForVoice(profile voiceProfile, seed uint32, stability float64) voiceTopology {
	lobes := voiceLobes(profile, seed, stability)
	topology := voiceTopology{envelopes: lobes}
	for index, lobe := range lobes {
		bodyStrength := 0.16
		if index == 0 {
			bodyStrength = 0.52
		}
		topology.bodies = append(topology.bodies, fieldLobe{
			offsetX: lobe.offsetX, offsetY: lobe.offsetY,
			scaleX: lobe.scaleX * 0.58, scaleY: lobe.scaleY * 0.58,
			strength: lobe.strength * bodyStrength,
		})
		if index == 0 {
			topology.cores = append(topology.cores, fieldLobe{
				scaleX: lobe.scaleX * 0.24, scaleY: lobe.scaleY * 0.24,
				strength: lobe.strength * 0.37,
			})
		}
	}
	spread := 0.12 + (1-clamp(stability, 0, 1))*0.22
	valleyCount := 1
	if profile.horizontalExtent >= 30 || profile.coherence < 0.6 {
		valleyCount = 2
	}
	for index := 0; index < valleyCount; index++ {
		x := (float64(stableByte(seed>>12, index+31)) - 127.5) / 127.5 * spread
		y := (float64(stableByte(seed>>20, index+37)) - 127.5) / 127.5 * spread
		topology.valleys = append(topology.valleys, fieldLobe{
			offsetX: x, offsetY: y,
			scaleX:   0.28 + float64(stableByte(seed, index+41))/255*0.16,
			scaleY:   0.3 + float64(stableByte(seed>>8, index+43))/255*0.16,
			strength: 0.07 + float64(stableByte(seed>>16, index+47))/255*0.03,
		})
	}
	return topology
}

// voiceLobes returns a small stable constellation of overlapping envelopes.
// The primary lobe remains centred on pan/frequency; secondary lobes only
// perturb the silhouette around it. Stability narrows their displacement.
func voiceLobes(profile voiceProfile, seed uint32, stability float64) []fieldLobe {
	stability = clamp(stability, 0, 1)
	spread := 0.16 + (1-stability)*0.42
	count, character := 2, "neutral"
	switch {
	case profile.coherence < 0.6:
		count, character = 3, "counterpoint"
	case profile.horizontalExtent >= 30:
		count, character = 4, "body"
	case profile.verticalExtent > profile.horizontalExtent*0.55:
		count, character = 2, "cantus"
	case profile.density < 0.7:
		count, character = 2, "support"
	}
	lobes := []fieldLobe{{scaleX: 0.86, scaleY: 0.86, strength: 0.52}}
	for index := 1; index < count; index++ {
		x := (float64(stableByte(seed, index)) - 127.5) / 127.5 * spread
		y := (float64(stableByte(seed>>8, index+7)) - 127.5) / 127.5 * spread
		scaleX := 0.46 + float64(stableByte(seed>>16, index+13))/255*0.28
		scaleY := 0.48 + float64(stableByte(seed>>24, index+19))/255*0.3
		strength := 0.1 + float64(stableByte(seed^0x9e3779b9, index+23))/255*0.1
		switch character {
		case "body":
			x *= 1.45
			y *= 0.7
		case "cantus":
			x *= 0.65
			y *= 1.35
		case "support":
			x *= 0.75
			y *= 0.75
			strength *= 0.72
		case "counterpoint":
			x *= 1.15
			y = x*0.45 + y*0.45
		}
		lobes = append(lobes, fieldLobe{offsetX: x, offsetY: y, scaleX: scaleX, scaleY: scaleY, strength: strength})
	}
	return lobes
}

func newDensityField(width, height int) densityField {
	size := width * height * 8
	return densityField{
		width: width * 2, height: height * 4, values: make([]float64, size),
		red: make([]float64, size), green: make([]float64, size), blue: make([]float64, size),
	}
}

func (f *densityField) clear() {
	if f == nil {
		return
	}
	for index := range f.values {
		f.values[index], f.red[index], f.green[index], f.blue[index] = 0, 0, 0, 0
	}
}

func (f *densityField) add(other densityField) {
	if f == nil || f.width != other.width || f.height != other.height {
		return
	}
	for index := range f.values {
		f.values[index] += other.values[index]
		f.red[index] += other.red[index]
		f.green[index] += other.green[index]
		f.blue[index] += other.blue[index]
	}
}

func (f *densityField) addGaussian(cx, cy, sx, sy, strength float64, color fieldColor) {
	f.addGaussianWithTailResponse(cx, cy, sx, sy, strength, color, false)
}

// addEnvelopeGaussian preserves an envelope's reach while making its faintest
// per-voice tail less able to form a continuous sheet through cross-voice
// accumulation. Above the normal first visibility threshold it is unchanged.
func (f *densityField) addEnvelopeGaussian(cx, cy, sx, sy, strength float64, color fieldColor) {
	f.addGaussianWithTailResponse(cx, cy, sx, sy, strength, color, true)
}

func (f *densityField) addGaussianWithTailResponse(cx, cy, sx, sy, strength float64, color fieldColor, sparseTail bool) {
	if f == nil || sx <= 0 || sy <= 0 || strength == 0 {
		return
	}
	minX := maxInt(0, int(math.Floor(cx-3*sx)))
	maxX := minInt(f.width-1, int(math.Ceil(cx+3*sx)))
	minY := maxInt(0, int(math.Floor(cy-3*sy)))
	maxY := minInt(f.height-1, int(math.Ceil(cy+3*sy)))
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			dx := (float64(px) - cx) / sx
			dy := (float64(py) - cy) / sy
			energy := strength * math.Exp(-0.5*(dx*dx+dy*dy))
			if sparseTail && energy < 0.035 {
				energy *= energy / 0.035
			}
			index := py*f.width + px
			next := math.Max(0, f.values[index]+energy)
			delta := next - f.values[index]
			f.values[index] = next
			f.red[index] = math.Max(0, f.red[index]+float64(color.r)*delta)
			f.green[index] = math.Max(0, f.green[index]+float64(color.g)*delta)
			f.blue[index] = math.Max(0, f.blue[index]+float64(color.b)*delta)
		}
	}
}

func (f densityField) color(x, y int) fieldColor {
	if x < 0 || y < 0 || x >= f.width || y >= f.height {
		return fieldColor{}
	}
	index := y*f.width + x
	energy := f.values[index]
	if energy <= 0 {
		return fieldColor{}
	}
	base := fieldColor{
		r: uint8(math.Round(f.red[index] / energy)),
		g: uint8(math.Round(f.green[index] / energy)),
		b: uint8(math.Round(f.blue[index] / energy)),
	}
	// Genuine high-energy overlap can move toward pale light, but a normal
	// voice cloud keeps its assigned hue.
	peak := clamp((energy-0.42)/1.1, 0, 0.3)
	return blendColor(base, fieldColor{228, 238, 242}, peak)
}

func blendColor(from, to fieldColor, amount float64) fieldColor {
	amount = clamp(amount, 0, 1)
	return fieldColor{
		r: uint8(math.Round(float64(from.r) + (float64(to.r)-float64(from.r))*amount)),
		g: uint8(math.Round(float64(from.g) + (float64(to.g)-float64(from.g))*amount)),
		b: uint8(math.Round(float64(from.b) + (float64(to.b)-float64(from.b))*amount)),
	}
}

func (f densityField) value(x, y int) float64 {
	if x < 0 || y < 0 || x >= f.width || y >= f.height {
		return 0
	}
	return f.values[y*f.width+x]
}

// rasterize turns local density into Braille occupancy and tone. Thresholds
// are fixed and perceptual: the field, rather than random dots, determines
// the shape while sub-cell variation keeps soft edges readable.
func (f densityField) rasterize(canvas *Canvas) {
	if canvas == nil {
		return
	}
	for cellY := 0; cellY < canvas.Height; cellY++ {
		for cellX := 0; cellX < canvas.Width; cellX++ {
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 2; sx++ {
					value := f.value(cellX*2+sx, cellY*4+sy)
					if value <= 0.022 {
						continue
					}
					// Slightly different thresholds across the Braille mask avoid
					// making every low-density cell look like the same glyph.
					threshold := 0.035 + float64((sx+sy*2)%5)*0.02
					if value < threshold {
						continue
					}
					tone := uint8(math.Round(clamp(70+value*185, 70, 245)))
					color := f.color(cellX*2+sx, cellY*4+sy)
					canvas.SetColor(cellX*2+sx, cellY*4+sy, color, tone)
				}
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
