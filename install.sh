#!/bin/sh

# First Obtain "kernel" name
KERNEL=$(uname -s)

if      [ $KERNEL = "Darwin" ]; then
        KERNEL=mac
elif        [ $KERNEL = "Linux" ]; then
        KERNEL=linux
elif        [ $KERNEL = "FreeBSD" ]; then
        KERNEL=linux
else
        echo "Unsupported OS"
fi

RAIDENV=v0.19.0
RAIDENBIN=raiden-$RAIDENV-$KERNEL

echo "Fetching Raiden $RAIDENV"
curl -O https://raiden-nightlies.ams3.digitaloceanspaces.com/$RAIDENBIN.tar.gz
tar -xzvf $RAIDENBIN.tar.gz
rm $RAIDENBIN.tar.gz
mv $RAIDENBIN raiden-binary
