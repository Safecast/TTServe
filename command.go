// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Device monitoring
package main

import (
    "os"
    "fmt"
    "time"
    "io/ioutil"
    "strings"
    "encoding/json"
)

// Structures
const (
    ObjGroup = "group"
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
            fmt.Printf("OZZIE reading %s\n", SafecastDirectory() + TTCommandStatePath + "/" + file.Name())
            contents, err := ioutil.ReadFile(SafecastDirectory() + TTCommandStatePath + "/" + file.Name())
            if err != nil {
                continue
            }
            fmt.Printf("OZZIE got:\n%s\n", contents)

            // Parse the JSON, and ignore it if nonparse-sable
            value := State{}
            err = json.Unmarshal(contents, &value)
            if err != nil {
                continue
            }
            fmt.Printf("OZZIE unmarshaled\n")

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
        user = ""
        objname = strings.Replace(objname, "=", "", 1)
    }

    // Loop over all user state objjects
    for _, s := range CachedState {

        // Skip if not relevant
        if s.User != user && s.User != "" {
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

    // No luck
    return false, ""

}

// Find a named object
func CommandObjList(user string, objtype string, objname string) string {

    // Refresh, just for good measure
    CommandCacheRefresh()

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

            oname := o.Name
            if s.User == "" {
                oname = "=" + o.Name
            }

            out += fmt.Sprintf("%s: %s", oname, o.Value)

        }

    }

    if out == "" {

        switch objtype {

        case ObjGroup:
            return "No device lists found. Add one by typing: device add <list-name> <device number or name>"

        case ObjMark:
            return "No saved marks found. Add one by typing: mark set <mark-name>"

        case ObjReport:
            return "No saved reports found. Add one by typing: report set <mark-name>"

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
        user = ""
        objname = strings.Replace(objname, "=", "", 1)
    }

    // Loop over all user state objjects
    for i, s := range CachedState {

        // Skip if not relevant
        if s.User != user && s.User != "" {
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

// Find a named object
func CommandParse(user string, objtype string, message string) string {

    args := strings.Split(message, " ")
    messageAfterSecondArg := ""
    if len(args) > 2 {
        messageAfterSecondArg = strings.Join(args[2:], " ")
    }

    if message == "" || len(args) == 1 {
        return CommandObjList(user, objtype, "")
    }

    objname := args[1]
    switch args[0] {

    case "get":
        fallthrough
    case "list":
        fallthrough
    case "show":
        return CommandObjList(user, objtype, objname)

    case "add":
        if objtype == ObjGroup {
            found, value := CommandObjGet(user, objtype, objname)
            return(fmt.Sprintf("not yet: get = %t %s", found, value))
        }
        // fallthrough
    case "set":
		if messageAfterSecondArg == "" {
			return fmt.Sprintf("Please specify the new value for %s.", objname)
		}
        CommandObjSet(user, objtype, objname, messageAfterSecondArg)
        return(CommandObjList(user, objtype, objname))

    case "remove":
        if objtype == ObjGroup {
            found, value := CommandObjGet(user, objtype, objname)
            return(fmt.Sprintf("not yet: get = %t %s", found, value))
        }
        // vallthrough
    case "delete":
        if (!CommandObjSet(user, objtype, objname, "")) {
            return "Not found."
        }
        return "Deleted."
    }

    return CommandObjList(user, objtype, args[0])

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
        return CommandParse(user, ObjGroup, messageAfterFirstArg)

    case "marks":
        fallthrough
    case "mark":
        return CommandParse(user, ObjMark, messageAfterFirstArg)

    case "reports":
        fallthrough
    case "report":
        return CommandParse(user, ObjReport, messageAfterFirstArg)

    }

    return "Unrecognized command"

}
