#! /bin/bash

# Note that by placing the body of this procedure into
# a separate shell script, github can updated it even while
# we are executing this one that is perpetually in-use.

# Make sure we're in the right directory, which is necessary via cron
cd $HOME/go/src/github.com/rayozzie/teletype-ttserve

# Loop forever
while [ : ]; do
    ./run-this.sh
    sleep 1s
done
