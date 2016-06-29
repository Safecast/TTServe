#! /bin/bash
while [ : ]; do
    echo "Updating from GitHub..."
    git pull https://ttserve:teletype123@github.com/rayozzie/teletype-ttserve
    echo "Rebuilding..."
    go build
    echo "Starting...\n"
    ./ttserve
done
