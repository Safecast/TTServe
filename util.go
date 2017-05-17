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

// UtilInit initializes the utility package
func UtilInit() {

    // Initialize the random number generator
    rand.Seed(time.Now().Unix() + int64(crc32.ChecksumIEEE([]byte(TTServeInstanceID))))

}

// Random gets a random number in a range
func Random(min, max int) int {
    return rand.Intn(max - min) + min
}

// SafecastDirectory gets the path of the root safecast file system folder shared among instances
func SafecastDirectory() string {
    directory := os.Args[1]
    if directory == "" {
        fmt.Printf("TTSERVE: first argument must be folder containing safecast data!\n")
        os.Exit(0)
    }
    return(directory)
}

// LogTime gets the current time in log format
func LogTime() string {
    return time.Now().Format(logDateFormat)
}

// NowInUTC gets the current time in UTC as a string formatted for log files
func NowInUTC() string {
    return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// AgoMinutes returns, readably, a duration given a count of minutes
func AgoMinutes(minutesAgo uint32) string {
    var hoursAgo = minutesAgo / 60
    var daysAgo = hoursAgo / 24
    minutesAgo -= hoursAgo * 60
    hoursAgo -= daysAgo * 24
    s := ""
    if daysAgo >= 14 {
        if 0 == (daysAgo%7) {
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

// Ago is like AgoMinutes, but with a time.Time
func Ago(when time.Time) string {
    return AgoMinutes(uint32(int64(time.Now().Sub(when) / time.Minute)))
}

// GetWhenFromOffset takes a GPS-formatted base date and time, plus offset, and returns a UTC string
func GetWhenFromOffset(baseDate uint32, baseTime uint32, offset uint32) string {
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

// ErrorString cleans up an error string to eliminate the filename so that it can be logged without PII
func ErrorString(err error) string {
    errString := fmt.Sprintf("%s", err)
    s0 := strings.Split(errString, ":")
    s1 := s0[len(s0)-1]
    s2 := strings.TrimSpace(s1)
    return s2
}

// RemapCommonUnicodeToASCII converts common UTF-8 strings to ASCII, where ASCII is required
func RemapCommonUnicodeToASCII(str string) string {
    Conversions := []string{
        "\\u2000", " ",  // EN QUAD
        "\\u2001", " ",  // EM QUAD
        "\\u2002", " ",  // EN SPACE
        "\\u2003", " ",  // EM SPACE
        "\\u2004", " ",  // THREE-PER-EM SPACE
        "\\u2005", " ",  // FOUR-PER-EM SPACE
        "\\u2006", " ",  // SIX-PER-EM SPACE
        "\\u2007", " ",  // FIGURE SPACE
        "\\u2008", " ",  // PUNCTUATION SPACE
        "\\u2009", " ",  // THIN SPACE
        "\\u200A", " ",  // HAIR SPACE
        "\\u200B", " ",  // ZERO WIDTH SPACE
        "\\u200C", " ",  // ZERO WIDTH NON-JOINER
        "\\u200D", " ",  // ZERO WIDTH JOINER
        "\\u200E", "",   // LEFT-TO-RIGHT MARK
        "\\u200F", "",   // RIGHT-TO-LEFT MARK
        "\\u2010", "-",  // HYPHEN
        "\\u2011", "-",  // NON-BREAKING HYPHEN
        "\\u2012", "-",  // FIGURE DASH
        "\\u2013", "-",  // EN DASH
        "\\u2014", "-",  // EM DASH
        "\\u2015", "-",  // HORIZONTAL BAR
        "\\u2016", "|",  // DOUBLE VERTICAL LINE
        "\\u2017", "_",  // DOUBLE LOW LINE
        "\\u2018", "'",  // LEFT SINGLE QUOTATION MARK
        "\\u2019", "'",  // RIGHT SINGLE QUOTATION MARK
        "\\u201A", "'",  // SINGLE LOW-9 QUOTATION MARK
        "\\u201B", "'",  // SINGLE HIGH-REVERSED-9 QUOTATION MARK
        "\\u201C", "\"", // LEFT DOUBLE QUOTATION MARK
        "\\u201D", "\"", // RIGHT DOUBLE QUOTATION MARK
        "\\u201E", "\"", // DOUBLE LOW-9 QUOTATION MARK
        "\\u201F", "\"", // DOUBLE HIGH-REVERSED-9 QUOTATION MARK
        "\\u2020", "|",  // DAGGER
        "\\u2021", "|",  // DOUBLE DAGGER
        "\\u2022", "-",  // BULLET
        "\\u2023", ">",  // TRIANGULAR BULLET
        "\\u2024", ".",  // ONE DOT LEADER
        "\\u2025", "..", // TWO DOT LEADER
        "\\u2026", "...",// HORIZONTAL ELLIPSIS
        "\\u2027", "-",  // HYPHENATION POINT
        "\\u2028", "",   // LINE SEPARATOR
        "\\u2029", "",   // PARAGRAPH SEPARATOR
        "\\u202A", "",   // LEFT-TO-RIGHT EMBEDDING
        "\\u202B", "",   // RIGHT-TO-LEFT EMBEDDING
        "\\u202C", "",   // POP DIRECTIONAL FORMATTING
        "\\u202D", "",   // LEFT-TO-RIGHT OVERRIDE
        "\\u202E", "",   // RIGHT-TO-LEFT OVERRIDE
        "\\u202F", " ",  // NARROW NO-BREAK SPACE
        "\\u2030", "%",  // PER MILLE SIGN
        "\\u2031", "%",  // PER TEN THOUSAND SIGN
        "\\u2032", "'",  // PRIME
        "\\u2033", "\"", // DOUBLE PRIME
        "\\u2034", "\"", // TRIPLE PRIME
        "\\u2035", "'",  // REVERSED PRIME
        "\\u2036", "\"", // REVERSED DOUBLE PRIME
        "\\u2037", "\"", // REVERSED TRIPLE PRIME
        "\\u2038", "^",  // CARET
        "\\u2039", "<",  // SINGLE LEFT-POINTING ANGLE QUOTATION MARK
        "\\u203A", ">",  // SINGLE RIGHT-POINTING ANGLE QUOTATION MARK
        "\\u203B", "*",  // REFERENCE MARK
        "\\u203C", "!!", // DOUBLE EXCLAMATION MARK
        "\\u203D", "?!", // INTERROBANG
        "\\u203E", "=",  // OVERLINE
        "\\u203F", "_",  // UNDERTIE
        "\\u2040", "-",  // TIE
        "\\u2041", "|",  // CARET INSERTION POINT
        "\\u2042", "*",  // ASTERISM
        "\\u2043", "-",  // HYPHEN BULLET
        "\\u2044", "/",  // FRACTION SLASH
        "\\u2045", "{",  // LEFT SQUARE BRACKET WITH QUILL
        "\\u2046", "}",  // RIGHT SQUARE BRACKET WITH QUILL
        "\\u2047", "??", // DOUBLE QUESTION MARK
        "\\u2048", "?!", // QUESTION EXCLAMATION MARK
        "\\u2049", "!?", // EXCLAMATION QUESTION MARK
        "\\u204A", "-",  // TIRONIAN SIGN ET
        "\\u204B", "%",  // REVERSED PILCROW SIGN
        "\\u204C", "<",  // BLACK LEFTWARDS BULLET
        "\\u204D", ">",  // BLACK RIGHTWARDS BULLET
        "\\u204E", "*",  // LOW ASTERISK
        "\\u204F", ";",  // REVERSED SEMICOLON
        "\\u2050", "_",  // CLOSE UP
        "\\u2051", "**", // TWO ASTERISKS ALIGNED VERTICALLY
        "\\u2052", "%",  // COMMERCIAL MINUS SIGN
        "\\u2053", "~",  // SWUNG DASH
        "\\u2054", "_",  // INVERTED UNDERTIE
        "\\u2055", "*",  // FLOWER PUNCTUATION MARK
        "\\u2056", "...",// THREE DOT PUNCTUATION
        "\\u2057", "\"\"",// QUADRUPLE PRIME
        "\\u2058", ":",  // FOUR DOT PUNCTUATION
        "\\u2059", ":",  // FIVE DOT PUNCTUATION
        "\\u205A", ":",  // TWO DOT PUNCTUATION
        "\\u205B", ":",  // FOUR DOT MARK
        "\\u205C", "#",  // DOTTED CROSS
        "\\u205D", ":",  // TRICOLON
        "\\u205E", ":",  // VERTICAL FOUR DOTS
        "\\u205F", " ",  // MEDIUM MATHEMATICAL SPACE
        "\\u2060", "",   // WORD JOINER
        "\\u2061", "",   // FUNCTION APPLICATION
        "\\u2062", "*",  // INVISIBLE TIMES
        "\\u2063", " ",  // INVISIBLE SEPARATOR
        "\\u2064", "+",  // INVISIBLE PLUS
        "\\u2065", " ",  // INVISIBLE SPACE
        "\\u2066", "",   // LEFT-TO-RIGHT ISOLATE
        "\\u2067", "",   // RIGHT-TO-LEFT ISOLATE
        "\\u2068", "",   // FIRST STRONG ISOLATE
        "\\u2069", "",   // POP DIRECTIONAL ISOLATE
        "\\u206A", "",   // INHIBIT SYMMETRIC SWAPPING
        "\\u206B", "",   // ACTIVATE SYMMETRIC SWAPPING
        "\\u206C", "",   // INHIBIT ARABIC FORM SHAPING
        "\\u206D", "",   // ACTIVATE ARABIC FORM SHAPING
        "\\u206E", "?",  // NATIONAL DIGIT SHAPES
        "\\u206F", "",   // NOMINAL DIGIT SHAPES
        "\\u2070", "0",  // SUPERSCRIPT ZERO
        "\\u2071", "i",  // SUPERSCRIPT LATIN SMALL LETTER I
        "\\u2072", "",   // ?
        "\\u2073", "",   // ?
        "\\u2074", "4",  // SUPERSCRIPT FOUR
        "\\u2075", "5",  // SUPERSCRIPT FIVE
        "\\u2076", "6",  // SUPERSCRIPT SIX
        "\\u2077", "7",  // SUPERSCRIPT SEVEN
        "\\u2078", "8",  // SUPERSCRIPT EIGHT
        "\\u2079", "9",  // SUPERSCRIPT NINE
        "\\u207A", "+",  // SUPERSCRIPT PLUS SIGN
        "\\u207B", "-",  // SUPERSCRIPT MINUS
        "\\u207C", "=",  // SUPERSCRIPT EQUALS SIGN
        "\\u207D", "(",  // SUPERSCRIPT LEFT PARENTHESIS
        "\\u207E", ")",  // SUPERSCRIPT RIGHT PARENTHESIS
        "\\u207F", "n",  // SUPERSCRIPT LATIN SMALL LETTER N
        "", ""}

    // First, convert UTF-8 to ASCII so we can replace it
    ascii := strconv.QuoteToASCII(str)

	// Now, loop through the conversions
    for i := 0; Conversions[i] != ""; i += 2 {
		ascii = strings.Replace(ascii, Conversions[i], Conversions[i+1], -1)
	}

	// Now, remove the outer quotes that we added
	ascii, _ = strconv.Unquote(ascii)
	
	// Done
	return ascii
	
}
