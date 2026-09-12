package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
	"github.com/jonlin218/hamnsignal-tui/internal/visual"
)

func TestViewSwitchingAndHelp(t *testing.T) {
	model := NewModel(hamnsignal.NewState(), "disabled")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.view != VisualView {
		t.Fatal("tab did not select visual view")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = updated.(Model)
	if model.view != DataView {
		t.Fatal("1 did not select data view")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model = updated.(Model)
	if model.view != VisualView {
		t.Fatal("2 did not select visual view")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(Model)
	if !strings.Contains(model.View(), "TAB switch view") {
		t.Fatal("help did not include view controls")
	}
}

func TestVisualTrafficFixtureLeavesDataTruthfulAndIsIndicated(t *testing.T) {
	state := hamnsignal.NewState()
	key, livePressure := "e45_queue", hamnsignal.Number(0)
	state.Traffic[key] = hamnsignal.EnvironmentalValue{Fields: hamnsignal.EnvironmentalFields{Key: &key, NormalizedValue: &livePressure}}
	fixture := .8
	model := NewModel(state, "disabled", visual.VisualFixture{TrafficPressure: &fixture})
	output := model.View()
	if !strings.Contains(output, "PRESSURE  0.00") || !strings.Contains(output, "VISUAL FIXTURE — TRAFFIC 0.80") {
		t.Fatalf("fixture obscured truthful DATA: %s", output)
	}
	if value := *state.Snapshot().Traffic[key].Fields.NormalizedValue; value != 0 {
		t.Fatalf("fixture mutated central traffic state: %v", value)
	}

	ordinary := NewModel(state, "disabled")
	if strings.Contains(ordinary.View(), "VISUAL FIXTURE") {
		t.Fatal("ordinary startup displayed a fixture indicator")
	}
}

func TestMotorikClickFixtureIsIndicatedWithoutChangingData(t *testing.T) {
	state := hamnsignal.NewState()
	model := NewModel(state, "disabled", visual.VisualFixture{MotorikClick: true})
	if output := model.View(); !strings.Contains(output, "VISUAL FIXTURE — MOTORIK CLICK") || strings.Contains(output, "PRESSURE") {
		t.Fatalf("unexpected click fixture presentation: %s", output)
	}
	pressure := .8
	combined := NewModel(state, "disabled", visual.VisualFixture{TrafficPressure: &pressure, MotorikClick: true})
	if output := combined.View(); !strings.Contains(output, "TRAFFIC 0.80 — MOTORIK CLICK") {
		t.Fatalf("combined fixture indication missing: %s", output)
	}
}

func TestRainFixtureLeavesLiveDataTruthfulAndComposes(t *testing.T) {
	state := hamnsignal.NewState()
	key, liveRain := "precipitation", hamnsignal.Number(0)
	state.Weather[key] = hamnsignal.EnvironmentalValue{Fields: hamnsignal.EnvironmentalFields{Key: &key, RawValue: &liveRain}}
	rain, traffic, radiation := .8, .5, .85
	model := NewModel(state, "disabled", visual.VisualFixture{TrafficPressure: &traffic, Precipitation: &rain, Radiation: &radiation, MotorikClick: true})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 160, Height: 35})
	model = updated.(Model)
	output := model.View()
	if !strings.Contains(output, "precipitation    0.00 mm") || !strings.Contains(output, "TRAFFIC 0.50 — RAIN 0.80 — RADIATION 0.85 — MOTORIK CLICK") {
		t.Fatalf("rain fixture obscured live DATA or indication: %s", output)
	}
	if value := *state.Snapshot().Weather[key].Fields.RawValue; value != 0 {
		t.Fatalf("rain fixture mutated central weather state: %v", value)
	}
}

func TestRadiationFixtureExactUpperBoundIsIndicated(t *testing.T) {
	state := hamnsignal.NewState()
	for _, radiation := range []float64{1, 1.0} {
		model := NewModel(state, "disabled", visual.VisualFixture{Radiation: &radiation})
		if output := model.View(); !strings.Contains(output, "VISUAL FIXTURE — RADIATION 1.00") {
			t.Fatalf("exact radiation fixture %v indication missing: %s", radiation, output)
		}
	}
}

func TestVisualResizeSafetyAndDataStateSurvivesSwitch(t *testing.T) {
	state := hamnsignal.NewState()
	event, err := hamnsignal.DecodeEvent([]byte(`{"type":"weather_state","time_unix_ms":1,"fields":{"key":"temperature","raw_value":"15.7"}}`))
	if err != nil {
		t.Fatal(err)
	}
	state.Apply(event)
	model := NewModel(state, "disabled")
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	model = updated.(Model)
	if !strings.Contains(model.View(), "Terminal too small") {
		t.Fatal("zero visual size was not handled")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	model = updated.(Model)
	if model.View() == "" {
		t.Fatal("large visual view was empty")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = updated.(Model)
	if !strings.Contains(model.View(), "15.70") {
		t.Fatal("data state was lost across view switch")
	}
}
