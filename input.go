// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func inputHandler() {

	// Create a scanner to watch stdin
	scanner := bufio.NewScanner(os.Stdin)
	var text string

	for {

		fmt.Print("\n> ")
		scanner.Scan()
		text = scanner.Text()

		switch strings.ToLower(text) {

		case "":

		case "q":
			ServerLog("*** RESTARTING at console request\n")
			os.Exit(0)

		default:
			fmt.Printf("Unrecognized: '%s'\n", text)

		}

	}

}
