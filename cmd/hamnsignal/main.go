package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonlin218/hamnsignal-tui/internal/app"
	"github.com/jonlin218/hamnsignal-tui/internal/audio"
	"github.com/jonlin218/hamnsignal-tui/internal/hamnsignal"
)

func main() {
	events := flag.Bool("events", false, "print incoming Hamnsignal events")
	noAudio := flag.Bool("no-audio", false, "disable mpv audio playback")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	state := hamnsignal.NewState()
	if !*events {
		runData(ctx, stop, state, *noAudio)
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

func runData(ctx context.Context, stop context.CancelFunc, state *hamnsignal.State, noAudio bool) {
	var program *tea.Program
	player := audio.NewPlayer(audio.Options{Disabled: noAudio, OnStatus: func(status audio.Status) {
		if program != nil {
			program.Send(app.AudioChangedMsg{Status: string(status)})
		}
	}})
	model := app.NewModel(state, string(player.Status()))
	model.SetAudioToggle(player.Toggle)
	program = tea.NewProgram(model, tea.WithAltScreen())
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
