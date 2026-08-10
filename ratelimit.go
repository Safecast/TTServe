// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Rejection of measurements that are uploaded far more frequently than the
// device is actually moving.  A bGeigiecast that sits parked in one place
// re-uploading every few seconds contributes nothing but load and clutter, so
// rather than ingesting, recording, or routing those measurements we reject
// them outright at the door with an HTTP rate-limiting status.  A reading that
// has RISEN meaningfully above the last one we accepted is always let through,
// because losing a spike would defeat the entire purpose of the network.
package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Debugging
const rateLimitDebug bool = false

// RateLimitSeconds is the minimum number of seconds that must elapse between the
// captured_at of one accepted measurement and the captured_at of the next, for a
// device that has not moved out of RateLimitMeters.
const RateLimitSeconds = 300

// RateLimitMeters is how far a device must travel from the location of its last
// accepted measurement in order to be exempted from RateLimitSeconds.
const RateLimitMeters = 100

// RateLimitSpikeSigmas is how far above the last accepted reading, measured in
// standard deviations of that reading, a measurement must be before it counts as
// a spike and bypasses the limit entirely.
//
// Radioactive counts are Poisson, so a reading of N CPM has a standard deviation
// of sqrt(N).  That is about 6 CPM at a typical 37 CPM background but about 22
// CPM for a device sitting somewhere hot at 500, which is why this scales rather
// than being a fixed number of CPM: a fixed margin generous enough for background
// sits inside the noise in a hot spot, and the device floods us again exactly
// where we can least afford it.
//
// Three sigma costs almost nothing in sensitivity, because a device uploading
// once a second gets many chances: it still reports a doubling within a second or
// two, and a 1.5x elevation within ten.
const RateLimitSpikeSigmas float64 = 3

// The formats in which a captured_at may reach us.  Devices are inconsistent
// about this, and safecast-air is known to emit dates such as 2017-9-7T2:3:4Z.
// The single-digit verbs accept two digits as well, a fractional second is
// accepted whether or not the layout mentions one, and Z07:00 accepts a literal
// Z as well as an offset -- so the first of these alone covers the canonical
// form, the loose form, RFC3339 and RFC3339Nano.
var rateLimitTimeFormats = []string{
	"2006-1-2T15:4:5Z07:00",
	"2006-1-2T15:4:5Z0700",
	"2006-1-2 15:4:5Z07:00",
}

// RateLimitPolicy states the policy in words, so that whoever is looking at a
// rejection understands what it would take to satisfy us
func RateLimitPolicy() string {
	window := fmt.Sprintf("%d sec", RateLimitSeconds)
	if RateLimitSeconds%60 == 0 {
		window = fmt.Sprintf("%d min", RateLimitSeconds/60)
	}
	return fmt.Sprintf("maximum of 1 update every %s from a given location (+- %dm) unless the reading rises sharply", window, RateLimitMeters)
}

// RateLimitSpikeThreshold is the CPM a measurement must exceed, given the last
// accepted reading, to count as a spike.  A reference of zero admits anything
// above zero, since going from nothing to something is always worth reporting.
func RateLimitSpikeThreshold(refCPM float64) float64 {
	if refCPM <= 0 {
		return 0
	}
	return refCPM + RateLimitSpikeSigmas*math.Sqrt(refCPM)
}

// RateLimitedDeviceClass returns true if measurements from this class of device
// are subject to rate limiting.  This covers "geigiecast" and "geigiecast-zen",
// and is written as a prefix test so that it covers future geigiecast variants.
func RateLimitedDeviceClass(deviceClass string) bool {
	return strings.HasPrefix(deviceClass, "geigiecast")
}

// RateLimitExceeded determines whether a measurement must be rejected because the
// device already gave us a measurement from essentially the same place less than
// RateLimitSeconds ago, and this one is no higher than that one.  When it returns
// true the caller must reject the measurement outright, without ingesting,
// recording, or routing it anywhere.
func RateLimitExceeded(deviceUID string, deviceClass string, sdV1 *SafecastDataV1) (exceeded bool, retryAfterSecs int) {

	// Only certain classes of device are rate-limited
	if !RateLimitedDeviceClass(deviceClass) || sdV1 == nil || sdV1.CapturedAt == nil {
		return false, 0
	}

	// A measurement we can't place in time can't be rate-limited
	when, ok := rateLimitParseTime(*sdV1.CapturedAt)
	if !ok {
		if rateLimitDebug {
			fmt.Printf("%s ratelimit: %s has an unparseable captured_at of '%s'\n", LogTime(), deviceUID, *sdV1.CapturedAt)
		}
		return false, 0
	}

	// Find the last measurement that was accepted from this device.  If there
	// isn't one, there is nothing to rate-limit against.
	ref := rateLimitReference(deviceUID)
	if !ref.found {
		return false, 0
	}

	// Reject only if this measurement was captured within the window of the last
	// accepted one.  The comparison is absolute so that a device replaying an
	// older measurement, or one whose clock has stepped backward, is caught too.
	elapsed := when.Sub(ref.capturedAt)
	if elapsed < 0 {
		elapsed = -elapsed
	}
	if elapsed >= RateLimitSeconds*time.Second {
		return false, 0
	}

	// ... and only if the device hasn't moved.  A measurement with no GPS fix,
	// or a reference with no GPS fix, counts as not having moved, because there
	// is no evidence that it did.
	newLat, newLon, newHasLoc := rateLimitLoc(sdV1.Latitude, sdV1.Longitude)
	if ref.hasLoc && newHasLoc {
		if rateLimitDistanceMeters(ref.lat, ref.lon, newLat, newLon) >= RateLimitMeters {
			return false, 0
		}
	}

	// ... and only if the reading has not risen meaningfully above the last one.
	// A spike must never be dropped, so it goes straight through, and by being
	// accepted it becomes the reference for the next one -- meaning a device
	// climbing into a hot area reports every step of the climb.  If either
	// reading is missing we cannot tell, and fall through to rejecting.
	newCPM, newHasCPM := rateLimitCPM(sdV1)
	if ref.hasCPM && newHasCPM && newCPM > RateLimitSpikeThreshold(ref.cpm) {
		return false, 0
	}

	// Tell the device roughly when it would be worth trying again
	retryAfterSecs = int(math.Ceil((RateLimitSeconds*time.Second - elapsed).Seconds()))
	if retryAfterSecs < 1 {
		retryAfterSecs = 1
	}

	return true, retryAfterSecs

}

// The last measurement accepted from a device, as recorded in its device status
// file
type rateLimitRef struct {
	found      bool
	capturedAt time.Time
	hasLoc     bool
	lat        float64
	lon        float64
	hasCPM     bool
	cpm        float64
}

// Fetch the last measurement accepted for a device.  The device status file is
// the only state shared by every TTSERVE instance behind the load balancer, and
// it is written by the normal ingestion pipeline (SafecastLog -> WriteToLogs ->
// WriteDeviceStatus) for accepted measurements only, which is exactly the
// reference this wants.
func rateLimitReference(deviceUID string) (ref rateLimitRef) {

	isAvail, isReset, ds := ReadDeviceStatus(deviceUID)
	if !isAvail || isReset || ds.CapturedAt == nil {
		return
	}

	when, ok := rateLimitParseTime(*ds.CapturedAt)
	if !ok {
		return
	}

	ref.found = true
	ref.capturedAt = when
	if ds.Loc != nil {
		ref.lat, ref.lon, ref.hasLoc = rateLimitLoc(ds.Loc.Lat, ds.Loc.Lon)
	}

	// U7318 is the tube that reformat.go assigns to every geigiecast, so it is
	// where the CPM of the last accepted measurement will have landed
	if ds.Lnd != nil && ds.Lnd.U7318 != nil {
		ref.cpm = *ds.Lnd.U7318
		ref.hasCPM = true
	}

	return

}

// The CPM this measurement is reporting, if it is reporting one.  A geigiecast
// sends a bare unit/value pair, which reformat.go turns into the U7318 reading
// that rateLimitReference reads back.
func rateLimitCPM(sdV1 *SafecastDataV1) (float64, bool) {
	if sdV1.Unit == nil || sdV1.Value == nil {
		return 0, false
	}
	if strings.ToLower(*sdV1.Unit) != "cpm" {
		return 0, false
	}
	return *sdV1.Value, true
}

// Extract a location, treating both an absent fix and the 0,0 that devices emit
// when they have no fix as no location at all.  (WriteDeviceStatus likewise
// refuses to overwrite a good lat/lon with 0,0.)
func rateLimitLoc(lat *float64, lon *float64) (float64, float64, bool) {
	if lat == nil || lon == nil {
		return 0, 0, false
	}
	if *lat == 0 && *lon == 0 {
		return 0, 0, false
	}
	return *lat, *lon, true
}

// Parse a captured_at in whichever of the formats a device happened to use.
// Unlike every other string field, captured_at is not trimmed by the V1 decoder,
// so do it here.
func rateLimitParseTime(s string) (when time.Time, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	for _, format := range rateLimitTimeFormats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t.UTC(), true
		}
	}
	return
}

// Great-circle distance in meters between two lat/lon pairs
func rateLimitDistanceMeters(lat1 float64, lon1 float64, lat2 float64, lon2 float64) float64 {
	const earthRadiusMeters = 6371008.8
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	return 2 * earthRadiusMeters * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
