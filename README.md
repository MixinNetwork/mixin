# mixin

The Mixin BFT-DAG network reference implementation, the Trusted Execution Environment is not integrated into this repository yet.

## Get Started

Install golang and setup `GOPATH` following this guide https://golang.org/doc/install.

```bash
$ go get -u github.com/MixinNetwork/mixin
$ cd $GOPATH/src/github.com/MixinNetwork/mixin
$ go build
```

The `mixin` command is both the kernel node and tools to communicate with the node RPC interface.

```
$ ./mixin

NAME:
   mixin - A free and lightning fast peer-to-peer transactional network for digital assets.

USAGE:
   mixin [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     kernel, k             Start the Mixin Kernel daemon
     setuptestnet          Setup the test nodes and genesis
     createaddress         Create a new Mixin address
     updateheadreference   Update the cache round external reference, never use it unless agree by other nodes
     signrawtransaction    Sign a JSON encoded transaction
     sendrawtransaction    Broadcast a hex encoded signed raw transaction
     decoderawtransaction  Decode a raw transaction as JSON
     listsnapshots         List finalized snapshots
     gettransaction        Get the finalized transaction by hash
     getinfo               Get info from the node
     help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## Mixin Kernel Address

Mixin Kernel address are a pair of ed25519 keys, following the [CryptoNote](https://cryptonote.org/standards/) protocol. To create a new address use the `createaddress` command.

```
$ ./mixin createaddress

address:	XINJkpCdwVk3qFqmS3AAAoTmC5Gm2fR3iRF7Rtt7hayuaLXNrtztS3LGPSxTmq5KQh3KJ2qYXYE5a9w8BWXhZAdsJKXqcvUr
view key:	568302b687a2fa3e8853ff35d99ffdf3817b98170de7b51e43d0dcf4fe30470f
spend key:	7c2b5c97278ed371d75610cccd9681af31b0d99be4adc2d66983f3c455fc9702
```

Share the `address` to receive assets from other Mixin Kernel addresses, and keep `view key` and `spend key` privately and securely.

Both the `view key` and `spend key` are required to spend the assets received from others, and the `view key` iteself is sufficient to decode and view all the transactions sent to `address`.

## Start a Kernel Node

To start a node, create a directory `mixin` for the config and network data files, then put the genesis.json, nodes.json and config.json files in it.

The main net genesis.json, nodes.json and an example config.example.json files can be obtained from [here](https://github.com/MixinNetwork/mixin/tree/master/config), you only need to put your own signer spend key in the config.json file.

```
$ ./mixin help kernel

NAME:
   mixin kernel - Start the Mixin Kernel daemon

USAGE:
   mixin kernel [command options] [arguments...]

OPTIONS:
   --dir value, -d value   the data directory
   --port value, -p value  the peer port to listen (default: 7239)
```

## Local Test Net

This will setup a minimum local test net, with all nodes in a single device.

```
$ ./mixin setuptestnet

$ ./mixin kernel -dir /tmp/mixin-7001 -port 7001
$ ./mixin kernel -dir /tmp/mixin-7002 -port 7002
$ ./mixin kernel -dir /tmp/mixin-7003 -port 7003
$ ./mixin kernel -dir /tmp/mixin-7004 -port 7004
$ ./mixin kernel -dir /tmp/mixin-7005 -port 7005
$ ./mixin kernel -dir /tmp/mixin-7006 -port 7006
$ ./mixin kernel -dir /tmp/mixin-7007 -port 7007
```
