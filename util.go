// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "os"
    "fmt"
    "strconv"
    "math/rand"
    "hash/crc32"
    "time"
    "strings"
)

// Initialize package
func UtilInit() {

    // Initialize the random number generator
    rand.Seed(time.Now().Unix() + int64(crc32.ChecksumIEEE([]byte(TTServeInstanceID))))

}

// Get a random number in a range
func random(min, max int) int {
    return rand.Intn(max - min) + min
}

// Get path of the safecast directory
func SafecastDirectory() string {
    directory := os.Args[1]
    if (directory == "") {
        fmt.Printf("TTSERVE: first argument must be folder containing safecast data!\n")
        os.Exit(0)
    }
    return(directory)
}

// Get the current time in log format
func logTime() string {
    return time.Now().Format(logDateFormat)
}

// Get the current time in UTC as a string
func nowInUTC() string {
    return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// How long ago, readably, given a count of minutes
func AgoMinutes(minutesAgo uint32) string {
    var hoursAgo uint32 = minutesAgo / 60
    var daysAgo uint32 = hoursAgo / 24
    minutesAgo -= hoursAgo * 60
    hoursAgo -= daysAgo * 24
    s := ""
    if daysAgo >= 14 {
        if ((daysAgo%7) == 0) {
            s = fmt.Sprintf("%d weeks", daysAgo/7)
        } else {
            s = fmt.Sprintf("%d+ weeks", daysAgo/7)
        }
    } else if daysAgo > 2 {
        s = fmt.Sprintf("%d days", daysAgo)
    } else if daysAgo != 0 {
        s = fmt.Sprintf("%dd %dh", daysAgo, hoursAgo)
    } else if hoursAgo != 0 {
        s = fmt.Sprintf("%dh %dm", hoursAgo, minutesAgo)
    } else if minutesAgo < 1 {
        s = fmt.Sprintf("<1m")
    } else if minutesAgo < 100 {
        s = fmt.Sprintf("%02dm", minutesAgo)
    } else {
        s = fmt.Sprintf("%dm", minutesAgo)
    }
    return s
}

// How long ago, readably, given a time
func Ago(when time.Time) string {
    return AgoMinutes(uint32(int64(time.Now().Sub(when) / time.Minute)))
}

// Take a GPS-formatted base date and time, plus offset, and return a UTC string
func getWhenFromOffset(baseDate uint32, baseTime uint32, offset uint32) string {
    var i64 uint64
    s := fmt.Sprintf("%06d%06d", baseDate, baseTime)
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[0], s[1]), 10, 32)
    day := uint32(i64)
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[2], s[3]), 10, 32)
    month := uint32(i64)
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[4], s[5]), 10, 32)
    year := uint32(i64) + 2000
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[6], s[7]), 10, 32)
    hour := uint32(i64)
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[8], s[9]), 10, 32)
    minute := uint32(i64)
    i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[10], s[11]), 10, 32)
    second := uint32(i64)
    tbefore := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
    tafter := tbefore.Add(time.Duration(offset) * time.Second)
    return tafter.UTC().Format("2006-01-02T15:04:05Z")
}

// Function to clean up an error string to eliminate the filename
func errorString(err error) string {
    errString := fmt.Sprintf("%s", err)
    s0 := strings.Split(errString, ":")
    s1 := s0[len(s0)-1]
    s2 := strings.TrimSpace(s1)
    return s2
}
