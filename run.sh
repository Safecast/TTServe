#! /bin/bash
while [ : ]; do
    echo "Updating from GitHub..."
    git pull
    echo "Rebuilding..."
    go build
    echo "Starting...\n")
    ./ttserve
done
