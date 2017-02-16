// Inbound support for the "/<deviceid>" HTTP topic
package main

import (
	"os"
    "net/http"
    "fmt"
	"time"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebValueHandler(rw http.ResponseWriter, req *http.Request) {

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    filename := req.RequestURI[len(TTServerTopicValue):]
    fmt.Printf("%s Device information request for %s\n", time.Now().Format(logDateFormat), filename)

    // Open the file
    file := SafecastDirectory() + TTServerValuePath + "/" + filename + ".json"
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}