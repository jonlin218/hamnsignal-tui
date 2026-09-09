<img width="1680" height="1050" alt="hamnsignal-tui" src="https://github.com/user-attachments/assets/2492a7ca-a303-4e3d-9ab6-a0a93187b5d6" />

# Hamnsignal TUI

Hamnsignal TUI is a Go terminal client for the live Hamnsignal installation.
**DATA** explains the current system state; **VISUAL** turns that same state
into an abstract Braille density field. Audio is played independently by mpv.

## Requirements

Go, `mpv`, network access, and a Unicode terminal with Braille and ANSI/truecolour support.

## Build and run

```bash
go build ./cmd/hamnsignal
./hamnsignal
./hamnsignal --no-audio
./hamnsignal --events
```

Controls: `TAB` switches views, `1` DATA, `2` VISUAL, `SPACE` plays/pauses audio,
`?` help, and `q` or `Ctrl+C` quits.

Sources: `wss://hamnsignal.se/ws` supplies graphics/state events and
`https://hamnsignal.se/live.mp3` supplies audio.

VISUAL is derived from Hamnsignal state events. It is not a waveform, spectrum,
FFT, or direct audio visualization.

## Limitations

mpv is external; Braille and colour appearance depend on terminal/font/transparency.
v0.1 has no history, persistence, or configuration UI. Natural non-zero-rain
appearance remains to be observed live.
