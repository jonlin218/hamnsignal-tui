package data

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	if !strings.Contains(output, "GÖTA ÄLV") || !strings.Contains(output, "LAST ARRIVAL") {
		t.Fatalf("missing empty sections: %s", output)
	}
	if !strings.Contains(output, "awaiting state") {
		t.Fatalf("missing unavailable state: %s", output)
	}
}

func TestSmallTerminalAndResizeRemainSafe(t *testing.T) {
	state := hamnsignal.NewState()
	model := NewModel(state)
	model.width, model.height = 30, 8
	if !strings.Contains(model.View(), "Terminal too small") {
		t.Fatal("small terminal message missing")
	}
	model.width, model.height = 120, 35
	if strings.Contains(model.View(), "Terminal too small") {
		t.Fatal("large terminal remained in small mode")
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
