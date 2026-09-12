package hamnsignal

import (
	"encoding/json"
	"testing"
)

func TestDecodeEnvelope(t *testing.T) {
	packet := []byte(`{"time_unix_ms":1788500000000,"source":"supercollider","path":"/voice/start","type":"voice_start","args":["x"],"fields":{"nodeId":4152,"future":true}}`)
	envelope, err := DecodeEnvelope(packet)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Type != "voice_start" || envelope.TimeUnixMS != 1788500000000 || envelope.Source != "supercollider" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if len(envelope.Fields) == 0 || len(envelope.Args) == 0 {
		t.Fatal("raw fields and args should be retained")
	}
}

func TestDecodeKnownEventsAndOptionalFields(t *testing.T) {
	cases := []struct {
		name, packet string
		check        func(t *testing.T, event Event)
	}{
		{"voice start", `{"type":"voice_start","time_unix_ms":1,"fields":{"nodeId":7,"freq":164.81,"unknown":1}}`, func(t *testing.T, event Event) {
			e, ok := event.(VoiceStart)
			if !ok || e.Fields.NodeID == nil || *e.Fields.NodeID != 7 || e.Fields.Amp != nil {
				t.Fatalf("unexpected voice: %#v", event)
			}
		}},
		{"voice end", `{"type":"voice_end","fields":{"nodeId":7}}`, func(t *testing.T, event Event) {
			if _, ok := event.(VoiceEnd); !ok {
				t.Fatalf("got %T", event)
			}
		}},
		{"state", `{"type":"state_update","fields":{"scope":"semantic","key":"brightness","value":0.28}}`, func(t *testing.T, event Event) {
			e := event.(StateUpdate)
			if e.Fields.Scope != "semantic" || e.Fields.Value.(float64) != 0.28 {
				t.Fatalf("unexpected state: %#v", e.Fields)
			}
		}},
		{"arrival", `{"type":"live_arrival","fields":{"line":"3","site":"masthuggstorget"}}`, func(t *testing.T, event Event) {
			e := event.(LiveArrival)
			if e.Fields.Line == nil || *e.Fields.Line != "3" || e.Fields.Mode != nil {
				t.Fatalf("unexpected arrival: %#v", e.Fields)
			}
		}},
		{"weather", `{"type":"weather_state","fields":{"key":"temperature","raw_value":16.2}}`, func(t *testing.T, event Event) {
			e := event.(WeatherState)
			if e.Fields.Key == nil || *e.Fields.Key != "temperature" || e.Fields.NormalizedValue != nil {
				t.Fatalf("unexpected weather: %#v", e.Fields)
			}
		}},
		{"river", `{"type":"river_state"}`, func(t *testing.T, event Event) {
			e := event.(RiverState)
			if e.Fields.Key != nil {
				t.Fatal("missing key should remain optional")
			}
		}},
		{"traffic", `{"type":"traffic_state","fields":{"key":"e45_queue","label":"e45Queue","layer":"live","normalized_value":0.45,"observed_at_unix_ms":1789139152648,"raw_value":50.4,"source":"trafikverket_traffic_flow","future":true}}`, func(t *testing.T, event Event) {
			e, ok := event.(TrafficState)
			if !ok || e.Fields.Key == nil || *e.Fields.Key != "e45_queue" || e.Fields.Label == nil || *e.Fields.Label != "e45Queue" || e.Fields.RawValue == nil || *e.Fields.RawValue != 50.4 || e.Fields.NormalizedValue == nil || *e.Fields.NormalizedValue != 0.45 || e.Fields.ObservedAtUnixMS == nil || *e.Fields.ObservedAtUnixMS != 1789139152648 {
				t.Fatalf("unexpected traffic: %#v", event)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := DecodeEvent([]byte(tc.packet))
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, event)
		})
	}
}

func TestDecodeTrafficStateOptionalFieldsAndNumericStrings(t *testing.T) {
	event := mustEvent(t, `{"type":"traffic_state","fields":{"key":"e45_queue","raw_value":"53.1","normalized_value":"0","observed_at_unix_ms":"1789139152648"}}`)
	traffic, ok := event.(TrafficState)
	if !ok || traffic.Fields.Label != nil || traffic.Fields.RawValue == nil || *traffic.Fields.RawValue != 53.1 || traffic.Fields.NormalizedValue == nil || *traffic.Fields.NormalizedValue != 0 || traffic.Fields.ObservedAtUnixMS == nil || *traffic.Fields.ObservedAtUnixMS != 1789139152648 {
		t.Fatalf("unexpected traffic: %#v", event)
	}

	missing := mustEvent(t, `{"type":"traffic_state"}`).(TrafficState)
	if missing.Fields.Key != nil || missing.Fields.RawValue != nil || missing.Fields.NormalizedValue != nil {
		t.Fatalf("missing optional traffic fields should remain absent: %#v", missing.Fields)
	}
}

func TestUnknownEventAndUnknownFields(t *testing.T) {
	event, err := DecodeEvent([]byte(`{"type":"future_event","fields":{"new_field":[1,2,3]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := event.(UnknownEvent); !ok {
		t.Fatalf("got %T", event)
	}
}

func TestStateVoiceLifecycleAndEnvironmentalUpdates(t *testing.T) {
	state := NewState()
	start := mustEvent(t, `{"type":"voice_start","time_unix_ms":10,"fields":{"nodeId":9,"layer":"body"}}`)
	state.Apply(start)
	voice, ok := state.Voices[9]
	if !ok || voice.Released {
		t.Fatalf("voice was not active: %+v", state.Voices)
	}
	state.Apply(mustEvent(t, `{"type":"voice_end","time_unix_ms":20,"fields":{"nodeId":9}}`))
	voice = state.Voices[9]
	if !voice.Released || voice.EndedAt != 20 {
		t.Fatalf("voice lifecycle not retained: %+v", voice)
	}
	state.Apply(mustEvent(t, `{"type":"weather_state","time_unix_ms":30,"fields":{"key":"temperature","raw_value":16}}`))
	state.Apply(mustEvent(t, `{"type":"weather_state","time_unix_ms":31,"fields":{"key":"temperature","raw_value":17}}`))
	if got := *state.Weather["temperature"].Fields.RawValue; got != 17 {
		t.Fatalf("weather was not replaced: %v", got)
	}
	state.Apply(mustEvent(t, `{"type":"river_state","time_unix_ms":32,"fields":{"key":"river_level","normalized_value":0.555}}`))
	if _, ok := state.River["river_level"]; !ok {
		t.Fatal("river value missing")
	}
	state.Apply(mustEvent(t, `{"type":"state_update","fields":{"scope":"phrase","key":"state","value":"repose"}}`))
	if state.Phrase["state"] != "repose" {
		t.Fatal("phrase state missing")
	}
}

func TestStateTrafficUpdatesAreIsolatedAndSnapshotIsIndependent(t *testing.T) {
	state := NewState()
	state.Apply(mustEvent(t, `{"type":"weather_state","time_unix_ms":10,"fields":{"key":"temperature","raw_value":16}}`))
	state.Apply(mustEvent(t, `{"type":"river_state","time_unix_ms":11,"fields":{"key":"river_level","normalized_value":0.5}}`))
	state.Apply(mustEvent(t, `{"type":"traffic_state","time_unix_ms":12,"fields":{"key":"e45_queue","label":"e45Queue","source":"trafikverket_traffic_flow","raw_value":53.1,"normalized_value":0,"observed_at_unix_ms":1789139152648}}`))

	snapshot := state.Snapshot()
	traffic, ok := snapshot.Traffic["e45_queue"]
	if !ok || traffic.At != 12 || traffic.Fields.Label == nil || *traffic.Fields.Label != "e45Queue" || traffic.Fields.Source == nil || *traffic.Fields.Source != "trafikverket_traffic_flow" || traffic.Fields.RawValue == nil || *traffic.Fields.RawValue != 53.1 || traffic.Fields.NormalizedValue == nil || *traffic.Fields.NormalizedValue != 0 || traffic.Fields.ObservedAtUnixMS == nil || *traffic.Fields.ObservedAtUnixMS != 1789139152648 {
		t.Fatalf("traffic state missing or incorrect: %#v", snapshot.Traffic)
	}
	if len(snapshot.Weather) != 1 || len(snapshot.River) != 1 || len(snapshot.Voices) != 0 || len(snapshot.Semantic) != 0 || len(snapshot.Phrase) != 0 || snapshot.LastArrival != nil {
		t.Fatalf("traffic update changed unrelated state: %#v", snapshot)
	}

	*traffic.Fields.RawValue = 1
	delete(snapshot.Traffic, "e45_queue")
	authoritative := state.Snapshot().Traffic["e45_queue"]
	if authoritative.Fields.RawValue == nil || *authoritative.Fields.RawValue != 53.1 {
		t.Fatalf("snapshot mutation changed authoritative traffic state: %#v", authoritative)
	}
}

func TestUnknownEventDoesNotMutateTrafficState(t *testing.T) {
	state := NewState()
	state.Apply(mustEvent(t, `{"type":"traffic_state","fields":{"key":"e45_queue","normalized_value":0.45}}`))
	state.Apply(mustEvent(t, `{"type":"future_event","fields":{"key":"e45_queue","normalized_value":1}}`))
	traffic := state.Snapshot().Traffic["e45_queue"]
	if traffic.Fields.NormalizedValue == nil || *traffic.Fields.NormalizedValue != 0.45 {
		t.Fatalf("unknown event mutated traffic state: %#v", traffic)
	}
}

func TestMissingFieldsDoNotPanic(t *testing.T) {
	state := NewState()
	for _, packet := range []string{`{"type":"voice_start"}`, `{"type":"voice_end"}`, `{"type":"live_arrival"}`, `{"type":"weather_state"}`, `{"type":"river_state"}`, `{"type":"traffic_state"}`, `{"type":"state_update"}`} {
		state.Apply(mustEvent(t, packet))
	}
}

func mustEvent(t *testing.T, packet string) Event {
	t.Helper()
	event, err := DecodeEvent([]byte(packet))
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestStateUpdateValueIsJSONCompatible(t *testing.T) {
	event := mustEvent(t, `{"type":"state_update","fields":{"scope":"semantic","key":"x","value":{"nested":true}}}`).(StateUpdate)
	if _, ok := event.Fields.Value.(map[string]any); !ok {
		b, _ := json.Marshal(event.Fields.Value)
		t.Fatalf("unexpected value: %s", b)
	}
}
