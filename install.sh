#!/bin/sh

RAIDENV=v0.18.0
RAIDENBIN=raiden-$RAIDENV-linux.tar.gz

echo "Fetching Onboarder Tool"
curl -O https://raiden-nightlies.ams3.digitaloceanspaces.com/onboarder-linux.tar.gz
tar -xzvf onboarder-linux.tar.gz
rm onboarder-linux.tar.gz
echo "Fetching Raiden $RAIDENV"
curl -O https://raiden-nightlies.ams3.digitaloceanspaces.com/$RAIDENBIN
tar -xzvf $RAIDENBIN
rm $RAIDENBIN
