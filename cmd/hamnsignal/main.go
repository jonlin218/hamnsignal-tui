package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonlin218/hamnsignal-tui/internal/app"
	"github.com/jonlin218/hamnsignal-tui/internal/audio"
	"github.com/jonlin218/hamnsignal-tui/internal/diagnostic"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
	"github.com/jonlin218/hamnsignal-tui/internal/visual"
)

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "hamnsignal:", err)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	state := hamnsignal.NewState()
	if !options.events {
		runData(ctx, stop, state, options.noAudio, options.fixture)
		return
	}

	client := hamnsignal.Client{
		OnConnection: state.SetConnected,
		OnEvent:      func(event hamnsignal.Event) { state.Apply(event); printEvent(event) },
		OnError: func(err error) {
			if ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "hamnsignal:", err)
			}
		},
	}
	_ = client.Run(ctx)
}

type options struct {
	events, noAudio bool
	fixture         visual.VisualFixture
}

func parseOptions(arguments []string) (options, error) {
	flags := flag.NewFlagSet("hamnsignal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	events := flags.Bool("events", false, "print incoming Hamnsignal events")
	noAudio := flags.Bool("no-audio", false, "disable mpv audio playback")
	traffic := flags.String("visual-traffic", "", "development-only normalized traffic pressure for VISUAL (0..1)")
	rain := flags.String("visual-rain", "", "development-only precipitation for VISUAL (0..10)")
	motorikClick := flags.Bool("visual-motorik-click", false, "development-only 152 BPM motorik timing reference")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	fixture, err := parseVisualTraffic(*traffic)
	if err != nil {
		return options{}, err
	}
	precipitation, err := parseVisualRain(*rain)
	if err != nil {
		return options{}, err
	}
	fixture.Precipitation = precipitation
	fixture.MotorikClick = *motorikClick
	return options{events: *events, noAudio: *noAudio, fixture: fixture}, nil
}

func parseVisualRain(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	precipitation, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(precipitation) || math.IsInf(precipitation, 0) {
		return nil, fmt.Errorf("--visual-rain must be a number from 0 to 10")
	}
	if precipitation < 0 || precipitation > 10 {
		return nil, fmt.Errorf("--visual-rain must be between 0 and 10")
	}
	return &precipitation, nil
}

func parseVisualTraffic(value string) (visual.VisualFixture, error) {
	if value == "" {
		return visual.VisualFixture{}, nil
	}
	pressure, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(pressure) || math.IsInf(pressure, 0) {
		return visual.VisualFixture{}, fmt.Errorf("--visual-traffic must be a number from 0 to 1")
	}
	if pressure < 0 || pressure > 1 {
		return visual.VisualFixture{}, fmt.Errorf("--visual-traffic must be between 0 and 1")
	}
	return visual.VisualFixture{TrafficPressure: &pressure}, nil
}

func runData(ctx context.Context, stop context.CancelFunc, state *hamnsignal.State, noAudio bool, fixture visual.VisualFixture) {
	var program *tea.Program
	player := audio.NewPlayer(audio.Options{Disabled: noAudio, OnStatus: func(status audio.Status) {
		if program != nil {
			program.Send(app.AudioChangedMsg{Status: string(status)})
		}
	}})
	model := app.NewModel(state, string(player.Status()), fixture)
	model.SetAudioToggle(player.Toggle)
	program = tea.NewProgram(model, tea.WithAltScreen())
	var click *diagnostic.MotorikClick
	if fixture.MotorikClick {
		click = diagnostic.NewMotorikClick()
		if err := click.Start(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "hamnsignal: motorik click:", err)
			click = nil
		}
	}
	client := hamnsignal.Client{
		OnConnection: func(connected bool) {
			state.SetConnected(connected)
			program.Send(app.StateChangedMsg{})
		},
		OnEvent: func(event hamnsignal.Event) {
			state.Apply(event)
			program.Send(app.StateChangedMsg{})
		},
		OnError: func(error) { program.Send(app.StateChangedMsg{}) },
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = client.Run(ctx) }()
	audioDone := make(chan struct{})
	go func() {
		defer close(audioDone)
		if !noAudio {
			_ = player.Start()
		}
	}()
	go func() { <-ctx.Done(); program.Quit() }()
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "hamnsignal:", err)
	}
	stop()
	if click != nil {
		click.Stop()
	}
	_ = player.Stop()
	<-audioDone
	<-done
}

func printEvent(event hamnsignal.Event) {
	meta := event.Meta()
	stamp := time.UnixMilli(meta.TimeUnixMS).UTC().Format(time.RFC3339Nano)
	fields := meta.Fields
	if len(fields) == 0 {
		fields = []byte("{}")
	}
	var pretty any
	if err := json.Unmarshal(fields, &pretty); err == nil {
		if formatted, err := json.MarshalIndent(pretty, "  ", "  "); err == nil {
			fields = formatted
		}
	}
	fmt.Printf("%s  %s\n  %s\n", stamp, meta.Type, fields)
}
