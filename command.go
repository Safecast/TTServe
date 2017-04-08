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
)
type Object struct {
    Name                string          `json:"obj_name,omitempty"`
    Type                string          `json:"obj_type,omitempty"`
    Value               string          `json:"obj_value,omitempty"`
}
type State struct {
    User                string          `json:"user,omitempty"`
    Objects             []Object        `json:"objects,omitempty"`
}
var CachedState []State


// Statics
var   CommandStateLastModified time.Time

// Refresh the command cache
func CommandCacheRefresh() {
    var RefreshedState []State

    // Exit if nothing needs refreshing
    LastModified := ControlFileTime(TTServerCommandStateControlFile, "")
    if LastModified == CommandStateLastModified {
        return
    }

    // Make sure that we only do this once per modification, even if errors
    CommandStateLastModified = LastModified

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
            value := State{}
            err = json.Unmarshal(contents, &value)
            if err != nil {
                continue
            }

            // Add to what we're accumulating
            RefreshedState = append(RefreshedState, value)

        }

    }

    // Replace the cached state
    CachedState = RefreshedState

}

// Find a named object
func CommandObjGet(user string, objtype string, objname string) (bool, string) {

    // Refresh, just for good measure
    CommandCacheRefresh()

    // Handle global queries
    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
		return CommandObjGet("", objtype, objname)
    }

    // Loop over all user state objjects
    for _, s := range CachedState {

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
		return CommandObjGet("", objtype, objname)
	}

    // No luck
    return false, ""

}

// Find a named object
func CommandObjList(user string, objtype string, objname string) string {

    // Refresh, just for good measure
    CommandCacheRefresh()

    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
		return CommandObjList("", objtype, objname)
    }

    // Init output buffer
    out := ""

    // Loop over all user state objjects
    for _, s := range CachedState {

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
func CommandStateUpdate(s State) {

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
        CommandStateLastModified = ControlFileTime(TTServerCommandStateControlFile, "state update")

    }

}

// Find a named object
func CommandObjSet(user string, objtype string, objname string, objval string) bool {

    // Refresh, just for good measure
    CommandCacheRefresh()

    // Handle global queries
    if strings.HasPrefix(objname, "=") {
        objname = strings.Replace(objname, "=", "", 1)
		return CommandObjSet("", objtype, objname, objval)
    }

    // Loop over all user state objjects
    for i, s := range CachedState {

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
                CachedState[i].Objects[j].Value = objval
            } else {
                if len(s.Objects) == 1 {
                    CachedState[i].Objects = nil
                } else  {
                    CachedState[i].Objects[j] = CachedState[i].Objects[len(s.Objects)-1]
                    CachedState[i].Objects = CachedState[i].Objects[:len(s.Objects)-1]
                }
            }

            // Update it
            CommandStateUpdate(CachedState[i])
            return true

        }

        // If we're removing it and it's not there, fail
        if objval == "" {
            return false
        }

        // Append the new object
        o := Object{}
        o.Name = objname
        o.Type = objtype
        o.Value = objval
        CachedState[i].Objects = append(CachedState[i].Objects, o)

        // Update it
        CommandStateUpdate(CachedState[i])
        return true

    }

    // If we couldn't find the user state, add it
    o := Object{}
    o.Name = objname
    o.Type = objtype
    o.Value = objval
    s := State{}
    s.User = user
    s.Objects = append(s.Objects, o)
    CachedState = append(CachedState, s)

    // Update it
    CommandStateUpdate(CachedState[len(CachedState)-1])
    return true

}

// Parse a command and execute it
func CommandParse(user string, objtype string, message string) string {

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
	
    switch args[0] {

    case "get":
        fallthrough
    case "list":
        fallthrough
    case "show":
        return CommandObjList(user, objtype, objname)

	case "run":
		if objtype != ObjReport {
			return fmt.Sprintf("%s is not a report.", objname)
		}
		return(ReportRun(user, messageAfterFirstArg))

    case "add":
        if objtype == ObjDevice {
			found, value := CommandObjGet(user, objtype, objname)
			if !found {
				value = ""
			}
		    for _, d := range strings.Split(messageAfterSecondArg, " ") {
				valid, result, _ := DeviceVerify(d)
				if !valid {
					return result
				}
				if value == "" {
					value = result
				} else {
					value = value + "," + result
				}
			}
			CommandObjSet(user, objtype, objname, value)
	        return(CommandObjList(user, objtype, objname))
        }
        fallthrough
    case "set":
		if objtype == ObjMark {
			valid, result := MarkVerify(messageAfterSecondArg)
			if !valid {
				return result
			}
			CommandObjSet(user, objtype, objname, result)
		} else if objtype == ObjReport {
			valid, result, _, _, _ := ReportVerify(user, messageAfterSecondArg)
			if !valid {
				return result
			}
			CommandObjSet(user, objtype, objname, result)
		} else if objtype == ObjDevice {
			value := ""
		    for _, d := range strings.Split(messageAfterSecondArg, " ") {
				valid, result, _ := DeviceVerify(d)
				if !valid {
					return result
				}
				if value == "" {
					value = result
				} else {
					value = value + "," + result
				}
			}
			CommandObjSet(user, objtype, objname, value)
        }

        return(CommandObjList(user, objtype, objname))

    case "remove":
        if objtype == ObjDevice {
            found, value := CommandObjGet(user, objtype, objname)
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
	        CommandObjSet(user, objtype, objname, newvalue)
	        return(CommandObjList(user, objtype, objname))
        }
        fallthrough
    case "delete":
        if (!CommandObjSet(user, objtype, objname, "")) {
            return fmt.Sprintf("%s not found.", objname)
        }
        return fmt.Sprintf("%s Deleted.", objname)
    }

	// Unrecognized command.  It might just be a raw report
	if objtype == ObjReport {
		return(ReportRun(user, message))
	} 
	
    return "Valid subcommands are show, add, set, remove, delete"

}

// Process a command that will modify the cache and the on-disk state
func Command(user string, message string) string {

    // Process the command arguments
    args := strings.Split(message, " ")
    messageAfterFirstArg := ""
    if len(args) > 1 {
        messageAfterFirstArg = strings.Join(args[1:], " ")
    }

    // Dispatch command
    switch args[0] {

    case "devs":
        fallthrough
    case "dev":
        return CommandParse(user, ObjDevice, messageAfterFirstArg)

    case "marks":
        fallthrough
    case "mark":
        return CommandParse(user, ObjMark, messageAfterFirstArg)

    case "run":
        fallthrough
    case "reports":
        fallthrough
    case "report":
        return CommandParse(user, ObjReport, messageAfterFirstArg)

    }

    return "Unrecognized command"

}

// Verify a device to be added to the device list
func DeviceVerify(device string) (bool, string, uint32) {

	valid, deviceid := WordsToNumber(device)
	if !valid {
		if device == "" {
			return false, fmt.Sprintf("Please supply a device identifier to add."), 0
		}			
		return false, fmt.Sprintf("%s is not a valid device identifier.", device), 0
	}

	return true, device, deviceid
}

// Verify a mark or transform it
func MarkVerify(mark string) (bool, string) {

	// If nothing is specified, make the mark NOW
	if mark == "" {
		return true, nowInUTC()
	}
		
	// Verify that this is a UTC date
    _, err := time.Parse("2006-01-02T15:04:05Z", mark)
    if err == nil {
		return true, mark
	}

	// If not, see if this is just a number of hours ago
	if strings.HasSuffix(mark, "h") {
		mark = strings.TrimSuffix(mark, "h")
		i64, err := strconv.ParseInt(mark, 10, 32)
		if err == nil {
			return true, time.Now().UTC().Add(time.Duration(i64) * time.Hour).Format("2006-01-02T15:04:05Z")
		}
	}
	if strings.HasSuffix(mark, "m") {
		mark = strings.TrimSuffix(mark, "m")
		i64, err := strconv.ParseInt(mark, 10, 32)
		if err == nil {
			return true, time.Now().UTC().Add(time.Duration(i64) * time.Minute).Format("2006-01-02T15:04:05Z")
		}
	}
	
	// Valid
	return false, fmt.Sprintf("The mark's UTC time must be either 2017-04-05T19:08:07Z or -5h for 5h ago")
}

// Verify a report or transform it
func ReportVerify(user string, report string) (rValid bool, rResult string, rDeviceList []uint32, rFrom string, rTo string) {

	// Break up into its parts
    args := strings.Split(report, " ")

	// The blank command is more-or-less the help string
	if report == "" || len(args) < 2 || len(args) > 3 {
		rValid = false
		rResult = "report add <report-name> <device> <from> [<to>]\n<device> can be device ID, device name, or the name of a device list\n<from> can be UTC date, -7h for 7h ago, or a mark name\n<to> can be a UTC date, a mark name, or is 'now' if ommitted"
		return
	}

	device_arg := args[0]
	from_arg := args[1]
	to_arg := ""
	if len(args) > 2 {
		to_arg = args[2]
	}
	
	// See if device is a device ID
	valid, result, deviceid := DeviceVerify(device_arg)
	if valid {

		// Just a single device
		rDeviceList = append(rDeviceList, deviceid)

	} else {

		// Expand the list
		valid, result := CommandObjGet(user, ObjDevice, device_arg)
		if valid {
		    for _, d := range strings.Split(result, ",") {
				valid, _, deviceid := DeviceVerify(d)
				if valid {
					rDeviceList = append(rDeviceList, deviceid)
				}
			}
		} else {
			rValid = false
			rResult = fmt.Sprintf("%s is neither a device or a device list name", device_arg)
			return
		}

	}

	// See if the next arg is a mark
	valid, result = MarkVerify(from_arg)
	if valid {
		rFrom = result
	} else {

		// See if it's a mark name
		valid, result := CommandObjGet(user, ObjMark, from_arg)
		if valid {
			rFrom = result
		} else {
			rValid = false
			rResult = fmt.Sprintf("%s is neither a date or a mark name", from_arg)
			return
		}

	}

	// We're done if there's no final arg
	if to_arg == "" {
		rValid = true
		rTo = nowInUTC()
		rResult = report
		return
	}

	// Validate the to arg
	valid, result = MarkVerify(to_arg)
	if valid {
		rTo = result
	} else {

		// See if it's a mark name
		valid, result := CommandObjGet(user, ObjMark, to_arg)
		if valid {
			rTo = result
		} else {
			rValid = false
			rResult = fmt.Sprintf("%s is neither a date or a mark name", to_arg)
			return
		}

	}

	// Valid
	rValid = true
	rResult = report
	return
	
}

// Run a report or transform it
func ReportRun(user string, report string) string {

	// See if there is only one arg which is the report name
	if !strings.Contains(report, " ") {
		found, value := CommandObjGet(user, ObjReport, report)
		if !found {
			return fmt.Sprintf("Report %s not found.", report)
		}
		report = value
	}	

	// Validate and expand the report
	valid, result, devices, from, to := ReportVerify(user, report)
	if !valid {
		return result
	}

	// Display the results
	return fmt.Sprintf("User:%s From:%s To:%s Devices:%v", user, from, to, devices)

}
