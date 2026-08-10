// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	ttdata "github.com/Safecast/safecast-go"
)

// A measurement big enough that marshalling it takes a while, and with DeviceID
// and DeviceSN left empty so that WriteDeviceStatus does no spreadsheet lookup,
// and Service.Transport left nil so that it makes no ip-api.com call
func statusSample(deviceUID string, capturedAt string, cpm float64) ttdata.SafecastData {
	var sd ttdata.SafecastData
	sd.DeviceUID = deviceUID
	sd.DeviceClass = "geigiecast"
	sd.CapturedAt = &capturedAt
	lat, lon := baseLat, baseLon
	sd.Loc = &ttdata.Loc{Lat: &lat, Lon: &lon}
	sd.Lnd = &ttdata.Lnd{U7318: &cpm}
	temp, humid, press := 21.5, 44.0, 1013.2
	sd.Env = &ttdata.Env{Temp: &temp, Humid: &humid, Press: &press}
	voltage := 4.02
	sd.Bat = &ttdata.Bat{Voltage: &voltage}
	return sd
}

// Hammer one device status file with concurrent writers while concurrent readers
// look for a partially-written one.  ReadDeviceStatus reports JSON it could not
// parse by returning isReset, which is precisely the corruption that the old
// O_TRUNC-then-write could expose and that the rename cannot.
func TestWriteDeviceStatusIsAtomic(t *testing.T) {
	const uid = "geigiecast:60100"
	const writers = 8
	const writesEach = 25

	var corrupt, reads int64
	stop := make(chan struct{})

	var readersDone sync.WaitGroup
	for i := 0; i < 4; i++ {
		readersDone.Add(1)
		go func() {
			defer readersDone.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, isReset, _ := ReadDeviceStatus(uid)
				atomic.AddInt64(&reads, 1)
				if isReset {
					atomic.AddInt64(&corrupt, 1)
				}
			}
		}()
	}

	var writersDone sync.WaitGroup
	for i := 0; i < writers; i++ {
		writersDone.Add(1)
		go func(n int) {
			defer writersDone.Done()
			for j := 0; j < writesEach; j++ {
				WriteDeviceStatus(statusSample(uid, at(n*1000+j), float64(n*100+j)))
			}
		}(i)
	}
	writersDone.Wait()
	close(stop)
	readersDone.Wait()

	if corrupt != 0 {
		t.Errorf("%d of %d concurrent reads saw a half-written file", corrupt, reads)
	}
	if reads < 100 {
		t.Fatalf("only %d reads raced the writers; the test proved little", reads)
	}
	t.Logf("%d writes, %d concurrent reads, %d corrupt", writers*writesEach, reads, corrupt)

	// What landed must be complete and parseable
	filename := GetDeviceStatusFilePath(uid)
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("cannot read the status file: %s", err)
	}
	var final DeviceStatus
	if err = json.Unmarshal(contents, &final); err != nil {
		t.Fatalf("the status file did not survive as valid JSON: %s", err)
	}
	if final.DeviceUID != uid {
		t.Errorf("status file holds device %q, expected %q", final.DeviceUID, uid)
	}
	if final.CapturedAt == nil || final.Lnd == nil || final.Lnd.U7318 == nil {
		t.Error("the merged values did not survive the concurrent writes")
	}

	// It must remain writable by every server instance, as O_CREATE 0666 made it
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("cannot stat the status file: %s", err)
	}
	if info.Mode().Perm() != 0666 {
		t.Errorf("status file mode is %v, expected -rw-rw-rw-", info.Mode().Perm())
	}

	// And no temp files may be left behind
	entries, err := os.ReadDir(filepath.Dir(filename))
	if err != nil {
		t.Fatalf("cannot list the status directory: %s", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}
}
