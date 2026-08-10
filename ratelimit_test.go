// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// SafecastDirectory() reads os.Args[1], and exits the process if it is empty, so
// point it at an empty scratch directory for the duration of the tests.  With no
// device status files present, the last-accepted reference comes entirely from
// the in-memory cache, which is what these tests exercise.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ttserve-ratelimit")
	if err != nil {
		fmt.Printf("can't create scratch directory: %s\n", err)
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

// Record an accepted measurement, as the ingestion path does
func accept(deviceUID string, deviceClass string, capturedAt string, lat *float64, lon *float64) {
	sd := ttdata.SafecastData{DeviceUID: deviceUID, DeviceClass: deviceClass, CapturedAt: &capturedAt}
	if lat != nil && lon != nil {
		sd.Loc = &ttdata.Loc{Lat: lat, Lon: lon}
	}
	RateLimitAccepted(sd)
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
	exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(0), f(baseLat), f(baseLon))
	if exceeded {
		t.Fatal("the first measurement from a device must be accepted")
	}
}

func TestRateLimitParkedDeviceIsRejected(t *testing.T) {
	uid := "geigiecast:60002"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	exceeded, retryAfter := RateLimitExceeded(uid, "geigiecast", at(10), f(baseLat), f(baseLon))
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
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", at(RateLimitSeconds-1), f(baseLat), f(baseLon)); !exceeded {
		t.Errorf("%ds after must be rejected", RateLimitSeconds-1)
	}

	// Exactly RateLimitSeconds after: accepted, since it is not LESS than the window
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", at(RateLimitSeconds), f(baseLat), f(baseLon)); exceeded {
		t.Errorf("exactly %ds after must be accepted", RateLimitSeconds)
	}
}

func TestRateLimitDistanceBoundary(t *testing.T) {
	uid := "geigiecast:60003"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// Well inside the radius and inside the window: rejected
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(10), f(latMetersNorth(RateLimitMeters-1)), f(baseLon)); !exceeded {
		t.Errorf("%dm away must be rejected", RateLimitMeters-1)
	}

	// Outside the radius, even well inside the window: accepted
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(10), f(latMetersNorth(RateLimitMeters+1)), f(baseLon)); exceeded {
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
		if exceeded, _ := RateLimitExceeded(uid, "geigiecast", capturedAt, f(lat), f(baseLon)); exceeded {
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
		exceeded, _ := RateLimitExceeded(uid, "geigiecast-zen", capturedAt, f(baseLat), f(baseLon))
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

func TestRateLimitMissingLocationCountsAsSameLocation(t *testing.T) {
	uid := "geigiecast:60005"
	accept(uid, "geigiecast", at(0), f(baseLat), f(baseLon))

	// No fix on the incoming measurement
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(10), nil, nil); !exceeded {
		t.Error("a measurement with no location must be rejected inside the window")
	}

	// A 0,0 "no fix" is treated the same way as an absent one
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(10), f(0), f(0)); !exceeded {
		t.Error("a measurement located at 0,0 must be rejected inside the window")
	}

	// And likewise when it is the reference that has no location
	uid2 := "geigiecast:60006"
	accept(uid2, "geigiecast", at(0), nil, nil)
	if exceeded, _ := RateLimitExceeded(uid2, "geigiecast", at(10), f(baseLat), f(baseLon)); !exceeded {
		t.Error("a reference with no location must still rate-limit inside the window")
	}
}

func TestRateLimitReplayedMeasurementIsRejected(t *testing.T) {
	uid := "geigiecast:60007"
	accept(uid, "geigiecast", at(600), f(baseLat), f(baseLon))

	// An older measurement arriving late, still inside the window
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(590), f(baseLat), f(baseLon)); !exceeded {
		t.Error("a replayed measurement inside the window must be rejected")
	}

	// An older measurement from well outside the window is not our concern
	if exceeded, _ := RateLimitExceeded(uid, "geigiecast", at(0), f(baseLat), f(baseLon)); exceeded {
		t.Error("a measurement outside the window must be accepted")
	}
}

func TestRateLimitOtherDeviceClassesAreUntouched(t *testing.T) {
	uid := "pointcast:10001"
	accept(uid, "pointcast", at(0), f(baseLat), f(baseLon))
	if exceeded, _ := RateLimitExceeded(uid, "pointcast", at(1), f(baseLat), f(baseLon)); exceeded {
		t.Error("pointcast must never be rate-limited")
	}
}

func TestRateLimitPolicy(t *testing.T) {
	// The wording is what a rejected device's operator reads, so pin it
	want := "maximum of 1 update every 5 min from a given location (+- 100m)"
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
