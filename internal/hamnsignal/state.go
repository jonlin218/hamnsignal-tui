package hamnsignal

import "sync"

// State is the small renderer-independent representation of the live stream.
type State struct {
	mu          sync.RWMutex
	Connected   bool
	Voices      map[int64]Voice
	Semantic    map[string]any
	Phrase      map[string]any
	Weather     map[string]EnvironmentalValue
	River       map[string]EnvironmentalValue
	LastArrival *Arrival
}

type StateSnapshot struct {
	Connected   bool
	Voices      map[int64]Voice
	Semantic    map[string]any
	Phrase      map[string]any
	Weather     map[string]EnvironmentalValue
	River       map[string]EnvironmentalValue
	LastArrival *Arrival
}

type Voice struct {
	Fields    VoiceFields
	StartedAt int64
	Released  bool
	EndedAt   int64
}

type Arrival struct {
	Fields ArrivalFields
	At     int64
}

type EnvironmentalValue struct {
	Fields EnvironmentalFields
	At     int64
}

func NewState() *State {
	return &State{
		Voices: map[int64]Voice{}, Semantic: map[string]any{}, Phrase: map[string]any{},
		Weather: map[string]EnvironmentalValue{}, River: map[string]EnvironmentalValue{},
	}
}

func (s *State) SetConnected(connected bool) {
	s.mu.Lock()
	s.Connected = connected
	s.mu.Unlock()
}

// Snapshot returns a read-only-by-convention copy suitable for renderers.
// Network callbacks may continue updating the live state concurrently.
func (s *State) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := StateSnapshot{Connected: s.Connected, Voices: map[int64]Voice{}, Semantic: map[string]any{}, Phrase: map[string]any{}, Weather: map[string]EnvironmentalValue{}, River: map[string]EnvironmentalValue{}}
	for key, value := range s.Voices {
		snapshot.Voices[key] = value
	}
	for key, value := range s.Semantic {
		snapshot.Semantic[key] = value
	}
	for key, value := range s.Phrase {
		snapshot.Phrase[key] = value
	}
	for key, value := range s.Weather {
		snapshot.Weather[key] = value
	}
	for key, value := range s.River {
		snapshot.River[key] = value
	}
	if s.LastArrival != nil {
		arrival := *s.LastArrival
		snapshot.LastArrival = &arrival
	}
	return snapshot
}

// Apply changes state for known events. Unknown events intentionally have no
// state effect, allowing relay protocol additions without client failure.
func (s *State) Apply(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMaps()
	switch e := event.(type) {
	case VoiceStart:
		if e.Fields.NodeID != nil {
			s.Voices[int64(*e.Fields.NodeID)] = Voice{Fields: e.Fields, StartedAt: e.TimeUnixMS}
		}
	case VoiceEnd:
		if e.Fields.NodeID != nil {
			if voice, ok := s.Voices[int64(*e.Fields.NodeID)]; ok {
				voice.Released, voice.EndedAt = true, e.TimeUnixMS
				s.Voices[int64(*e.Fields.NodeID)] = voice
			}
		}
	case StateUpdate:
		if e.Fields.Key == "" {
			return
		}
		switch e.Fields.Scope {
		case "semantic":
			s.Semantic[e.Fields.Key] = e.Fields.Value
		case "phrase":
			s.Phrase[e.Fields.Key] = e.Fields.Value
		}
	case LiveArrival:
		s.LastArrival = &Arrival{Fields: e.Fields, At: e.TimeUnixMS}
	case WeatherState:
		if e.Fields.Key != nil {
			s.Weather[*e.Fields.Key] = EnvironmentalValue{Fields: e.Fields, At: e.TimeUnixMS}
		}
	case RiverState:
		if e.Fields.Key != nil {
			s.River[*e.Fields.Key] = EnvironmentalValue{Fields: e.Fields, At: e.TimeUnixMS}
		}
	}
}

func (s *State) ensureMaps() {
	if s.Voices == nil {
		s.Voices = map[int64]Voice{}
	}
	if s.Semantic == nil {
		s.Semantic = map[string]any{}
	}
	if s.Phrase == nil {
		s.Phrase = map[string]any{}
	}
	if s.Weather == nil {
		s.Weather = map[string]EnvironmentalValue{}
	}
	if s.River == nil {
		s.River = map[string]EnvironmentalValue{}
	}
}
