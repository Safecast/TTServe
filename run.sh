#! /bin/bash
while [ : ]; do
    echo "Updating from GitHub..."
    git pull https://ttserve:teletype123@github.com/rayozzie/teletype-ttserve
    echo "Rebuilding..."
    go build
    echo "Starting..."
    ./teletype-ttserve
    echo "Restarting..."
    sleep 2s
done
