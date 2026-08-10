// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// SafecastDirectory() reads os.Args[1], and exits the process if it is empty, so
// point it at a scratch directory for the duration of the tests.  The rate
// limiter reads device status files out of it, and accept() below writes them.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ttserve-ratelimit")
	if err != nil {
		fmt.Printf("can't create scratch directory: %s\n", err)
		os.Exit(1)
	}
	if err = os.Mkdir(dir+TTDeviceStatusPath, 0777); err != nil {
		fmt.Printf("can't create device status directory: %s\n", err)
		os.Exit(1)
	}

	// Consume the -test.* flags before overwriting the args they arrived in,
	// because m.Run() only parses them if nobody else already has
	flag.Parse()
	os.Args = []string{os.Args[0], dir}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// The captured_at of a measurement, offset from a fixed base
func at(offsetSecs int) string {
	base := time.Date(2026, 8, 10, 13, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetSecs) * time.Second).Format("2006-01-02T15:04:05Z")
}

// A latitude that is exactly the given number of meters due north of baseLat.
// Due north, the haversine below reduces to distance = earthRadius * deltaPhi.
const baseLat = 34.48255
const baseLon = 136.16325

func latMetersNorth(meters float64) float64 {
	return baseLat + (meters/6371008.8)*180/math.Pi
}

// The CPM that accept() records and that v1() reports, so that by default a
// probe reads exactly the same as the reference and is not treated as a spike
const baseCPM = 47.0

// Record an accepted measurement the way the ingestion pipeline eventually does,
// by writing the device status file that rateLimitReference reads.  This is
// WriteDeviceStatus without its merge of every other field, which these tests do
// not care about.
func accept(deviceUID string, deviceClass string, capturedAt string, lat *float64, lon *float64) {
	acceptCPM(deviceUID, deviceClass, capturedAt, lat, lon, baseCPM)
}

func acceptCPM(deviceUID string, deviceClass string, capturedAt string, lat *float64, lon *float64, cpm float64) {
	var ds DeviceStatus
	ds.DeviceUID = deviceUID
	ds.DeviceClass = deviceClass
	ds.CapturedAt = &capturedAt
	if lat != nil && lon != nil {
		ds.Loc = &ttdata.Loc{Lat: lat, Lon: lon}
	}
	ds.Lnd = &ttdata.Lnd{U7318: &cpm}
	contents, err := json.Marshal(ds)
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(GetDeviceStatusFilePath(deviceUID), contents, 0666); err != nil {
		panic(err)
	}
}

// An inbound V1 measurement, as SafecastV1Decode would have produced it
func v1(capturedAt string, lat *float64, lon *float64) *SafecastDataV1 {
	return v1cpm(capturedAt, lat, lon, baseCPM)
}

func v1cpm(capturedAt string, lat *float64, lon *float64, cpm float64) *SafecastDataV1 {
	unit := "cpm"
	return &SafecastDataV1{CapturedAt: &capturedAt, Latitude: lat, Longitude: lon, Unit: &unit, Value: &cpm}
}

func f(v float64) *float64 {
	return &v
}

func TestRateLimitedDeviceClass(t *testing.T) {
	limited := []string{"geigiecast", "geigiecast-zen"}
	for _, class := range limited {
		if !RateLimitedDeviceClass(class) {
			t.Errorf("%s should be rate-limited", class)
		}
	}
	unlimited := []string{"pointcast", "safecast-air", "ngeigie", "", "product:com.blues.radnote"}
	for _, class := range unlimited {
		if RateLimitedDeviceClass(class) {
			t.Errorf("%s should not be rate-limited", class)
		}
	}
}

func TestRateLimitFirstMeasurementIsAccepted(t *testing.T) {
	uid := "geigiecast:60001"
	exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(0), f(baseLat), f(baseLon)))
	if exceeded {
		t.Fatal("the first measurement from a device must be accepted")
	}
}

func TestRateLimitParkedDeviceIsRejected(t *testing.T) {
	uid := "geigiecast:60002"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	exceeded, retryAfter := RateLimitExceeded(uid, "geigiecast", v1(at(10), f(baseLat), f(baseLon)))
	if !exceeded {
		t.Fatal("a measurement 10s later from the same spot must be rejected")
	}
	if retryAfter != RateLimitSeconds-10 {
		t.Errorf("Retry-After was %d, expected %d", retryAfter, RateLimitSeconds-10)
	}
}

func TestRateLimitWindowBoundary(t *testing.T) {
	uid := "geigiecast-zen:65002"
	accept(uid, "geigiecast-zen", at(0), f(baseLat), f(baseLon))

	// Less than RateLimitSeconds after the accepted measurement: rejected
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", v1(at(RateLimitSeconds-1), f(baseLat), f(baseLon))); !exceeded {
		t.Errorf("%ds after must be rejected", RateLimitSeconds-1)
	}

	// Exactly RateLimitSeconds after: accepted, since it is not LESS than the window
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", v1(at(RateLimitSeconds), f(baseLat), f(baseLon))); exceeded {
		t.Errorf("exactly %ds after must be accepted", RateLimitSeconds)
	}
}

func TestRateLimitDistanceBoundary(t *testing.T) {
	uid := "geigiecast:60003"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// Well inside the radius and inside the window: rejected
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(10), f(latMetersNorth(RateLimitMeters-1)), f(baseLon))); !exceeded {
		t.Errorf("%dm away must be rejected", RateLimitMeters-1)
	}

	// Outside the radius, even well inside the window: accepted
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(10), f(latMetersNorth(RateLimitMeters+1)), f(baseLon))); exceeded {
		t.Errorf("%dm away must be accepted", RateLimitMeters+1)
	}
}

func TestRateLimitMovingDeviceIsAccepted(t *testing.T) {
	uid := "geigiecast:60004"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// A device driving a survey route reports every 10s from a new place each time
	for i := 1; i <= 10; i++ {
		capturedAt := at(i * 10)
		lat := latMetersNorth(float64(i) * 150)
		if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(capturedAt, f(lat), f(baseLon))); exceeded {
			t.Fatalf("a moving device must never be rejected (sample %d)", i)
		}
		accept(uid, "geigiecast", capturedAt, f(lat), f(baseLon))
	}
}

func TestRateLimitOneSamplePerWindowWhileParked(t *testing.T) {
	uid := "geigiecast-zen:65128"
	accepted := 0

	// A parked device uploading every 10s for an hour
	for secs := 0; secs <= 3600; secs += 10 {
		capturedAt := at(secs)
		exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", v1(capturedAt, f(baseLat), f(baseLon)))
		if !exceeded {
			accepted++
			accept(uid, "geigiecast-zen", capturedAt, f(baseLat), f(baseLon))
		}
	}

	// One at t=0, then one every RateLimitSeconds thereafter
	expected := 1 + 3600/RateLimitSeconds
	if accepted != expected {
		t.Errorf("accepted %d measurements in an hour, expected %d", accepted, expected)
	}
}

// The first whole CPM that clears, and the last that does not clear, the spike
// threshold above a given reference
func justAboveSpike(ref float64) float64 { return math.Floor(RateLimitSpikeThreshold(ref)) + 1 }
func justBelowSpike(ref float64) float64 { return math.Floor(RateLimitSpikeThreshold(ref)) }

func TestRateLimitSpikeThresholdScalesWithTheReading(t *testing.T) {
	// Poisson noise grows as sqrt, so the bar must too.  A margin generous at
	// background has to be proportionally generous in a hot spot.
	for _, ref := range []float64{20, 37, 100, 500, 2000} {
		margin := RateLimitSpikeThreshold(ref) - ref
		sigmas := margin / math.Sqrt(ref)
		if math.Abs(sigmas-RateLimitSpikeSigmas) > 0.001 {
			t.Errorf("at %.0f CPM the margin is %.1f sigma, expected %.1f", ref, sigmas, RateLimitSpikeSigmas)
		}
	}

	// Going from nothing to something is always a spike
	if RateLimitSpikeThreshold(0) != 0 {
		t.Error("a zero reference must admit anything above zero")
	}
}

func TestRateLimitSpikeAlwaysGoesThrough(t *testing.T) {
	uid := "geigiecast:60008"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// Same place, one second later, but the reading has risen past the threshold
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), justAboveSpike(baseCPM))); exceeded {
		t.Error("a reading above the spike threshold must never be rejected")
	}

	// A dramatic one, likewise
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), baseCPM*100)); exceeded {
		t.Error("a large spike must never be rejected")
	}

	// A rise that is only counting noise is not a spike
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), justBelowSpike(baseCPM))); !exceeded {
		t.Error("a rise within the noise must be rejected inside the window")
	}

	// Nor is an unchanged or falling reading
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), baseCPM)); !exceeded {
		t.Error("an unchanged reading must be rejected inside the window")
	}
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), baseCPM-1)); !exceeded {
		t.Error("a lower reading must be rejected inside the window")
	}
}

func TestRateLimitSpikeResetsTheWindow(t *testing.T) {
	uid := "geigiecast:60009"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// A spike at t=1 is accepted, and becomes the new reference
	spike := justAboveSpike(baseCPM)
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(1), f(baseLat), f(baseLon), spike)); exceeded {
		t.Fatal("the spike must be accepted")
	}
	acceptCPM(uid, "geigiecast", at(1), f(baseLat), f(baseLon), spike)

	// So the window now runs from t=1, not from t=0
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(RateLimitSeconds), f(baseLat), f(baseLon), baseCPM)); !exceeded {
		t.Error("the window must restart from the spike, so t=300 is still inside it")
	}
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(RateLimitSeconds+1), f(baseLat), f(baseLon), baseCPM)); exceeded {
		t.Error("t=301 is outside the window that restarted at t=1")
	}

	// And the bar for the next spike has risen with it
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(2), f(baseLat), f(baseLon), justBelowSpike(spike))); !exceeded {
		t.Error("a reading below the new threshold must be rejected")
	}
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(2), f(baseLat), f(baseLon), justAboveSpike(spike))); exceeded {
		t.Error("a reading above the new threshold must be accepted")
	}
}

func TestRateLimitClimbIntoAHotAreaIsFullyReported(t *testing.T) {
	uid := "geigiecast:60010"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// A device driving into a hot area, uploading every second.  Every step of
	// the climb must be reported even though it never moves.
	cpm := baseCPM
	for i := 1; i <= 12; i++ {
		cpm = justAboveSpike(cpm)
		if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(i), f(baseLat), f(baseLon), cpm)); exceeded {
			t.Fatalf("step %d of a climb was dropped at %.0f CPM", i, cpm)
		}
		acceptCPM(uid, "geigiecast", at(i), f(baseLat), f(baseLon), cpm)
	}

	// Twelve seconds of minimum-sized steps is enough to go from background to
	// well into hot-spot territory, with every step reported
	if cpm < 500 {
		t.Errorf("the climb only reached %.0f CPM", cpm)
	}

	// Once it plateaus, rate limiting resumes even though it is now sitting hot
	for i := 13; i <= 22; i++ {
		if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(i), f(baseLat), f(baseLon), cpm)); !exceeded {
			t.Fatalf("a plateau at %.0f CPM must be rate-limited (sample %d)", cpm, i)
		}
	}

	// And at that elevated level the bar is proportionally higher, so ordinary
	// counting noise on top of it does not get through
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1cpm(at(23), f(baseLat), f(baseLon), cpm+math.Sqrt(cpm))); !exceeded {
		t.Error("a one-sigma wobble in a hot spot must still be rate-limited")
	}
}

func TestRateLimitNonCPMUnitIsNotASpike(t *testing.T) {
	uid := "geigiecast:60011"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// A "status" measurement carries a temperature in value, not a count, so it
	// must not be mistaken for a rising reading
	unit := "status"
	value := baseCPM + 1000
	sd := &SafecastDataV1{CapturedAt: func() *string { s := at(10); return &s }(),
		Latitude: f(baseLat), Longitude: f(baseLon), Unit: &unit, Value: &value}
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", sd); !exceeded {
		t.Error("a non-CPM unit must not bypass the limit")
	}
}

func TestRateLimitMissingLocationCountsAsSameLocation(t *testing.T) {
	uid := "geigiecast:60005"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// No fix on the incoming measurement
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(10), nil, nil)); !exceeded {
		t.Error("a measurement with no location must be rejected inside the window")
	}

	// A 0,0 "no fix" is treated the same way as an absent one
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(10), f(0), f(0))); !exceeded {
		t.Error("a measurement located at 0,0 must be rejected inside the window")
	}

	// And likewise when it is the reference that has no location
	uid2 := "geigiecast:60006"
	accept(uid2, "geigiecast", at(0), nil, nil)
	if exceeded, _ := RateLimitExceeded(uid2, "geigiecast", v1(at(10), f(baseLat), f(baseLon))); !exceeded {
		t.Error("a reference with no location must still rate-limit inside the window")
	}
}

func TestRateLimitReplayedMeasurementIsRejected(t *testing.T) {
	uid := "geigiecast:60007"
	accept(uid, "geigiecast", at(600), f(baseLat), f(baseLon))

	// An older measurement arriving late, still inside the window
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(590), f(baseLat), f(baseLon))); !exceeded {
		t.Error("a replayed measurement inside the window must be rejected")
	}

	// An older measurement from well outside the window is not our concern
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", v1(at(0), f(baseLat), f(baseLon))); exceeded {
		t.Error("a measurement outside the window must be accepted")
	}
}

func TestRateLimitOtherDeviceClassesAreUntouched(t *testing.T) {
	uid := "pointcast:10001"
	accept(uid, "pointcast", at(0), f(baseLat), f(baseLon))
	if exceeded, _ := RateLimitExceeded(uid, "pointcast", v1(at(1), f(baseLat), f(baseLon))); exceeded {
		t.Error("pointcast must never be rate-limited")
	}
}

func TestRateLimitPolicy(t *testing.T) {
	// The wording is what a rejected device's operator reads, so pin it
	want := "maximum of 1 update every 5 min from a given location (+- 100m) unless the reading rises sharply"
	if got := RateLimitPolicy(); got != want {
		t.Errorf("policy reads %q, expected %q", got, want)
	}
}

func TestRateLimitParseTime(t *testing.T) {
	valid := map[string]time.Time{
		"2026-08-10T13:07:47Z":      time.Date(2026, 8, 10, 13, 7, 47, 0, time.UTC),
		"2017-9-7T2:3:4Z":           time.Date(2017, 9, 7, 2, 3, 4, 0, time.UTC),
		"2026-08-10T13:07:47.500Z":  time.Date(2026, 8, 10, 13, 7, 47, 500000000, time.UTC),
		"2026-08-10T22:07:47+09:00": time.Date(2026, 8, 10, 13, 7, 47, 0, time.UTC),
		"2017-9-7T11:3:4+09:00":     time.Date(2017, 9, 7, 2, 3, 4, 0, time.UTC),
		"2026-08-10T22:07:47+0900":  time.Date(2026, 8, 10, 13, 7, 47, 0, time.UTC),
		"2026-08-10 13:07:47Z":      time.Date(2026, 8, 10, 13, 7, 47, 0, time.UTC),
		"  2026-08-10T13:07:47Z  ":  time.Date(2026, 8, 10, 13, 7, 47, 0, time.UTC),
	}
	for s, want := range valid {
		got, ok := rateLimitParseTime(s)
		if !ok {
			t.Errorf("could not parse %s", s)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("parsed %s as %v, expected %v", s, got, want)
		}
	}
	for _, s := range []string{"", "not a time", "1502726400"} {
		if _, ok := rateLimitParseTime(s); ok {
			t.Errorf("%s should not have parsed", s)
		}
	}
}

func TestRateLimitDistanceMeters(t *testing.T) {
	// Same point
	if d := rateLimitDistanceMeters(baseLat, baseLon, baseLat, baseLon); d != 0 {
		t.Errorf("distance to the same point was %f, expected 0", d)
	}

	// A known due-north offset
	if d := rateLimitDistanceMeters(baseLat, baseLon, latMetersNorth(100), baseLon); math.Abs(d-100) > 0.01 {
		t.Errorf("distance was %f, expected 100", d)
	}

	// The two devices in the sample logs, roughly 8,700km apart
	d := rateLimitDistanceMeters(49.608237, 20.819761, 34.48255, 136.16325)
	if math.Abs(d-8.708e6) > 1.0e4 {
		t.Errorf("Poland to Japan measured %f meters, expected roughly 8.708e6", d)
	}
}
