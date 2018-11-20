#!/bin/bash
./raiden-v0.17.0-linux \
    --keystore-path keystore \
    --password YOURSECRETPASSWORD \
    --network-id kovan \
    --environment-type development \
    --gas-price 20000000000 \
    --eth-rpc-endpoint http://localhost:8545 \
    --api-address 0.0.0.0:8888
