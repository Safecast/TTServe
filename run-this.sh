#! /bin/bash

# Echo commands as we execute them
set -v

# Mount the EFS volume, assuming that we're running under AWS
while [ ! -d "$HOME/efs/safecast" ]; do
    pushd "$HOME"
    sudo mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 fs-dd1ad674.efs.us-west-2.amazonaws.com:/ efs
    popd
    sleep 5s
done

# Update from github
git pull https://ttserve:teletype123@github.com/rayozzie/teletype-ttserve
# Update dependencies
go get -u
# Build executables
go build

# Run the server.  Note that we use sudo because it is a Linux
# design constraint that non-supervisors cannot listen on ports
# less than 1024.  We need this.
sudo ./teletype-ttserve $HOME/efs/safecast
