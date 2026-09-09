// Package hamnsignal decodes and consumes the public Hamnsignal relay stream.
package hamnsignal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

const Endpoint = "wss://hamnsignal.se/ws"

// Envelope is the common public relay packet shape. Fields is retained as raw
// JSON so diagnostic clients can inspect it without losing information.
type Envelope struct {
	TimeUnixMS int64           `json:"time_unix_ms"`
	Source     string          `json:"source"`
	Path       string          `json:"path"`
	Type       string          `json:"type"`
	Args       json.RawMessage `json:"args"`
	Fields     json.RawMessage `json:"fields"`
}

// Event is one relay event. UnknownEvent deliberately implements Event so new
// protocol messages can pass through a client unchanged.
type Event interface {
	Meta() Envelope
}

type VoiceStart struct {
	Envelope
	Fields VoiceFields
}

type VoiceEnd struct {
	Envelope
	Fields VoiceEndFields
}

type StateUpdate struct {
	Envelope
	Fields StateUpdateFields
}

type LiveArrival struct {
	Envelope
	Fields ArrivalFields
}

type WeatherState struct {
	Envelope
	Fields EnvironmentalFields
}

type RiverState struct {
	Envelope
	Fields EnvironmentalFields
}

type UnknownEvent struct{ Envelope }

func (e VoiceStart) Meta() Envelope   { return e.Envelope }
func (e VoiceEnd) Meta() Envelope     { return e.Envelope }
func (e StateUpdate) Meta() Envelope  { return e.Envelope }
func (e LiveArrival) Meta() Envelope  { return e.Envelope }
func (e WeatherState) Meta() Envelope { return e.Envelope }
func (e RiverState) Meta() Envelope   { return e.Envelope }
func (e UnknownEvent) Meta() Envelope { return e.Envelope }

// VoiceFields contains the documented voice_start fields. Pointers preserve
// the distinction between a missing optional field and a zero value.
type VoiceFields struct {
	NodeID      *Int64  `json:"nodeId"`
	Layer       *string `json:"layer"`
	Freq        *Number `json:"freq"`
	Amp         *Number `json:"amp"`
	Pan         *Number `json:"pan"`
	Attack      *Number `json:"attack"`
	Release     *Number `json:"release"`
	Brightness  *Number `json:"brightness"`
	Stability   *Number `json:"stability"`
	Synth       *string `json:"synth"`
	Source      *string `json:"source"`
	PhraseState *string `json:"phraseState"`
	PhrasePos   *Number `json:"phrasePos"`
	Motif       *string `json:"motif"`
}

type VoiceEndFields struct {
	NodeID *Int64 `json:"nodeId"`
}

type StateUpdateFields struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type ArrivalFields struct {
	Site     *string `json:"site"`
	Mode     *string `json:"mode"`
	Line     *string `json:"line"`
	Platform *string `json:"platform"`
	Source   *string `json:"source"`
}

type EnvironmentalFields struct {
	Key              *string `json:"key"`
	Source           *string `json:"source"`
	Label            *string `json:"label"`
	RawValue         *Number `json:"raw_value"`
	NormalizedValue  *Number `json:"normalized_value"`
	ObservedAtUnixMS *Int64  `json:"observed_at_unix_ms"`
}

// Number and Int64 accept the stringified numbers emitted by the live relay as
// well as ordinary JSON numbers shown in the client guide examples.
type Number float64
type Int64 int64

func (n *Number) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*n = Number(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("number must be numeric or a numeric string: %w", err)
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("parse numeric string %q: %w", text, err)
	}
	*n = Number(parsed)
	return nil
}

func (n *Int64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var number int64
	if err := json.Unmarshal(data, &number); err == nil {
		*n = Int64(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("integer must be numeric or a numeric string: %w", err)
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("parse integer string %q: %w", text, err)
	}
	*n = Int64(parsed)
	return nil
}

// DecodeEnvelope decodes only the stable relay envelope.
func DecodeEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode event envelope: %w", err)
	}
	return envelope, nil
}

// DecodeEvent decodes known packets into typed events and leaves future event
// types as UnknownEvent. Unknown fields are ignored by encoding/json.
func DecodeEvent(data []byte) (Event, error) {
	envelope, err := DecodeEnvelope(data)
	if err != nil {
		return nil, err
	}

	fields := envelope.Fields
	if len(fields) == 0 || bytes.Equal(bytes.TrimSpace(fields), []byte("null")) {
		fields = []byte("{}")
	}

	switch envelope.Type {
	case "voice_start":
		var value VoiceFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode voice_start fields: %w", err)
		}
		return VoiceStart{Envelope: envelope, Fields: value}, nil
	case "voice_end":
		var value VoiceEndFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode voice_end fields: %w", err)
		}
		return VoiceEnd{Envelope: envelope, Fields: value}, nil
	case "state_update":
		var value StateUpdateFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode state_update fields: %w", err)
		}
		return StateUpdate{Envelope: envelope, Fields: value}, nil
	case "live_arrival":
		var value ArrivalFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode live_arrival fields: %w", err)
		}
		return LiveArrival{Envelope: envelope, Fields: value}, nil
	case "weather_state":
		var value EnvironmentalFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode weather_state fields: %w", err)
		}
		return WeatherState{Envelope: envelope, Fields: value}, nil
	case "river_state":
		var value EnvironmentalFields
		if err := json.Unmarshal(fields, &value); err != nil {
			return nil, fmt.Errorf("decode river_state fields: %w", err)
		}
		return RiverState{Envelope: envelope, Fields: value}, nil
	default:
		return UnknownEvent{Envelope: envelope}, nil
	}
}
