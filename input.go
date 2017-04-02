// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "os"
    "bufio"
    "fmt"
)

func inputHandler() {

    // Create a scanner to watch stdin
    scanner := bufio.NewScanner(os.Stdin)
    var text string

    for {

        fmt.Print("Enter your text: ")
        scanner.Scan()
        text = scanner.Text()

        switch text {

        case "q":
            fmt.Printf("Your text was: '%s'\n", text)
        }

    }

}
