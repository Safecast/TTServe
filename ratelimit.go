// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Rejection of measurements that are uploaded far more frequently than the
// device is actually moving.  A bGeigiecast that sits parked in one place
// re-uploading every few seconds contributes nothing but load and clutter, so
// rather than ingesting, recording, or routing those measurements we reject
// them outright at the door with an HTTP rate-limiting status.
package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	ttdata "github.com/Safecast/safecast-go"
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

// How long an unheard-from device is kept in the in-memory cache below
const rateLimitCacheExpiration = 24 * time.Hour

// The last measurement that this instance accepted from a device.  The device
// status file is the shared and durable record of that same thing, however it is
// written asynchronously after a deliberate delay of up to 30s (see
// WriteDeviceStatus), and so for up to 30s after an upload it still describes the
// PRIOR measurement.  Without this cache in front of it, a device uploading every
// few seconds would slip several measurements through each window.
type rateLimitEntry struct {
	capturedAt time.Time
	hasLoc     bool
	lat        float64
	lon        float64
	accepted   time.Time
}

var rateLimitLock sync.Mutex
var rateLimitCache = map[string]rateLimitEntry{}

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
	return fmt.Sprintf("maximum of 1 update every %s from a given location (+- %dm)", window, RateLimitMeters)
}

// RateLimitedDeviceClass returns true if measurements from this class of device
// are subject to rate limiting.  This covers "geigiecast" and "geigiecast-zen",
// and is written as a prefix test so that it covers future geigiecast variants.
func RateLimitedDeviceClass(deviceClass string) bool {
	return strings.HasPrefix(deviceClass, "geigiecast")
}

// RateLimitExceeded determines whether a measurement must be rejected because the
// device already gave us a measurement from essentially the same place less than
// RateLimitSeconds ago.  When it returns true the caller must reject the
// measurement outright, without ingesting, recording, or routing it anywhere.
func RateLimitExceeded(deviceUID string, deviceClass string, capturedAt string, lat *float64, lon *float64) (exceeded bool, retryAfterSecs int) {

	// Only certain classes of device are rate-limited
	if !RateLimitedDeviceClass(deviceClass) {
		return false, 0
	}

	// A measurement we can't place in time can't be rate-limited
	when, ok := rateLimitParseTime(capturedAt)
	if !ok {
		if rateLimitDebug {
			fmt.Printf("%s ratelimit: %s has an unparseable captured_at of '%s'\n", LogTime(), deviceUID, capturedAt)
		}
		return false, 0
	}

	// Find the last measurement that was accepted from this device.  If there
	// isn't one, there is nothing to rate-limit against.
	refWhen, refLat, refLon, refHasLoc, found := rateLimitReference(deviceUID)
	if !found {
		return false, 0
	}

	// Reject only if this measurement was captured within the window of the last
	// accepted one.  The comparison is absolute so that a device replaying an
	// older measurement, or one whose clock has stepped backward, is caught too.
	elapsed := when.Sub(refWhen)
	if elapsed < 0 {
		elapsed = -elapsed
	}
	if elapsed >= RateLimitSeconds*time.Second {
		return false, 0
	}

	// ... and only if the device hasn't moved.  A measurement with no GPS fix,
	// or a reference with no GPS fix, counts as not having moved, because there
	// is no evidence that it did.
	newLat, newLon, newHasLoc := rateLimitLoc(lat, lon)
	if refHasLoc && newHasLoc {
		if rateLimitDistanceMeters(refLat, refLon, newLat, newLon) >= RateLimitMeters {
			return false, 0
		}
	}

	// Tell the device roughly when it would be worth trying again
	retryAfterSecs = int(math.Ceil((RateLimitSeconds*time.Second - elapsed).Seconds()))
	if retryAfterSecs < 1 {
		retryAfterSecs = 1
	}

	return true, retryAfterSecs

}

// RateLimitAccepted records a measurement that has just been accepted, making it
// the reference against which the next measurement from that device is judged.
func RateLimitAccepted(sd ttdata.SafecastData) {

	if !RateLimitedDeviceClass(sd.DeviceClass) || sd.DeviceUID == "" || sd.CapturedAt == nil {
		return
	}

	when, ok := rateLimitParseTime(*sd.CapturedAt)
	if !ok {
		return
	}

	entry := rateLimitEntry{capturedAt: when, accepted: time.Now()}
	if sd.Loc != nil {
		entry.lat, entry.lon, entry.hasLoc = rateLimitLoc(sd.Loc.Lat, sd.Loc.Lon)
	}

	rateLimitLock.Lock()
	rateLimitCache[sd.DeviceUID] = entry

	// Discard devices we haven't heard from in a very long time, so that the
	// cache can't grow without bound.  There are only ever hundreds of entries,
	// so a full sweep on each accepted measurement costs nothing.
	for uid, e := range rateLimitCache {
		if time.Since(e.accepted) > rateLimitCacheExpiration {
			delete(rateLimitCache, uid)
		}
	}
	rateLimitLock.Unlock()

}

// Fetch the last measurement accepted for a device, preferring whichever of the
// shared device status file and our own in-memory record is more recent
func rateLimitReference(deviceUID string) (when time.Time, lat float64, lon float64, hasLoc bool, found bool) {

	// The device status file is the record shared by all TTSERVE instances, and
	// it survives a restart of any of them
	isAvail, isReset, ds := ReadDeviceStatus(deviceUID)
	if isAvail && !isReset && ds.CapturedAt != nil {
		t, ok := rateLimitParseTime(*ds.CapturedAt)
		if ok {
			when = t
			found = true
			if ds.Loc != nil {
				lat, lon, hasLoc = rateLimitLoc(ds.Loc.Lat, ds.Loc.Lon)
			}
		}
	}

	// That file lags what this instance has accepted by up to 30s, so use our
	// own record of it whenever it is the newer of the two
	rateLimitLock.Lock()
	entry, present := rateLimitCache[deviceUID]
	rateLimitLock.Unlock()
	if present && (!found || entry.capturedAt.After(when)) {
		when = entry.capturedAt
		lat = entry.lat
		lon = entry.lon
		hasLoc = entry.hasLoc
		found = true
	}

	return

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
