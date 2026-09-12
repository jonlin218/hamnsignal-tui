package main

import (
	"strconv"
	"testing"
)

func TestParseVisualTraffic(t *testing.T) {
	for _, value := range []string{"0", "0.20", "1"} {
		fixture, err := parseVisualTraffic(value)
		if err != nil || fixture.TrafficPressure == nil {
			t.Fatalf("valid traffic fixture %q failed: fixture=%#v err=%v", value, fixture, err)
		}
	}
	for _, value := range []string{"traffic", "NaN", "-0.01", "1.01"} {
		if _, err := parseVisualTraffic(value); err == nil {
			t.Fatalf("invalid traffic fixture %q succeeded", value)
		}
	}
}

func TestParseVisualRain(t *testing.T) {
	for _, value := range []string{"0", "0.20", "0.50", "0.80", "1", "10"} {
		precipitation, err := parseVisualRain(value)
		if err != nil || precipitation == nil {
			t.Fatalf("valid rain fixture %q failed: precipitation=%v err=%v", value, precipitation, err)
		}
	}
	for _, value := range []string{"rain", "NaN", "Inf", "-0.01", "10.01"} {
		if _, err := parseVisualRain(value); err == nil {
			t.Fatalf("invalid rain fixture %q succeeded", value)
		}
	}
}

func TestParseVisualRadiation(t *testing.T) {
	for _, value := range []string{"0", ".15", ".50", ".85", ".999", "1", "1.0"} {
		radiation, err := parseVisualRadiation(value)
		if err != nil || radiation == nil {
			t.Fatalf("valid radiation fixture %q failed: %v", value, err)
		}
	}
	for _, value := range []string{"radiation", "NaN", "Inf", "-0.01", "1.01"} {
		if _, err := parseVisualRadiation(value); err == nil {
			t.Fatalf("invalid radiation fixture %q succeeded", value)
		}
	}
}

func TestParseOptionsPreservesExactRadiationFixtureBounds(t *testing.T) {
	for _, value := range []string{"0", ".999", "1", "1.0"} {
		options, err := parseOptions([]string{"--visual-radiation=" + value})
		if err != nil || options.fixture.Radiation == nil {
			t.Fatalf("radiation fixture %q was not preserved: options=%#v err=%v", value, options, err)
		}
		want, _ := strconv.ParseFloat(value, 64)
		if got := *options.fixture.Radiation; got != want {
			t.Fatalf("radiation fixture %q changed in options: got=%v want=%v", value, got, want)
		}
	}
	if _, err := parseOptions([]string{"--visual-radiation=1.000001"}); err == nil {
		t.Fatal("out-of-range radiation fixture was accepted")
	}
}

func TestParseOptionsComposesTrafficFixtureAndNoAudio(t *testing.T) {
	options, err := parseOptions([]string{"--no-audio", "--visual-traffic=0.50", "--visual-rain=0.80", "--visual-motorik-click"})
	if err != nil || !options.noAudio || !options.fixture.MotorikClick || options.fixture.TrafficPressure == nil || *options.fixture.TrafficPressure != .5 || options.fixture.Precipitation == nil || *options.fixture.Precipitation != .8 {
		t.Fatalf("unexpected options: %#v, %v", options, err)
	}
	ordinary, err := parseOptions(nil)
	if err != nil || ordinary.noAudio || ordinary.events || ordinary.fixture.TrafficPressure != nil || ordinary.fixture.Precipitation != nil || ordinary.fixture.MotorikClick {
		t.Fatalf("ordinary startup changed: %#v, %v", ordinary, err)
	}
	events, err := parseOptions([]string{"--events", "--visual-rain=.5", "--visual-motorik-click"})
	if err != nil || !events.events || !events.fixture.MotorikClick || events.fixture.Precipitation == nil {
		t.Fatalf("event-mode click parsing changed: %#v, %v", events, err)
	}
}
