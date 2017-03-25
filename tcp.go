// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound TCP support
package main

import (
	"io"
    "net"
    "fmt"
	"bufio"
)

// Kick off TCP single-upload request server
func TcpInboundHandler() {

    fmt.Printf("Now handling inbound TCP on %s\n", TTServerTCPPort)

    ServerAddr, err := net.ResolveTCPAddr("tcp", TTServerTCPPort)
    if err != nil {
        fmt.Printf("Error resolving TCP port: \n%v\n", err)
        return
    }

    ServerConn, err := net.ListenTCP("tcp", ServerAddr)
    if err != nil {
        fmt.Printf("Error listening on TCP port: \n%v\n", err)
        return
    }
    defer ServerConn.Close()

    for {

		// Accept the TCP connection
        conn, err := ServerConn.AcceptTCP()
        if err != nil {
            fmt.Printf("Error accepting TCP session: \n%v\n", err)
            continue
        }

		// Create a reader on that connection
		rdconn := bufio.NewReader(conn)

		// Read the payload buffer format
		payload_format_len := 1
        payload_format := make([]byte, payload_format_len)
        n, err := io.ReadFull(rdconn, payload_format)
        if err != nil {
            fmt.Printf("tcp: can't read format: \n%v\n", err)
            conn.Close()
            continue
        }
		if n != payload_format_len {
            fmt.Printf("tcp: can't read format: %d/%d\n", n, payload_format_len)
            conn.Close()
            continue
		}
		if (payload_format[0] != BUFF_FORMAT_PB_ARRAY) {
            fmt.Printf("tcp: unsupported buffer format: %d\n", payload_format[0])
            conn.Close()
            continue
		}

		// Read the number of array entries
		payload_count_len := 1
        payload_count := make([]byte, payload_count_len)
        n, err = io.ReadFull(rdconn, payload_count)
        if err != nil {
            fmt.Printf("tcp: can't read count: \n%v\n", err)
            conn.Close()
            continue
        }
		if n != payload_count_len {
            fmt.Printf("tcp: can't read count: %d/%d\n", n, payload_count_len)
            conn.Close()
            continue
		}
		if (payload_count[0] == 0) {
            fmt.Printf("tcp: unsupported count: %d\n", payload_count[0])
            conn.Close()
            continue
		}

		// Read the length array
		payload_entry_lengths_len := int(payload_count[0])
        payload_entry_lengths := make([]byte, payload_entry_lengths_len)
        n, err = io.ReadFull(rdconn, payload_entry_lengths)
        if err != nil {
            fmt.Printf("tcp: can't read entry_lengths: \n%v\n", err)
            conn.Close()
            continue
        }
		if n != int(payload_entry_lengths_len) {
            fmt.Printf("tcp: can't read entry_lengths: %d/%d\n", n, payload_entry_lengths_len)
            conn.Close()
            continue
		}

		// Read the entries
		payload_entries_len := 0
		for i:=0; i<int(payload_entry_lengths_len); i++ {
			payload_entries_len += int(payload_entry_lengths[i])
		}
        payload_entries := make([]byte, payload_entries_len)
        n, err = io.ReadFull(rdconn, payload_entries)
        if err != nil {
            fmt.Printf("tcp: can't read entries: \n%v\n", err)
            conn.Close()
            continue
        }
		if n != payload_entries_len {
            fmt.Printf("tcp: can't read entries: %d/%d\n", n, payload_entries_len)
            conn.Close()
            continue
		}

		// Combine all that we've read
		payload := append(payload_format, payload_count...)
		payload = append(payload, payload_entry_lengths...)
		payload = append(payload, payload_entries...)
		
        // Initialize a new AppReq
        AppReq := IncomingAppReq{}
        AppReq.SvTransport = "device-tcp:" + ipv4(conn.RemoteAddr().String())

		// Get the reply device ID
        ReplyToDeviceId := getReplyDeviceIdFromPayload(payload)
		
        // Push it to be processed
        go AppReqPushPayload(AppReq, payload, "device directly")
        stats.Count.TCP++;

        // Is there a device ID to reply to?
        if (ReplyToDeviceId != 0) {

            // See if there's an outbound message waiting for this device.
            isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceId)
            if (isAvailable) {

                // Responses are binary on TCP
                conn.Write(payload)
                sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceId), SLACK_MSG_UNSOLICITED)
            }

        }

        // Close the connection
        conn.Close()

    }

}
