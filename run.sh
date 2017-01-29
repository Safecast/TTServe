#! /bin/bash

# Note that by placing the body of this procedure into
# a separate shell script, github can updated it even while
# we are executing this one that is perpetually in-use.

# Trap Signal 2 (Ctrl+c)
#trap '' 2

# Loop forever
while [ : ]; do
    ./run-this.sh
    sleep 1s
done
