#!/bin/sh

if [ -z ${1+x} ]; then
    # Obtain "kernel" name
    KERNEL=$(uname -s)
else
    # Set "kernel" name from parameter
    KERNEL=$1
fi

if [ $KERNEL = "Darwin" ] || [ $KERNEL = "darwin" ]; then
    KERNEL=macOS
    FILE=.zip
elif [ $KERNEL = "Linux" ] || [ $KERNEL = "linux" ]; then
    KERNEL=linux
    FILE=.tar.gz
elif [ $KERNEL = "FreeBSD" ]; then
    KERNEL=linux
    FILE=.tar.gz
else
    echo "Unsupported OS"
    exit 1
fi


RAIDENV=v0.19.0
RAIDENBIN=raiden-$RAIDENV-$KERNEL

echo "Fetching Raiden $RAIDENV"
curl -O https://raiden-nightlies.ams3.digitaloceanspaces.com/$RAIDENBIN$FILE

if [ $KERNEL = "linux" ]; then
    tar -xzvf $RAIDENBIN$FILE
else
    unzip $RAIDENBIN$FILE
fi
rm $RAIDENBIN$FILE
mv $RAIDENBIN raiden-binary
