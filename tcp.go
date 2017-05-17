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

// TCPInboundHandler kicks off TCP single-upload request server
func TCPInboundHandler() {

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
            fmt.Printf("\nTCP: rror accepting TCP session: \n%v\n", err)
            continue
        }

        // Create a reader on that connection
        rdconn := bufio.NewReader(conn)

        // Read the payload buffer format
        payloadFormatLen := 1
        payloadFormat := make([]byte, payloadFormatLen)
        n, err := io.ReadFull(rdconn, payloadFormat)
        if err != nil {
            fmt.Printf("\nTCP: can't read format: \n%v\n", err)
            conn.Close()
            continue
        }
        if n != payloadFormatLen {
            fmt.Printf("\nTCP: can't read format: %d/%d\n", n, payloadFormatLen)
            conn.Close()
            continue
        }
        if payloadFormat[0] != BuffFormatPBArray {
            fmt.Printf("\n%s TCP request from %s ignored\n", LogTime(), ipv4(conn.RemoteAddr().String()))
			buf1 := make([]byte, 1024)
			n, err := rdconn.Read(buf1)
            if err == nil || err == io.EOF || err == io.ErrUnexpectedEOF {
                buf2 := append(payloadFormat, buf1[:n]...)
                b := make([]byte, len(buf2))
                var bl int
				var ch, chPrev byte
                for i := 0; i < len(buf2); i++ {
					ch = buf2[i]
					if ch < 32 || ch >= 127 {
						if chPrev == ';' {
							ch = ' '
						} else {
							ch = ';'
						}
					}
					if ch != ' ' || chPrev != ' ' {
	                    b[bl] = ch
						bl++
					}
					chPrev = ch
                }
                if bl != 0 {
                    fmt.Printf("%s\n", string(b[:bl]))
                }
            }
            conn.Close()
            continue
        }

        // Read the number of array entries
        payloadCountLen := 1
        payloadCount := make([]byte, payloadCountLen)
        n, err = io.ReadFull(rdconn, payloadCount)
        if err != nil {
            fmt.Printf("\nTCP: can't read count: \n%v\n", err)
            conn.Close()
            continue
        }
        if n != payloadCountLen {
            fmt.Printf("\nTCP: can't read count: %d/%d\n", n, payloadCountLen)
            conn.Close()
            continue
        }
        if payloadCount[0] == 0 {
            fmt.Printf("\nTCP: unsupported count: %d\n", payloadCount[0])
            conn.Close()
            continue
        }

        // Read the length array
        payloadEntryLengthsLen := int(payloadCount[0])
        payloadEntryLengths := make([]byte, payloadEntryLengthsLen)
        n, err = io.ReadFull(rdconn, payloadEntryLengths)
        if err != nil {
            fmt.Printf("\nTCP: can't read entry_lengths: \n%v\n", err)
            conn.Close()
            continue
        }
        if n != int(payloadEntryLengthsLen) {
            fmt.Printf("\nTCP: can't read entry_lengths: %d/%d\n", n, payloadEntryLengthsLen)
            conn.Close()
            continue
        }

        // Read the entries
        payloadEntriesLen := 0
        for i:=0; i<int(payloadEntryLengthsLen); i++ {
            payloadEntriesLen += int(payloadEntryLengths[i])
        }
        payloadEntries := make([]byte, payloadEntriesLen)
        n, err = io.ReadFull(rdconn, payloadEntries)
        if err != nil {
            fmt.Printf("\nTCP: can't read entries: \n%v\n", err)
            conn.Close()
            continue
        }
        if n != payloadEntriesLen {
            fmt.Printf("\nTCP: can't read entries: %d/%d\n", n, payloadEntriesLen)
            conn.Close()
            continue
        }

        // Combine all that we've read
        payload := append(payloadFormat, payloadCount...)
        payload = append(payload, payloadEntryLengths...)
        payload = append(payload, payloadEntries...)

        // Initialize a new AppReq
        AppReq := IncomingAppReq{}
        AppReq.SvTransport = "device-tcp:" + ipv4(conn.RemoteAddr().String())

        // Get the reply device ID
        ReplyToDeviceID := getReplyDeviceIDFromPayload(payload)

        // Push it to be processed
        go AppReqPushPayload(AppReq, payload, "device directly")
        stats.Count.TCP++

        // Is there a device ID to reply to?
        if ReplyToDeviceID != 0 {

            // See if there's an outbound message waiting for this device.
            isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceID)
            if isAvailable {

                // Responses are binary on TCP
                conn.Write(payload)
                sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID), SlackMsgUnsolicitedOps)
            }

        }

        // Close the connection
        conn.Close()

    }

}
