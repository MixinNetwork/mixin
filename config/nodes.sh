#!/bin/bash

NODES=(
  "https://kernel.mixin.dev"
  "https://rpc-mixin.exinpool.com"
)

for NODE in "${NODES[@]}"
do
  echo "$NODE"
  mixin -n "$NODE" getinfo | jq '.network, .node, .version, .timestamp, .graph.topology'
  echo
done

