<img width="1680" height="1050" alt="hamnsignal-tui" src="https://github.com/user-attachments/assets/2492a7ca-a303-4e3d-9ab6-a0a93187b5d6" />

# Hamnsignal TUI

Hamnsignal TUI is a Go terminal client for the live [Hamnsignal](https://hamnsignal.se) installation.

**DATA** explains the current system state. **VISUAL** turns that same state into an abstract terminal-native density field. Audio is played independently by `mpv`.

The two views are different readings of the same live system: one explains it, the other lets it leave a trace.

## Requirements

- `mpv` for audio playback
- Network access
- A Unicode terminal with Braille and ANSI/truecolour support

Building from source additionally requires Go.

## Download

Prebuilt binaries are available from the GitHub Releases page.

v0.2 currently provides a macOS Apple Silicon (`darwin-arm64`) build.

Audio playback requires `mpv` to be installed and available in `PATH`.

## Build and run

```bash
go build ./cmd/hamnsignal
./hamnsignal
```

Run without audio:

```bash
./hamnsignal --no-audio
```

Inspect incoming events:

```bash
./hamnsignal --events
```

## Controls

```text
TAB      switch DATA / VISUAL
1        DATA
2        VISUAL
SPACE    play / pause audio
?        help
q        quit
CTRL+C   quit
```

## DATA

DATA presents the current Hamnsignal state in a readable terminal view, including:

- Göta älv
- Weather
- Traffic
- Musical state
- Active voices
- Last arrival

## VISUAL

VISUAL is an abstract interpretation of the same state observed by DATA.

Voice activity forms a shared Braille density field. Environmental and event state can also leave distinct traces:

- Göta älv forms a persistent lower field
- Wind influences spatial movement
- Rain produces sparse falling traces
- Arrivals create temporary disturbances
- Traffic pressure introduces a 152 BPM motorik current
- Global radiation forms an atmospheric upper field

These behaviours interact through the shared visual field rather than being presented as separate graphs or widgets.

VISUAL is not a waveform, spectrum, FFT, or direct audio visualization.

> Hamnsignal determines why a visual behaviour exists. The TUI is allowed to interpret how that behaviour is expressed.

## Development fixtures

VISUAL states can be inspected locally without altering Hamnsignal's actual DATA state:

```bash
./hamnsignal --visual-traffic=0.80
./hamnsignal --visual-rain=5
./hamnsignal --visual-radiation=0.85
./hamnsignal --visual-motorik-click
```

Fixtures can be combined and used with `--no-audio`.

## Sources

Hamnsignal provides the live state and audio used by the client:

```text
wss://hamnsignal.se/ws
https://hamnsignal.se/live.mp3
```

## Limitations

`mpv` is external. Braille, colour and density appearance depend on terminal, font and transparency settings.

Hamnsignal TUI has no history, persistence, or configuration UI.
