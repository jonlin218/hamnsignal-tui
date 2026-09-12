package visual

// VisualFixture contains optional development-only renderer overrides. It is
// intentionally separate from Hamnsignal protocol state so DATA remains a
// truthful reading of the live relay.
type VisualFixture struct {
	TrafficPressure *float64
	Precipitation   *float64
	Radiation       *float64
	MotorikClick    bool
}

func (f VisualFixture) clone() VisualFixture {
	clone := VisualFixture{MotorikClick: f.MotorikClick}
	if f.TrafficPressure != nil {
		pressure := *f.TrafficPressure
		clone.TrafficPressure = &pressure
	}
	if f.Precipitation != nil {
		precipitation := *f.Precipitation
		clone.Precipitation = &precipitation
	}
	if f.Radiation != nil {
		radiation := *f.Radiation
		clone.Radiation = &radiation
	}
	return clone
}
