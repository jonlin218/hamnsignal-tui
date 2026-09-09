package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
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
