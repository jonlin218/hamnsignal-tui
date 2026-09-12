package data

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
)

func TestSemanticNumericStringFormatting(t *testing.T) {
	if got := semanticValue("0.4768"); got != "0.48" {
		t.Fatalf("got %q", got)
	}
	if got := semanticValue("repose"); got != "repose" {
		t.Fatalf("got %q", got)
	}
}

func TestActiveVoicesFilterAndDeterministicOrder(t *testing.T) {
	layerBody, layerCantus := "body", "cantus"
	sourceA, sourceB := "a", "b"
	freqLow, freqHigh := hamnsignal.Number(110), hamnsignal.Number(220)
	state := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{
		3: {Fields: hamnsignal.VoiceFields{Layer: &layerBody, Source: &sourceB, Freq: &freqHigh}},
		2: {Fields: hamnsignal.VoiceFields{Layer: &layerBody, Source: &sourceA, Freq: &freqLow}},
		1: {Fields: hamnsignal.VoiceFields{Layer: &layerCantus, Source: &sourceA, Freq: &freqLow}},
		4: {Released: true, Fields: hamnsignal.VoiceFields{Layer: &layerBody}},
	}}
	voices := activeVoices(state)
	if len(voices) != 3 {
		t.Fatalf("got %d active voices", len(voices))
	}
	if *voices[0].Fields.Source != "a" || *voices[1].Fields.Source != "b" || *voices[2].Fields.Layer != "cantus" {
		t.Fatalf("unexpected order: %#v", voices)
	}
}

func TestMissingEnvironmentAndArrivalAreQuiet(t *testing.T) {
	state := hamnsignal.StateSnapshot{Voices: map[int64]hamnsignal.Voice{}}
	view := NewModelFromSnapshot(state, 80, 24)
	output := view.View()
	if !strings.Contains(output, "GÖTA ÄLV") || !strings.Contains(output, "TRAFFIC") || !strings.Contains(output, "LAST ARRIVAL") {
		t.Fatalf("missing empty sections: %s", output)
	}
	if !strings.Contains(output, "awaiting state") {
		t.Fatalf("missing unavailable state: %s", output)
	}
}

func TestTrafficSectionShowsSpeedAndPressure(t *testing.T) {
	state := trafficState(50.4, 0.45)
	output := NewModelFromSnapshot(state, 80, 24).View()
	for _, expected := range []string{"TRAFFIC", "E45 / OSCARSLEDEN", "50.4 km/h", "PRESSURE  0.45", "GÖTA ÄLV", "WEATHER", "MUSICAL STATE", "ACTIVE VOICES", "LAST ARRIVAL"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q in DATA view: %s", expected, output)
		}
	}
}

func TestTrafficSectionShowsZeroPressureNeutrally(t *testing.T) {
	output := NewModelFromSnapshot(trafficState(53.1, 0), 80, 24).View()
	if !strings.Contains(output, "53.1 km/h") || !strings.Contains(output, "PRESSURE  0.00") {
		t.Fatalf("zero traffic pressure was not rendered: %s", output)
	}
	for _, unwanted := range []string{"CLEAR", "BUSY", "HEAVY", "SEVERE", "MOTORIK"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("invented traffic status %q: %s", unwanted, output)
		}
	}
}

func TestTrafficSectionNarrowLayoutDoesNotOverflow(t *testing.T) {
	output := NewModelFromSnapshot(trafficState(50.4, 0.45), 60, 24).View()
	if !strings.Contains(output, "TRAFFIC") || !strings.Contains(output, "E45 / OSCARSLEDEN") {
		t.Fatalf("traffic section missing from narrow view: %s", output)
	}
	for _, line := range strings.Split(output, "\n") {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("narrow view overflowed (%d): %q", lipgloss.Width(line), line)
		}
	}
}

func TestSmallTerminalAndResizeRemainSafe(t *testing.T) {
	model := NewModelFromSnapshot(trafficState(50.4, 0.45), 30, 8)
	if !strings.Contains(model.View(), "Terminal too small") {
		t.Fatal("small terminal message missing")
	}
	model.width, model.height = 120, 35
	if strings.Contains(model.View(), "Terminal too small") {
		t.Fatal("large terminal remained in small mode")
	}
	if !strings.Contains(model.View(), "TRAFFIC") {
		t.Fatal("traffic section missing after resize")
	}
}

func TestAudioFooterAndSpaceControl(t *testing.T) {
	state := hamnsignal.NewState()
	model := NewModel(state, "playing")
	if !strings.Contains(model.View(), "audio ▶ playing") {
		t.Fatal("playing audio status missing")
	}
	model.help = true
	if !strings.Contains(model.View(), "Space  play/pause") {
		t.Fatal("audio help missing")
	}
	called := false
	model.SetAudioToggle(func() error { called = true; return nil })
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	if command == nil {
		t.Fatal("space did not create an audio command")
	}
	_ = command()
	if !called || updated == nil {
		t.Fatal("space did not invoke audio control")
	}
}

// NewModelFromSnapshot keeps presentation tests independent of network state.
func NewModelFromSnapshot(snapshot hamnsignal.StateSnapshot, width, height int) Model {
	state := hamnsignal.NewState()
	state.SetConnected(snapshot.Connected)
	for key, value := range snapshot.Voices {
		state.Voices[key] = value
	}
	for key, value := range snapshot.Weather {
		state.Weather[key] = value
	}
	for key, value := range snapshot.River {
		state.River[key] = value
	}
	for key, value := range snapshot.Traffic {
		state.Traffic[key] = value
	}
	for key, value := range snapshot.Semantic {
		state.Semantic[key] = value
	}
	for key, value := range snapshot.Phrase {
		state.Phrase[key] = value
	}
	state.LastArrival = snapshot.LastArrival
	model := NewModel(state)
	model.width, model.height = width, height
	return model
}

func trafficState(raw, pressure hamnsignal.Number) hamnsignal.StateSnapshot {
	key := "e45_queue"
	return hamnsignal.StateSnapshot{Traffic: map[string]hamnsignal.EnvironmentalValue{
		key: {Fields: hamnsignal.EnvironmentalFields{Key: &key, RawValue: &raw, NormalizedValue: &pressure}},
	}}
}
