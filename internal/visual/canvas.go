// Package visual contains the VISUAL presentation layer and terminal canvas.
package visual

import (
	"os"
	"strings"
)

// Canvas stores a high-resolution dot grid projected onto Braille cells.
// Every terminal cell represents two columns by four rows of virtual pixels.
type Canvas struct {
	Width  int
	Height int
	dots   []uint8
	tones  []uint8
	colors []fieldColor
}

func NewCanvas(width, height int) Canvas {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return Canvas{Width: width, Height: height, dots: make([]uint8, width*height), tones: make([]uint8, width*height), colors: make([]fieldColor, width*height)}
}

func (c *Canvas) Set(x, y int) {
	c.SetTone(x, y, 255)
}

// SetTone marks a dot and records its luminance on a 0–255 scale. Repeated
// marks accumulate by retaining the brightest tone in the cell.
func (c *Canvas) SetTone(x, y int, tone uint8) {
	r, g, b := coolTone(tone)
	c.SetColor(x, y, fieldColor{uint8(r), uint8(g), uint8(b)}, tone)
}

// SetColor marks a dot with its resolved field colour. The brightest
// sub-cell contribution selects the terminal cell's foreground colour.
func (c *Canvas) SetColor(x, y int, color fieldColor, tone uint8) {
	if c == nil || x < 0 || y < 0 || x >= c.Width*2 || y >= c.Height*4 {
		return
	}
	cellX, cellY := x/2, y/4
	bit := brailleBit(x%2, y%4)
	index := cellY*c.Width + cellX
	c.dots[index] |= bit
	if tone > c.tones[index] {
		c.tones[index] = tone
		c.colors[index] = color
	}
}

func brailleBit(x, y int) uint8 {
	return [...]uint8{1, 2, 4, 64, 8, 16, 32, 128}[y+4*x]
}

func (c Canvas) Render() string {
	return c.render(false)
}

// RenderANSI renders the same structure with a small cool truecolor palette.
// NO_COLOR and dumb terminals receive the plain monochrome representation.
func (c Canvas) RenderANSI() string {
	if os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return c.render(false)
	}
	return c.render(true)
}

func (c Canvas) render(colour bool) string {
	if c.Width <= 0 || c.Height <= 0 {
		return ""
	}
	rows := make([]string, c.Height)
	for y := 0; y < c.Height; y++ {
		var row strings.Builder
		row.Grow(c.Width)
		for x := 0; x < c.Width; x++ {
			index := y*c.Width + x
			cell := rune(0x2800 + int(c.dots[index]))
			if colour && c.dots[index] != 0 {
				colour := c.colors[index]
				r, g, b := int(colour.r), int(colour.g), int(colour.b)
				if colour == (fieldColor{}) {
					r, g, b = coolTone(c.tones[index])
				}
				row.WriteString("\x1b[38;2;")
				row.WriteString(strings.Join([]string{itoa(r), itoa(g), itoa(b)}, ";"))
				row.WriteString("m")
				row.WriteRune(cell)
				row.WriteString("\x1b[0m")
			} else {
				row.WriteRune(cell)
			}
		}
		rows[y] = row.String()
	}
	return strings.Join(rows, "\n")
}

func coolTone(tone uint8) (int, int, int) {
	// Deep blue-grey through cool cyan to near-white. The upper end is rare:
	// normal marks remain atmospheric and wakes stay visibly recessed.
	t := float64(tone) / 255
	return int(38 + 185*t), int(55 + 190*t), int(72 + 180*t)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
