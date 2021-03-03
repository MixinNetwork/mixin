# mixin

The Mixin BFT-DAG network reference implementation, the Trusted Execution Environment is not integrated into this repository yet.

## Get Started

Install golang and setup `GOPATH` following this guide https://golang.org/doc/install.

```bash
$ git clone https://github.com/MixinNetwork/mixin.git
$ cd mixin
$ go build
```

The `mixin` command is both the kernel node and tools to communicate with the node RPC interface.

```
$ mixin

NAME:
   mixin - A free, lightning fast and decentralized network for transferring digital assets.

USAGE:
   mixin [global options] command [command options] [arguments...]

VERSION:
   v0.12.0

COMMANDS:
   kernel, k                    Start the Mixin Kernel daemon
   clone                        Clone a graph to intialize the kernel
   setuptestnet                 Setup the test nodes and genesis
   createaddress                Create a new Mixin address
   decodeaddress                Decode an address as public view key and public spend key
   decodesignature              Decode a signature
   decryptghostkey              Decrypt a ghost key with the private view key
   updateheadreference          Update the cache round external reference, never use it unless agree by other nodes
   removegraphentries           Remove data entries by prefix from the graph data storage
   validategraphentries         Validate transaction hash integration
   signrawtransaction           Sign a JSON encoded transaction
   sendrawtransaction           Broadcast a hex encoded signed raw transaction
   decoderawtransaction         Decode a raw transaction as JSON
   buildnodepledgetransaction   Build the transaction to pledge a node
   buildnodecanceltransaction   Build the transaction to cancel a pledging node
   decodenodepledgetransaction  Decode the extra info of a pledge transaction
   getroundlink                 Get the latest link between two nodes
   getroundbynumber             Get a specific round
   getroundbyhash               Get a specific round
   listsnapshots                List finalized snapshots
   getsnapshot                  Get the snapshot by hash
   gettransaction               Get the finalized transaction by hash
   getcachetransaction          Get the transaction in cache by hash
   getutxo                      Get the UTXO by hash and index
   listmintworks                List mint works
   listmintdistributions        List mint distributions
   listallnodes                 List all nodes ever existed
   getinfo                      Get info from the node
   dumpgraphhead                Dump the graph head
   help, h                      Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --node value, -n value  the node RPC endpoint (default: "127.0.0.1:8239")
   --dir value, -d value   the data directory
   --time                  print the runtime (default: false)
   --help, -h              show help (default: false)
   --version, -v           print the version (default: false)
```

## Mixin Kernel Address

Mixin Kernel address are a pair of ed25519 keys, following the [CryptoNote](https://cryptonote.org/standards/) protocol. To create a new address use the `createaddress` command.

```
$ mixin createaddress

address:	XINJkpCdwVk3qFqmS3AAAoTmC5Gm2fR3iRF7Rtt7hayuaLXNrtztS3LGPSxTmq5KQh3KJ2qYXYE5a9w8BWXhZAdsJKXqcvUr
view key:	568302b687a2fa3e8853ff35d99ffdf3817b98170de7b51e43d0dcf4fe30470f
spend key:	7c2b5c97278ed371d75610cccd9681af31b0d99be4adc2d66983f3c455fc9702
```

Share the `address` to receive assets from other Mixin Kernel addresses, and keep `view key` and `spend key` privately and securely.

Both the `view key` and `spend key` are required to spend the assets received from others, and the `view key` iteself is sufficient to decode and view all the transactions sent to `address`.


## Sign and Send Raw Transaction

Basic Mixin Kernel transaction is similar to the transaction in Bitcoin, with following format.

```json
{
  "version": 1,
  "asset": "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
  "extra": "34366362393932382d653636632d343966392d386165632d366462366137346666663638",
  "outputs": [
    {
      "type": 0,
      "amount": "115.06849309",
      "script": "fffe01",
      "accounts": [
        "XINPXu5NBXszhpZDRJ8iA26TbQ2oWTSq1tXqKKeVeYWgLSz8yXGTtVhMogynYytoMewYVFR541wauLhy1YV33zg445E49YA7"
      ]
    }
  ],
  "inputs": [
    {
      "hash": "20001842d6eff5129c11f7c053bf1209f0267bf223f1681c9cb9d19fc773a692",
      "index": 11
    }
  ]
}
```

This is the same UTXO model used in Bitcoin, but with different field names. Among them `version`, `type` and `script` should not be modified unless you know some advanced topics.

Compact the raw transaction JSON and sign it with the private view and spend key as following.

```
$ mixin signrawtransaction -n mixin-node:8239 \
    -key 0d48c96d383d325a97eea5295cbf3afa7766c49db477b68fd8032ff7f59b0b00d77e434f96f3f42c2d1796662c7cc90497feaf3863a5815f27ba49fd5e29b906 \
    -raw '{"version":1,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","extra":"34366362393932382d653636632d343966392d386165632d366462366137346666663638","outputs":[{"type":0,"amount":"115.06849309","script":"fffe01","accounts":["XINPXu5NBXszhpZDRJ8iA26TbQ2oWTSq1tXqKKeVeYWgLSz8yXGTtVhMogynYytoMewYVFR541wauLhy1YV33zg445E49YA7"]}],"inputs":[{"hash":"20001842d6eff5129c11f7c053bf1209f0267bf223f1681c9cb9d19fc773a692","index":11}]}'
```


## Start a Kernel Node

To start a node, create a directory `mixin` for the config and network data files, then put the genesis.json, nodes.json and config.toml files in it.

The main net genesis.json, nodes.json and an example config.example.toml files can be obtained from [here](https://github.com/MixinNetwork/mixin/tree/master/config), you only need to put your own signer spend key in the config.toml file.

Change the `consensus-only` option to `false` will allow the node to start in archive mode, which syncs all the graph data.

```
$ mixin help kernel

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
$ mixin setuptestnet

$ mixin kernel -dir /tmp/mixin-7001 -port 7001
$ mixin kernel -dir /tmp/mixin-7002 -port 7002
$ mixin kernel -dir /tmp/mixin-7003 -port 7003
$ mixin kernel -dir /tmp/mixin-7004 -port 7004
$ mixin kernel -dir /tmp/mixin-7005 -port 7005
$ mixin kernel -dir /tmp/mixin-7006 -port 7006
$ mixin kernel -dir /tmp/mixin-7007 -port 7007
```
