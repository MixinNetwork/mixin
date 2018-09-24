# mixin

the Mixin TEE-BFT network reference implementation


## Local Test Net

This will setup a minimum test net, with all nodes in a single device.

```
./mixin setuptestnet
./mixin kernel -dir /tmp/mixin-7001 -port 7001 &
./mixin kernel -dir /tmp/mixin-7002 -port 7002 &
./mixin kernel -dir /tmp/mixin-7003 -port 7003 &
./mixin kernel -dir /tmp/mixin-7004 -port 7004 &
./mixin kernel -dir /tmp/mixin-7005 -port 7005 &
./mixin kernel -dir /tmp/mixin-7006 -port 7006 &
./mixin kernel -dir /tmp/mixin-7007 -port 7007
```
