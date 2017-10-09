// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Device monitoring
package main

import (
    "os"
    "fmt"
    "time"
    "strconv"
    "io/ioutil"
    "strings"
    "encoding/json"
)

// Structures
const (
    ObjDevice = "group"
    ObjMark = "mark"
    ObjReport = "report"
	ReportHelp = "report <show, set, delete, run>\nreport <report-name>\nreport <device> <from> [<to>]\n    <device> is device name/number or device list name\n    <from> is UTC datetime or NNh/NNm *ago* or mark name\n    <to> is UTC datetime or NNh/NNm *duration* or mark name"
)

type commandObject struct {
    Name                string          `json:"obj_name,omitempty"`
    Type                string          `json:"obj_type,omitempty"`
    Value               string          `json:"obj_value,omitempty"`
}
type commandState struct {
    User                string          `json:"user,omitempty"`
    Objects             []commandObject        `json:"objects,omitempty"`
}
var cachedState []commandState

// DeviceRange is a span of DeviceIDs
type DeviceRange struct {
	Low		uint32
	High	uint32
}

// Statics
var commandStateLastModified time.Time

// Refresh the command cache
func commandCacheRefresh() {
    var RefreshedState []commandState

    // Exit if nothing needs refreshing
    LastModified := ControlFileTime(TTServerCommandStateControlFile, "")
    if LastModified == commandStateLastModified {
        return
    }

    // Make sure that we only do this once per modification, even if errors
    commandStateLastModified = LastModified

    // Iterate over all files in the directory, loading their contents
    files, err := ioutil.ReadDir(SafecastDirectory() + TTCommandStatePath)
    if err == nil {

        // Iterate over each of the values
        for _, file := range files {

            // Skip things we can't read
            if file.IsDir() {
                continue
            }

            // Read the file if we can
            contents, err := ioutil.ReadFile(SafecastDirectory() + TTCommandStatePath + "/" + file.Name())
            if err != nil {
                continue
            }

            // Parse the JSON, and ignore it if nonparse-sable
            value := commandState{}
            err = json.Unmarshal(contents, &value)
            if err != nil {
                continue
            }

            // Add to what we're accumulating
            RefreshedState = append(RefreshedState, value)

        }

    }

    // Replace the cached state
    cachedState = RefreshedState

}

// Find a named object
func commandObjGet(user string, objtype string, objname string) (bool, string) {

    // Refresh, just for good measure
    commandCacheRefresh()

    // Handle global queries
    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
        return commandObjGet("", objtype, objname)
    }

    // Loop over all user state objjects
    for _, s := range cachedState {

        // Skip if not relevant
        if s.User != user {
            continue
        }

        // Search for this object
        for _, o := range s.Objects {

            // Skip if not what we're looking for
            if objtype != o.Type || objname != o.Name {
                continue
            }

            // Got it
            return true, o.Value

        }


    }

    // See if it's there as a global
    if user != "" {
        return commandObjGet("", objtype, objname)
    }

    // No luck
    return false, ""

}

// Find a named object
func commandObjList(user string, objtype string, objname string) string {

    // Refresh, just for good measure
    commandCacheRefresh()

    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
        return commandObjList("", objtype, objname)
    }

    // Init output buffer
    out := ""

    // Loop over all user state objjects
    for _, s := range cachedState {

        // Skip if not relevant
        if s.User != user && s.User != "" {
            continue
        }

        // Search for this object
        for _, o := range s.Objects {

            // Skip if not what we're looking for
            if objtype != o.Type {
                continue
            }

            // If objname is specified, skip if not it
            if objname != "" && o.Name != objname {
                continue
            }

            if out != "" {
                out += "\n"
            }

            val := o.Value
            if objtype == ObjDevice {
                val = strings.Replace(val, ",", "  ", -1)
            }
            if s.User == "" {
                out += fmt.Sprintf("%s=  %s", o.Name, val)
            } else {
                out += fmt.Sprintf("%s:  %s", o.Name, val)
            }
        }

    }

    if out == "" {

        switch objtype {

        case ObjDevice:
            if objname != "" {
                return "No device: " + objname
            }
            return "No device lists. Add one by typing: device add <list-name> <device number or name>"

        case ObjMark:
            if objname != "" {
                return "No mark: " + objname
            }
            return "No marks. Add one by typing: mark add <mark-name>"

        case ObjReport:
            if objname != "" {
                return "No report: " + objname
            }
            return "No reports. Add one by typing: report add <mark-name>"

        default:
            return "Not found."

        }

    }

    return out

}

// Update state
func commandStateUpdate(s commandState) {

    // Marshall the state
    contents, _ := json.MarshalIndent(s, "", "    ")

    // Update the file
    filename := s.User
    if s.User == "" {
        filename = "global"
    }

    path := SafecastDirectory() + TTCommandStatePath + "/" + filename + ".json"

    fd, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
    if err == nil {

        // Write the data
        fd.WriteString(string(contents))
        fd.Close()

        // Update the control file time
        commandStateLastModified = ControlFileTime(TTServerCommandStateControlFile, "state update")

    }

}

// Find a named object
func commandObjSet(user string, objtype string, objname string, objval string) bool {

    // Refresh, just for good measure
    commandCacheRefresh()

    // Handle global queries
    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
        return commandObjSet("", objtype, objname, objval)
    }

    // Loop over all user state objjects
    for i, s := range cachedState {

        // Skip if not relevant
        if s.User != user {
            continue
        }

        // Search for this object
        for j, o := range s.Objects {

            // Skip if not what we're looking for
            if objtype != o.Type || objname != o.Name {
                continue
            }

            // Update or remove the element
            if objval != "" {
                cachedState[i].Objects[j].Value = objval
            } else {
                if len(s.Objects) == 1 {
                    cachedState[i].Objects = nil
                } else  {
                    cachedState[i].Objects[j] = cachedState[i].Objects[len(s.Objects)-1]
                    cachedState[i].Objects = cachedState[i].Objects[:len(s.Objects)-1]
                }
            }

            // Update it
            commandStateUpdate(cachedState[i])
            return true

        }

        // If we're removing it and it's not there, fail
        if objval == "" {
            return false
        }

        // Append the new object
        o := commandObject{}
        o.Name = objname
        o.Type = objtype
        o.Value = objval
        cachedState[i].Objects = append(cachedState[i].Objects, o)

        // Update it
        commandStateUpdate(cachedState[i])
        return true

    }

    // If we couldn't find the user state, add it
    o := commandObject{}
    o.Name = objname
    o.Type = objtype
    o.Value = objval
    s := commandState{}
    s.User = user
    s.Objects = append(s.Objects, o)
    cachedState = append(cachedState, s)

    // Update it
    commandStateUpdate(cachedState[len(cachedState)-1])
    return true

}

// Parse a command and execute it
func commandParse(user string, command string, objtype string, message string) string {

    args := strings.Split(message, " ")
    messageAfterFirstArg := ""
    if len(args) > 1 {
        messageAfterFirstArg = strings.Join(args[1:], " ")
    }
    messageAfterSecondArg := ""
    if len(args) > 2 {
        messageAfterSecondArg = strings.Join(args[2:], " ")
    }
    objname := ""
    if len(args) > 1 {
        objname = args[1]
    }

    switch strings.ToLower(args[0]) {

    case "get":
        fallthrough
    case "list":
        fallthrough
    case "show":
        return commandObjList(user, objtype, objname)

    case "run":
        if objtype != ObjReport {
            return fmt.Sprintf("%s is not a report.", objname)
        }
		_, result, _ := ReportRun(user, true, messageAfterFirstArg)
        return result

    case "add":
        if objtype == ObjDevice {
            found, value := commandObjGet(user, objtype, objname)
            if !found {
                value = ""
            }
            for _, d := range strings.Split(messageAfterSecondArg, " ") {
                if d == "" {
                    continue
                }
				valid, result, _ := rangeVerify(d)
				if !valid {
	                valid, result, _ = deviceVerify(d)
				}
                if !valid {
                    return result
                }
                if value == "" {
                    value = result
                } else {
                    value = value + "," + result
                }
            }
            commandObjSet(user, objtype, objname, value)
            return(commandObjList(user, objtype, objname))
        }
        fallthrough
    case "set":
        if objtype == ObjMark {
            valid, result, _ := markVerify(messageAfterSecondArg, NowInUTC(), false)
            if !valid {
                return result
            }
            commandObjSet(user, objtype, objname, result)
        } else if objtype == ObjReport {
            valid, result, _, _, _, _, _, _ := reportVerify(user, messageAfterSecondArg)
            if !valid {
                return result
            }
            commandObjSet(user, objtype, objname, result)
        } else if objtype == ObjDevice {
            value := ""
            for _, d := range strings.Split(messageAfterSecondArg, " ") {
                if d == "" {
                    continue
                }
				valid, result, _ := rangeVerify(d)
				if !valid {
	                valid, result, _ = deviceVerify(d)
				}
                if !valid {
                    return result
                }
                if value == "" {
                    value = result
                } else {
                    value = value + "," + result
                }
            }
            commandObjSet(user, objtype, objname, value)
        }

        return(commandObjList(user, objtype, objname))

    case "remove":
        if objtype == ObjDevice {
            found, value := commandObjGet(user, objtype, objname)
            if !found {
                return fmt.Sprintf("Device list %s not found.", objname)
            }
            if messageAfterSecondArg == "" || strings.Contains(messageAfterSecondArg, " ") {
                return fmt.Sprintf("Please specify a single device identifier to remove.")
            }
            newvalue := ""
            for _, d := range strings.Split(value, ",") {
                if d == messageAfterSecondArg {
                    continue
                }
                if newvalue == "" {
                    newvalue = d
                } else {
                    newvalue = newvalue + "," + d
                }
            }
            if newvalue == value {
                return fmt.Sprintf("Device list %s does not contain %s", objname, messageAfterSecondArg)
            }
            commandObjSet(user, objtype, objname, newvalue)
            if newvalue == "" {
                return fmt.Sprintf("%s deleted.", objname)
            }
            return(commandObjList(user, objtype, objname))
        }
        fallthrough
    case "delete":
        if (!commandObjSet(user, objtype, objname, "")) {
            return fmt.Sprintf("%s not found.", objname)
        }
        return fmt.Sprintf("%s deleted.", objname)
    }

    // Unrecognized command.  It might just be a raw report
    if objtype == ObjReport {

		// Run the report
		if strings.ToLower(command) != "check" {
			_, result, _ := ReportRun(user, true, message)
	        return result
		}

		// Get the JSON
		
        success, result, filename := ReportRun(user, false, message)
		if !success {
			return result
		}

		// Create the output file for the check
	    file := time.Now().UTC().Format("2006-01-02-150405") + "-" + user + ".txt"
	    outfile := SafecastDirectory() + TTInfluxQueryPath + "/"  + file
	    fd, err := os.OpenFile(outfile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
		if err != nil {
			return fmt.Sprintf("Error creating output file: %s", err)
		}
	    defer fd.Close()

		// Perform the check
		checksuccess, checkresults := CheckJSON(filename)
		if !checksuccess {
			return checkresults
		}

		// Write it to the file
	    fd.WriteString(checkresults)

		// Done
	    url := fmt.Sprintf("http://%s%s%s", TTServerHTTPAddress, TTServerTopicQueryResults, file)
		return fmt.Sprintf("Check results are <%s|here> and %s", url, result)

    }

    return "Valid subcommands are show, add, set, remove, delete"

}

// Do the command and report output to Safecast, usable as a goroutine
func sendCommandToSlack(user string, message string) {
    response := command(user, message)
    sendToSafecastOps(response, SlackMsgReply)
}

// Process a command that will modify the cache and the on-disk state
func command(user string, message string) string {

    // Process the command arguments
    args := strings.Split(message, " ")
    messageAfterFirstArg := ""
    if len(args) > 1 {
        messageAfterFirstArg = strings.Join(args[1:], " ")
    }
    messageAfterSecondArg := ""
    if len(args) > 2 {
        messageAfterSecondArg = strings.Join(args[2:], " ")
    }

    // Dispatch command
	command := strings.ToLower(args[0])
    switch command {

    case "devices":
        fallthrough
    case "device":
        return commandParse(user, args[0], ObjDevice, messageAfterFirstArg)

    case "marks":
        fallthrough
    case "mark":
        return commandParse(user, args[0], ObjMark, messageAfterFirstArg)

    case "run":
        fallthrough
    case "check":
        fallthrough
    case "report":
        return commandParse(user, args[0], ObjReport, messageAfterFirstArg)

    case "checkall":
        fallthrough
    case "reportall": 
		devicelist := ""
		if len(args) > 1 {
			devicelist = args[1]
		}
	    valid, _, devices, _, _ := DeviceList(user, devicelist)
	    if !valid {
	        return "Invalid device list"
	    }
		s := ""
		for _, device := range devices {
			newArg0 := "check"
			if command != "checkall" {
				newArg0 = "report"
			}
			newMessageAfterFirstArg := fmt.Sprintf("%d %s", device, messageAfterSecondArg)
			if s != "" {
				s += "\n"
			}
			s += commandParse(user, newArg0, ObjReport, newMessageAfterFirstArg)
		}
		if s == "" {
			s = "No device specified"
		}
		return s
    }

    return "Unrecognized command"

}

// Parse the plus code pttern
func plusCodePattern(code string) string {

	// Turn it into a pattern by replacing the + wildcard with a single-char wildcard
	code = strings.Replace(code, "+", ".", 1)
	
	// Extract the pattern, and exit if no pattern present
	components := strings.Split(code, "~")
	if len(components) != 2 {
		return code
	}
	c := components[0]

	// Try to recognize the pattern
	switch strings.ToLower(components[1]) {

	default:
		return c
		
	case "3m":
		return c
		
	case "14m":
		return c[0:11] + "*"

	case "275m":
		return c[0:8] + "*"

	case "5500m":
		fallthrough
	case "5.5km":
		return c[0:6] + "*"

	case "110km":
		return c[0:4] + "*"

	case "2200km":
		return c[0:2] + "*"
		
	}

}

// See if this string is a location query specifier
func plusCode(code string) bool {
    if strings.Contains(code, "+") {
        return true
    }
    return false
}

// Look up a number from two or three simple words
func rangeVerify(what string) (bool, string, DeviceRange) {
	var r DeviceRange
	
    parts := strings.Split(what, "-")
    if len(parts) != 2 {
		return false, "Not a device range", DeviceRange{}
	}

    // See if low part parses cleanly as a number
    i64, err := strconv.ParseUint(parts[0], 10, 32)
    if err != nil {
		return false, "Not a device range", DeviceRange{}
	}
	r.Low = uint32(i64)

    // See if high part parses cleanly as a number
    i64, err = strconv.ParseUint(parts[1], 10, 32)
    if err != nil {
		return false, "Not a device range", DeviceRange{}
	}
	r.High = uint32(i64)

	return true, what, r
}

// DeviceList gets a list of devices
func DeviceList(user string, devicelist string) (rValid bool, rResult string, rExpanded []uint32, rExpandedRange []DeviceRange, rExpandedplusCodes []string) {

	isrange, _, r := rangeVerify(devicelist)
    isdevice, result, deviceid := deviceVerify(devicelist)

	if isrange {

		rExpandedRange = append(rExpandedRange, r)
		
    } else if isdevice {

        // Just a single device or plus code
        if deviceid != 0 {
            rExpanded = append(rExpanded, deviceid)
        } else {
            rExpandedplusCodes = append(rExpandedplusCodes, result)
        }

    } else {

        // Expand the list
        valid, result := commandObjGet(user, ObjDevice, devicelist)
        if valid {
            for _, d := range strings.Split(result, ",") {

				isrange, _, r := rangeVerify(d)
			    isdevice, result, deviceid := deviceVerify(d)

				if isrange {

					rExpandedRange = append(rExpandedRange, r)

                } else if isdevice {
					
			        // Append the device or plus code
                    if deviceid != 0 {
                        rExpanded = append(rExpanded, deviceid)
                    } else {
                        rExpandedplusCodes = append(rExpandedplusCodes, result)
                    }

                }

            }

        } else {

            rValid = false
            rResult = fmt.Sprintf("%s is neither a device or a device list name", devicelist)
            return

        }

    }

    rValid = true
    return

}

// Verify a device to be added to the device list
func deviceVerify(device string) (rValid bool, rResult string, rDeviceID uint32) {

    valid, deviceid := WordsToNumber(device)
    if !valid {
        if plusCode(device) {
            return true, device, 0
        }
        if device == "" {
            return false, fmt.Sprintf("Please supply a device identifier to add."), 0
        }
        return false, fmt.Sprintf("%s is not a valid device identifier.", device), 0
    }

    return true, device, deviceid
}

// Verify a mark or transform it
func markVerify(mark string, reference string, fBackwards bool) (rValid bool, rOriginal string, rExpanded string) {

    // If nothing is specified, just return the reference
    if mark == "" {
        return true, NowInUTC(), reference
    }

    // If not, see if this is just a number of days/hrs/mins
	minutesOffset := 0
    if strings.HasSuffix(mark, "w") {
        markval := strings.TrimSuffix(mark, "w")
        i64, err := strconv.ParseInt(markval, 10, 32)
        if err != nil {
		    return false, fmt.Sprintf("Badly formatted number of weeks: %s", mark), ""
        }
		if i64 < 0 {
			i64 = -i64
		}
		minutesOffset = int(i64) * 60 * 24 * 7
    }
    if strings.HasSuffix(mark, "d") {
        markval := strings.TrimSuffix(mark, "d")
        i64, err := strconv.ParseInt(markval, 10, 32)
        if err != nil {
		    return false, fmt.Sprintf("Badly formatted number of days: %s", mark), ""
        }
		if i64 < 0 {
			i64 = -i64
		}
		minutesOffset = int(i64) * 60 * 24
    }
    if strings.HasSuffix(mark, "h") {
        markval := strings.TrimSuffix(mark, "h")
        i64, err := strconv.ParseInt(markval, 10, 32)
        if err != nil {
		    return false, fmt.Sprintf("Badly formatted number of hours: %s", mark), ""
        }
		if i64 < 0 {
			i64 = -i64
		}
		minutesOffset = int(i64) * 60
    }
    if strings.HasSuffix(mark, "m") {
        markval := strings.TrimSuffix(mark, "m")
        i64, err := strconv.ParseInt(markval, 10, 32)
        if err != nil {
		    return false, fmt.Sprintf("Badly formatted number of minutes: %s", mark), ""
        }
		if i64 < 0 {
			i64 = -i64
		}
		minutesOffset = int(i64)
    }

    // Verify that this is a UTC date
	if minutesOffset == 0 {
	    _, err := time.Parse("2006-01-02T15:04:05Z", mark)
	    if err != nil {
		    return false, fmt.Sprintf("Badly formatted UTC date: %s", mark), ""
		}
        return true, mark, mark
	}

	// We need to offset the reference time by either a positive or negative amount of time
    referenceTime, err := time.Parse("2006-01-02T15:04:05Z", reference)
    if err != nil {
	    return false, fmt.Sprintf("Badly formatted UTC reference date: %s", reference), ""
	}

	if fBackwards {
		minutesOffset = -minutesOffset
	}

    return true, mark, referenceTime.UTC().Add(time.Duration(minutesOffset) * time.Minute).Format("2006-01-02T15:04:05Z")

}

// Verify a report or transform it
func reportVerify(user string, report string) (rValid bool, rResult string, deviceArg string, rDeviceList []uint32, rDeviceRange []DeviceRange, rplusCodeList []string, rFrom string, rTo string) {

    // Break up into its parts
    args := strings.Split(report, " ")

    // The blank command is more-or-less the help string
    if report == "" || len(args) < 2 || len(args) > 3 {
        rValid = false
        rResult = ReportHelp
        return
    }

    deviceArg = args[0]
    fromArg := args[1]
    toArg := ""
    if len(args) > 2 {
        toArg = args[2]
    }

    // See if device is a valid device ID
    valid, result, devicelist, devicerange, pluscodelist := DeviceList(user, deviceArg)
    if valid {
        rDeviceList = devicelist
		rDeviceRange = devicerange
        rplusCodeList = pluscodelist
    } else {
        rValid = false
        rResult = result
        return
    }


    // See if the next arg is a mark
    valid, _, result = markVerify(fromArg, NowInUTC(), true)
    if valid {
        rFrom = result
    } else {

        // See if it's a mark name
        valid, result := commandObjGet(user, ObjMark, fromArg)
        if valid {
            valid, _, result = markVerify(result, NowInUTC(), true)
            if valid {
                rFrom = result
            }
        }
        if !valid {
            rValid = false
            rResult = fmt.Sprintf("%s is neither a date or a mark name", fromArg)
            return
        }

    }

    // We're done if there's no final arg
    if toArg == "" {
        rValid = true
        rTo = NowInUTC()
        rResult = report
        return
    }

    // Validate the to arg
	valid, _, result = markVerify(toArg, rFrom, false)
    if valid {
        rTo = result
    } else {

        // See if it's a mark name
        valid, result := commandObjGet(user, ObjMark, toArg)
        if valid {
            valid, _, result = markVerify(result, rFrom, false)
            if valid {
                rTo = result
            }
        }
        if !valid {
            rValid = false
            rResult = fmt.Sprintf("%s is neither a date or a mark name", toArg)
            return
        }

    }

    // Valid
    rValid = true
    rResult = report
    return

}

// ReportRun runs a report
func ReportRun(user string, csv bool, report string) (success bool, result string, filename string) {
	
    // See if there is only one arg which is the report name
    if !strings.Contains(report, " ") {
		if report == "" {
			return false, ReportHelp, ""
		}
        found, value := commandObjGet(user, ObjReport, report)
        if !found {
            return false, fmt.Sprintf("Report %s not found.", report), ""
        }
        report = value
    }

    // Validate and expand the report
    valid, result, deviceArg, devices, ranges, pluscodes, from, to := reportVerify(user, report)
    if !valid {
        return false, result, ""
    }

    // Generate base of query
    sql := "* FROM data"

    // Generate device filter, which is required
    sql += " WHERE ( "
	first := true
    for _, d := range devices {
        if !first {
            sql += " OR "
        }
		first = false
        sql += fmt.Sprintf("device = %d", d)
    }
    for _, r := range ranges {
        if !first {
            sql += " OR "
        }
		first = false
		sql += fmt.Sprintf("( device >= %d AND device <= %d )", r.Low, r.High)
    }
    for _, s := range pluscodes {
        if !first {
            sql += " OR "
        }
		first = false
        sql += fmt.Sprintf("loc_olc =~ /%s/", plusCodePattern(s))
    }
    sql += " )"

    // Generate time filter
    sql += fmt.Sprintf(" AND ( time >= '%s' AND time < '%s' )", from, to)

    // Execute the query
    success, numrows, result, filename := InfluxQuery(user, deviceArg, sql, csv)
    if !success {
        return false, result, ""
    }

    // Done
    return true, fmt.Sprintf("%d rows of data for %s are <%s|here>, @%s.", numrows, deviceArg, result, user), filename

}
