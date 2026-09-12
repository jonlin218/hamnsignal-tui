// Package app coordinates the DATA and VISUAL views without coupling either
// renderer to the WebSocket receiver.
package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonlin218/hamnsignal-tui/internal/data"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
	"github.com/jonlin218/hamnsignal-tui/internal/visual"
)

type View int

const (
	DataView View = iota
	VisualView
)

type StateChangedMsg struct{}
type AudioChangedMsg struct{ Status string }
type tickMsg struct{ At time.Time }

type Model struct {
	state       *hamnsignal.State
	data        data.Model
	presenter   *visual.Presenter
	view        View
	width       int
	height      int
	help        bool
	audio       string
	toggleAudio func() error
	fixture     visual.VisualFixture
}

func NewModel(state *hamnsignal.State, audioStatus string, fixtures ...visual.VisualFixture) Model {
	fixture := visual.VisualFixture{}
	if len(fixtures) > 0 {
		fixture = fixtures[0]
	}
	presenter := visual.NewPresenter()
	presenter.SetFixture(fixture)
	return Model{state: state, data: data.NewModel(state, audioStatus), presenter: presenter, width: 80, height: 24, audio: audioStatus, fixture: fixture}
}

func (m *Model) SetAudioToggle(toggle func() error) {
	m.toggleAudio = toggle
	m.data.SetAudioToggle(toggle)
}
func (m Model) Init() tea.Cmd { return tickCmd() }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.view = 1 - m.view
			m.help = false
		case "1":
			m.view = DataView
			m.help = false
		case "2":
			m.view = VisualView
			m.help = false
		case "?":
			m.help = !m.help
		case " ":
			if m.toggleAudio != nil {
				toggle := m.toggleAudio
				return m, func() tea.Msg { _ = toggle(); return nil }
			}
		}
		m.data.SetHelp(m.help)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.data.Resize(msg.Width, msg.Height)
	case AudioChangedMsg:
		m.audio = msg.Status
		m.data.SetAudioStatus(msg.Status)
	case tickMsg:
		m.presenter.Sync(m.state.Snapshot(), msg.At)
		return m, tickCmd()
	case StateChangedMsg:
	}
	return m, nil
}

func (m Model) View() string {
	var content string
	if m.view == VisualView {
		if m.help {
			content = m.visualView() + "\n\n" + visualHelp(m.width)
		} else {
			content = m.visualView()
		}
	} else if m.help {
		content = m.data.View() + "\n\n" + dataHelp(m.width)
	} else {
		content = m.data.View()
	}
	parts := make([]string, 0, 3)
	if m.fixture.TrafficPressure != nil {
		parts = append(parts, fmt.Sprintf("TRAFFIC %.2f", *m.fixture.TrafficPressure))
	}
	if m.fixture.Precipitation != nil {
		parts = append(parts, fmt.Sprintf("RAIN %.2f", *m.fixture.Precipitation))
	}
	if m.fixture.MotorikClick {
		parts = append(parts, "MOTORIK CLICK")
	}
	if len(parts) > 0 {
		return content + "\nVISUAL FIXTURE — " + strings.Join(parts, " — ")
	}
	return content
}

func (m Model) visualView() string {
	if m.width < 40 || m.height < 8 {
		return "HAMNSIGNAL\n\nTerminal too small for VISUAL view"
	}
	return m.presenter.RenderANSI(m.state.Snapshot(), m.width, m.height, time.Now())
}

func tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(at time.Time) tea.Msg { return tickMsg{At: at} })
}
func visualHelp(width int) string { return helpLine(width) }
func dataHelp(width int) string   { return helpLine(width) }
func helpLine(width int) string {
	text := "TAB switch view   1 DATA   2 VISUAL   Space play/pause   q / Ctrl+C quit   ? close help"
	if width < 1 {
		return text
	}
	return text
}
