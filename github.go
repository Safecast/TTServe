// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Github webhook that enables server auto-restart on commit
package main

import (
    "os"
    "fmt"
    "net/http"
    "io/ioutil"
    "encoding/json"
)

// Github webhook
func inboundWebGithubHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++
	stats.Count.HTTPGithub++
	
	// Unpack the request
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Github webhook: error reading body:", err)
        return
    }
    var p PushPayload
    err = json.Unmarshal(body, &p)
    if err != nil {
        fmt.Printf("Github webhook: error unmarshaling body:", err)
        return
    }

    // Handle 'git commit -mm' and 'git commit -amm', used in dev intermediate builds, in a more aesthetically pleasing manner.
    if p.HeadCommit.Commit.Message == "m" {
        ServerLog(fmt.Sprintf("*** RESTARTING because %s pushed %s's commit to GitHub\n", p.Pusher.Name, p.HeadCommit.Commit.Committer.Name))
    } else {
        sendToSafecastOps(fmt.Sprintf("** Restarting ** %s %s",
            p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message), SLACK_MSG_UNSOLICITED_OPS)
        ServerLog(fmt.Sprintf("*** RESTARTING because %s pushed %s's commit to GitHub: %s\n",
			p.Pusher.Name, p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message))
    }

	// Exit
    os.Exit(0)

}
