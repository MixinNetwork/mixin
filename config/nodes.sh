#!/bin/bash

NODES=(
  "new-mixin-node0.exinpool.com:8239"
  "new-mixin-node1.exinpool.com:8239"
  "new-mixin-node2.exinpool.com:8239"
  "new-mixin-node3.exinpool.com:8239"
  "mixin-node-lehigh-1.hotot.org:8239"
  "mixin-node-lehigh-2.hotot.org:8239"
  "mixin-node-42.f1ex.io:8239"
  "mixin-node-fes.f1ex.io:8239"
  "mixin-node-box-1.b.watch:8239"
  "mixin-node-box-2.b.watch:8239"
  "mixin-node-box-3.b.watch:8239"
  "mixin-node-box-4.b.watch:8239"
  "mixin-node-okashi.mixin.fan:8239"
  "mixin-node1.b1.run:8239"
  "mixin-node2.b1.run:8239"
  "mixin-node3.b1.run:8239"
  "mixin-node4.b1.run:8239"
  "mixin-node6.b1.run:8239"
  "mixin-node7.b1.run:8239"
  "mixin-node8b.b1.run:8239"
  "34.42.197.136:8239"
  "13.51.72.77:8239"
  "3.227.254.217:8239"
  "44.197.199.140:8239"
  "16.170.250.120:8239"
  "13.51.169.35:8239"
  "43.206.154.20:8239"
)

for NODE in "${NODES[@]}"
do
  echo "$NODE"
  mixin -n "$NODE" getinfo | jq '.network, .node, .version, .timestamp, .graph.topology'
  echo
done

