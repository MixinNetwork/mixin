# mixin

The Mixin-BFT-DAG network reference implementation, the Trusted Execution Environment is not integrated into this repository yet.

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
   clone                        Clone a graph to initialize the kernel
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

Both the `view key` and `spend key` are required to spend the assets received from others, and the `view key` itself is sufficient to decode and view all the transactions sent to `address`.


## Sign and Send Raw Transaction

Basic Mixin Kernel transaction is similar to the transaction in Bitcoin, with following format.

```json
{
  "version": 5,
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
$ mixin -n mixin-node:8239 signrawtransaction \
    -key 0d48c96d383d325a97eea5295cbf3afa7766c49db477b68fd8032ff7f59b0b00d77e434f96f3f42c2d1796662c7cc90497feaf3863a5815f27ba49fd5e29b906 \
    -raw '{"version":5,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","extra":"34366362393932382d653636632d343966392d386165632d366462366137346666663638","outputs":[{"type":0,"amount":"115.06849309","script":"fffe01","accounts":["XINPXu5NBXszhpZDRJ8iA26TbQ2oWTSq1tXqKKeVeYWgLSz8yXGTtVhMogynYytoMewYVFR541wauLhy1YV33zg445E49YA7"]}],"inputs":[{"hash":"20001842d6eff5129c11f7c053bf1209f0267bf223f1681c9cb9d19fc773a692","index":11}]}'
```


## Start a Kernel Node

To start a node, create a directory `mixin` for the config and network data files, then put the genesis.json, nodes.json and config.toml files in it.

The main net genesis.json, nodes.json and an example config.example.toml files can be obtained from [here](https://github.com/MixinNetwork/mixin/tree/master/config), you only need to put your own signer spend key in the config.toml file.

Changing the `consensus-only` option to `false` will allow the node to start in archive mode, which syncs all the graph data.

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

This will set up a minimum local test net, with all nodes in a single device.

```
$ mixin setuptestnet
=>
network:    331c1956ffb45db8a61d30a2a16e3eb0a263b844a6781253acaa07dc4ad555a8
custodian:  XINVo9oZLCzQc39QuhcmApzN23s2RjnMfABrbZvGPsB74SsYqhCYLFsXSULrLs4rokgGXNY5oUZvVm7ZQgHzBv7PPPRW7kFm
view key:   95974a5f8b90d685527f246605e13968dc73f3bc9a72e6db23128220a6579e08
spend key:  109c54394e977d46c2c6385faf05b4a25024ddb476588529b889f6d71912ab0b
```

The output above indicates the network id and default custodian key for the test net. The custodian key should be kept well for future deposit of the test net.

To boot the test net, just launch the node with each configuration directory.

```
$ mixin kernel -dir /tmp/mixin-6861
$ mixin kernel -dir /tmp/mixin-6862
$ mixin kernel -dir /tmp/mixin-6863
$ mixin kernel -dir /tmp/mixin-6864
$ mixin kernel -dir /tmp/mixin-6865
$ mixin kernel -dir /tmp/mixin-6866
$ mixin kernel -dir /tmp/mixin-6867
```

Then we can generate a test address and deposit some money into the address.

```
$ mixin createaddress
=>
address:    XINBpBaDKtcu5SBuuqE1pwMrdeFvyCwFChdjLS23ewKdoLbURL4iJYCXLvsXP1nVbB3CGRbWg6UgVH8AWVgjSgmenMsrgpRY
view key:   be25d97ba8eb80336998facfb033f71713f9c1cabdc417478f831c749fcd9001
spend key:  5189b286e5717ea36435a29dfb1aaddebad5e216b05e8b4de59693dfb9fe1f06

$ mixin signcustodiandeposit -custodian CUSODIANPRIVATEVIEWPRIVATESPEND \
      -receiver XINBpBaDKtcu5SBuuqE1pwMrdeFvyCwFChdjLS23ewKdoLbURL4iJYCXLvsXP1nVbB3CGRbWg6UgVH8AWVgjSgmenMsrgpRY \
      -asset a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc \
      -chain 8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27 \
      -asset_key 0xa974c709cfb4566686553a20790685a47aceaa33 \
      -transaction 0x13f805f2593c59becd0b89673390249415f833fc9c821288f69cce5e7c6eb09f \
      -index 0 \
      -amount 100.123
=>
77770005a99c2e0e2b1da4d648...3c6cfb57f8cd1830c

$ mixin -n 127.0.0.1:6861 sendrawtransaction -raw 77770005a99c2e0e2b1da4d648...3c6cfb57f8cd1830c
=>
{"hash":"bf7f2bdbed2f77c452e91febd48c4a8f876e42895737d2616f9da956fb622888"}
```
