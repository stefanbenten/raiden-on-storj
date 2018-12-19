#!/bin/sh

RAIDENV=v0.19.0
RAIDENBIN=raiden-$RAIDENV-linux

echo "Fetching Raiden $RAIDENV"
curl -O https://raiden-nightlies.ams3.digitaloceanspaces.com/$RAIDENBIN.tar.gz
tar -xzvf $RAIDENBIN.tar.gz
rm $RAIDENBIN.tar.gz
mv $RAIDENBIN raiden-binary
