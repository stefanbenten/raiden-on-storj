#!/bin/bash
./raiden-v0.17.0-linux \
    --keystore-path keystore \
    --network-id kovan \
    --environment-type development \
    --gas-price 20000000000 \
    --eth-rpc-endpoint https://kovan.infura.io/v3/d636d4118e2f4cab9b72296d74b9ff62 \
    --api-address 0.0.0.0:8888
    
