// Package data contains the Bubble Tea DATA view.
package data

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
)

type StateChangedMsg struct{}
type AudioChangedMsg struct{ Status string }

type Model struct {
	state       *hamnsignal.State
	width       int
	height      int
	help        bool
	audio       string
	toggleAudio func() error
}

func NewModel(state *hamnsignal.State, audio ...string) Model {
	status := "disabled"
	if len(audio) > 0 && audio[0] != "" {
		status = audio[0]
	}
	return Model{state: state, width: 80, height: 24, audio: status}
}
func (m *Model) SetAudioToggle(toggle func() error) { m.toggleAudio = toggle }
func (m *Model) SetAudioStatus(status string) {
	if status != "" {
		m.audio = status
	}
}
func (m *Model) SetHelp(help bool)        { m.help = help }
func (m *Model) Resize(width, height int) { m.width, m.height = width, height }
func (m Model) Init() tea.Cmd             { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			if m.toggleAudio != nil {
				toggle := m.toggleAudio
				return m, func() tea.Msg { _ = toggle(); return nil }
			}
		case "?":
			m.help = !m.help
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case StateChangedMsg:
	case AudioChangedMsg:
		m.audio = msg.Status
	}
	return m, nil
}

func (m Model) View() string {
	if m.width < 40 || m.height < 18 {
		return m.smallView()
	}
	snapshot := m.state.Snapshot()
	contentWidth := m.width
	if contentWidth < 1 {
		contentWidth = 1
	}
	var body string
	if contentWidth >= 72 {
		columnWidth := (contentWidth - 3) / 2
		left := strings.Join([]string{riverSection(snapshot, columnWidth), weatherSection(snapshot, columnWidth), arrivalSection(snapshot, columnWidth)}, "\n\n")
		right := strings.Join([]string{musicalSection(snapshot, columnWidth), voiceSection(snapshot, columnWidth, m.height)}, "\n\n")
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(columnWidth).Render(left), "   ", lipgloss.NewStyle().Width(columnWidth).Render(right))
	} else {
		sections := []string{riverSection(snapshot, contentWidth), weatherSection(snapshot, contentWidth), musicalSection(snapshot, contentWidth), voiceSection(snapshot, contentWidth, m.height)}
		if m.height >= 24 {
			sections = append(sections, arrivalSection(snapshot, contentWidth))
		}
		separator := "\n\n"
		if m.height < 28 {
			separator = "\n"
		}
		body = strings.Join(sections, separator)
	}
	header := headerView(snapshot.Connected, contentWidth)
	footer := footerView(m.audio, snapshot.Connected, contentWidth)
	if m.help {
		return header + "\n\n" + body + "\n\n" + helpView(contentWidth)
	}
	return header + "\n\n" + body + "\n\n" + footer
}

var (
	sectionStyle = lipgloss.NewStyle().Bold(true)
	mutedStyle   = lipgloss.NewStyle().Faint(true)
	accentStyle  = lipgloss.NewStyle().Bold(true)
)

func headerView(connected bool, width int) string {
	status := "websocket ○ reconnecting"
	if connected {
		status = "websocket ● connected"
	}
	gap := width - lipgloss.Width("HAMNSIGNAL") - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}
	return lipgloss.NewStyle().Width(width).Render(accentStyle.Render("HAMNSIGNAL") + strings.Repeat(" ", gap) + mutedStyle.Render(status))
}

func footerView(audio string, connected bool, width int) string {
	audioText := "audio — disabled"
	switch audio {
	case "starting":
		audioText = "audio … starting"
	case "playing":
		audioText = "audio ▶ playing"
	case "paused":
		audioText = "audio ❚❚ paused"
	case "stopped":
		audioText = "audio ○ stopped"
	case "unavailable":
		audioText = "audio ! unavailable"
	case "error":
		audioText = "audio ! error"
	}
	status := "websocket ○ reconnecting"
	if connected {
		status = "websocket ● connected"
	}
	gap := width - lipgloss.Width(audioText) - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}
	return mutedStyle.Render(lipgloss.NewStyle().Width(width).Render(audioText + strings.Repeat(" ", gap) + status))
}

func helpView(width int) string {
	return mutedStyle.Render(lipgloss.NewStyle().Width(width).Render("Space  play/pause    q / Ctrl+C  quit    ?  close help"))
}

func section(title string, lines []string, width int) string {
	if len(lines) == 0 {
		lines = []string{mutedStyle.Render("awaiting state")}
	}
	return lipgloss.NewStyle().Width(width).Render(sectionStyle.Render(title) + "\n" + strings.Join(lines, "\n"))
}

func riverSection(state hamnsignal.StateSnapshot, width int) string {
	value, ok := state.River["river_level"]
	if !ok {
		return section("GÖTA ÄLV", nil, width)
	}
	lines := []string{}
	if value.Fields.RawValue != nil {
		lines = append(lines, fmt.Sprintf("level       %s", number(*value.Fields.RawValue, 3)))
	}
	if value.Fields.NormalizedValue != nil {
		lines = append(lines, fmt.Sprintf("normalized  %s", number(*value.Fields.NormalizedValue, 3)))
	}
	return section("GÖTA ÄLV", lines, width)
}

func weatherSection(state hamnsignal.StateSnapshot, width int) string {
	keys := []struct{ key, label, suffix string }{{"temperature", "temperature", " °C"}, {"wind_speed", "wind speed", " m/s"}, {"wind_direction", "wind direction", "°"}, {"precipitation", "precipitation", " mm"}, {"global_radiation", "global radiation", " W/m²"}}
	lines := []string{}
	for _, item := range keys {
		if width < 60 && (item.key == "precipitation" || item.key == "global_radiation") {
			continue
		}
		value, ok := state.Weather[item.key]
		if !ok || value.Fields.RawValue == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-17s%s", item.label, number(*value.Fields.RawValue, 2)+item.suffix))
	}
	return section("WEATHER", lines, width)
}

func musicalSection(state hamnsignal.StateSnapshot, width int) string {
	lines := []string{}
	if phrase, ok := state.Phrase["phraseState"]; ok {
		lines = append(lines, "phrase       "+semanticValue(phrase))
	}
	if brightness, ok := state.Semantic["brightness"]; ok {
		lines = append(lines, "brightness   "+semanticValue(brightness))
	}
	active := 0
	for _, voice := range state.Voices {
		if !voice.Released {
			active++
		}
	}
	lines = append(lines, fmt.Sprintf("voices       %d", active))
	return section("MUSICAL STATE", lines, width)
}

func voiceSection(state hamnsignal.StateSnapshot, width, height int) string {
	voices := activeVoices(state)
	limit := 4
	if height >= 24 {
		limit = 6
	}
	if height >= 32 {
		limit = 8
	}
	if len(voices) > limit {
		voices = voices[:limit]
	}
	lines := []string{}
	for _, voice := range voices {
		identity := "?"
		if voice.Fields.Layer != nil {
			identity = *voice.Fields.Layer
		}
		if voice.Fields.Source != nil && *voice.Fields.Source != "" {
			identity += "/" + *voice.Fields.Source
		}
		frequency := "—"
		if voice.Fields.Freq != nil {
			frequency = number(*voice.Fields.Freq, 1) + " Hz"
		}
		lines = append(lines, fmt.Sprintf("%-18s%s", identity, frequency))
	}
	return section("ACTIVE VOICES", lines, width)
}

func arrivalSection(state hamnsignal.StateSnapshot, width int) string {
	if state.LastArrival == nil {
		return section("LAST ARRIVAL", nil, width)
	}
	fields := state.LastArrival.Fields
	lines := []string{}
	modeLine := ""
	if fields.Mode != nil {
		modeLine = *fields.Mode
	}
	if fields.Line != nil {
		if modeLine != "" {
			modeLine += " "
		}
		modeLine += *fields.Line
	}
	if modeLine != "" {
		lines = append(lines, strings.ToUpper(modeLine))
	}
	location := ""
	if fields.Site != nil {
		location = strings.ToUpper(*fields.Site)
	}
	if fields.Platform != nil {
		if location != "" {
			location += " · "
		}
		location += "PLATFORM " + strings.ToUpper(*fields.Platform)
	}
	if location != "" {
		lines = append(lines, location)
	}
	return section("LAST ARRIVAL", lines, width)
}

type voiceRow struct {
	id    int64
	voice hamnsignal.Voice
}

func activeVoices(state hamnsignal.StateSnapshot) []hamnsignal.Voice {
	rows := []voiceRow{}
	for id, voice := range state.Voices {
		if !voice.Released {
			rows = append(rows, voiceRow{id: id, voice: voice})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := rows[i].voice.Fields, rows[j].voice.Fields
		layer := func(fields hamnsignal.VoiceFields) string {
			if fields.Layer == nil {
				return ""
			}
			return *fields.Layer
		}
		source := func(fields hamnsignal.VoiceFields) string {
			if fields.Source == nil {
				return ""
			}
			return *fields.Source
		}
		if layer(left) != layer(right) {
			return layer(left) < layer(right)
		}
		if source(left) != source(right) {
			return source(left) < source(right)
		}
		lf, rf := float64(0), float64(0)
		if left.Freq != nil {
			lf = float64(*left.Freq)
		}
		if right.Freq != nil {
			rf = float64(*right.Freq)
		}
		if lf != rf {
			return lf < rf
		}
		return rows[i].id < rows[j].id
	})
	result := make([]hamnsignal.Voice, len(rows))
	for i, row := range rows {
		result[i] = row.voice
	}
	return result
}

func number(value hamnsignal.Number, decimals int) string {
	return strconv.FormatFloat(float64(value), 'f', decimals, 64)
}
func semanticValue(value any) string {
	if text, ok := value.(string); ok {
		if parsed, err := strconv.ParseFloat(text, 64); err == nil {
			return strconv.FormatFloat(parsed, 'f', 2, 64)
		}
		return text
	}
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', 2, 64)
	}
	return fmt.Sprint(value)
}

func (m Model) smallView() string {
	connected := false
	if m.state != nil {
		connected = m.state.Snapshot().Connected
	}
	status := "websocket ○ reconnecting"
	if connected {
		status = "websocket ● connected"
	}
	return accentStyle.Render("HAMNSIGNAL") + "\n\n" + mutedStyle.Render("Terminal too small for DATA view") + "\n" + mutedStyle.Render(audioLabel(m.audio)) + "   " + mutedStyle.Render(status)
}

func audioLabel(audio string) string {
	switch audio {
	case "starting":
		return "audio … starting"
	case "playing":
		return "audio ▶ playing"
	case "paused":
		return "audio ❚❚ paused"
	case "stopped":
		return "audio ○ stopped"
	case "unavailable":
		return "audio ! unavailable"
	case "error":
		return "audio ! error"
	default:
		return "audio — disabled"
	}
}
