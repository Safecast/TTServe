#! /bin/bash

# The assumption is that we are running in the folder
#     $GOPATH/src/github.com/rayozzie/teletype-ttserve

# First, ensure that GOPATH is set to the folder containing "src"
export GOPATH=$(readlink -m ../../../..)

while [ : ]; do
    echo "Updating from GitHub..."
    git pull https://ttserve:teletype123@github.com/rayozzie/teletype-ttserve
    echo "Rebuilding..."
    go get -u
    go build
    echo "Starting..."
    # Note that we must "sudo" so we can listen on port # less than 1024
    sudo ./teletype-ttserve $HOME
    echo "Restarting..."
    sleep 2s
done
