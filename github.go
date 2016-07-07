// Github webhook, used so that we can sense new commits
package main

import (
    "os"
    "fmt"
    "net/http"
    "io/ioutil"
    "encoding/json"
    "./github"
)

// Github webhook
func inboundWebGithubHandler(rw http.ResponseWriter, req *http.Request) {

	// Unpack the request
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Github webhook: error reading body:", err)
        return
    }
    var p github.PushPayload
    err = json.Unmarshal(body, &p)
    if err != nil {
        fmt.Printf("Github webhook: error unmarshaling body:", err)
        return
    }

    // Handle 'git commit -mm' and 'git commit -amm', used in dev intermediate builds, in a more aesthetically pleasing manner.
    if p.HeadCommit.Commit.Message == "m" {
        fmt.Printf("\n***\n***\n*** RESTARTING because\n*** %s\n***\n***\n\n",
            fmt.Sprintf("%s pushed %s's commit to GitHub", p.Pusher.Name, p.HeadCommit.Commit.Committer.Name))
    } else {
        sendToSlack(fmt.Sprintf("** Restarting ** %s %s",
            p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message))
        fmt.Printf("\n***\n***\n*** RESTARTING because\n*** %s\n***\n***\n\n",
            fmt.Sprintf("%s pushed %s's commit to GitHub: %s",
                p.Pusher.Name, p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message))
    }

    os.Exit(0)

}
