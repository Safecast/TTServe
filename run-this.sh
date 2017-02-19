#! /bin/bash

# Note that when we run the server we use sudo because it is a Linux
# design constraint that non-supervisors cannot listen on ports
# less than 1024. This was discovered when running on GCS, which
# by default runs our code unprivileged.

# Mount the EFS volume, assuming that we're now running under AWS
while [ ! -d "$HOME/efs/safecast" ]; do
    pushd "$HOME"
    sudo mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 fs-dd1ad674.efs.us-west-2.amazonaws.com:/ efs
    popd
    sleep 5s
done

set -v
git pull https://ttserve:teletype123@github.com/rayozzie/teletype-ttserve
go get -u
go build
sudo ./teletype-ttserve $HOME/efs/safecast
