# Mixin Kernel Remote Procedure Calls

> A free and lightning fast peer-to-peer transactional network for digital assets.

Mixin Kernel RPCs accept multiple subcommand and interactive with the network.

### Quick Reference

* [kernel, k](#kernel): Start the Mixin Kernel daemon.
* [setuptestnet](#setuptestnet): Setup the test nodes and genesis.
* [createaddress](#createaddress): Create a new Mixin address
* [decodeaddress](#decodeaddress): Decode an address as public view key and public spend key.
* [decryptghostkey](#decryptghostkey):  Decrypt a ghost key with the private view key.
* [updateheadreference](#updateheadreference): Update the cache round external reference, never use it unless agree by other nodes.
* [removegraphentries](#removegraphentries): Remove data entries by prefix from the graph data storage.
* [validategraphentries](#validategraphentries): Validate transaction hash integration.
* [signrawtransaction](#signrawtransaction): Sign a JSON encoded transaction.
* [sendrawtransaction](#sendrawtransaction): Broadcast a hex encoded signed raw transaction.
* [decoderawtransaction](#decoderawtransaction): Decode a raw transaction as JSON.
* [buildnodecanceltransaction](#buildnodecanceltransaction): Build the transaction to cancel a pledging node.
* [decodenodepledgetransaction](#decodenodepledgetransaction): Decode the extra info of a pledge transaction.
* [getroundlink](#getroundlink): Get the latest link between two nodes.
* [getroundbynumber](#getroundbynumber): Get a specific round.
* [getroundbyhash](#getroundbyhash): Get a specific round.
* [listsnapshots](#listsnapshots): List finalized snapshots.
* [getsnapshot](#getsnapshot): Get the snapshot by hash.
* [gettransaction](#gettransaction): Get the finalized transaction by hash.
* [getcachetransaction](#getcachetransaction): Get the transaction in cache by hash.
* [getutxo](#getutxo): Get the UTXO by hash and index.
* [listmintdistributions](#listmintdistributions): List mint distributions.
* [listallnodes](#listallnodes): List all nodes ever existed.
* [getinfo](#getinfo): Get info from the node.
* [dumpgraphhead](#dumpgraphhead): Dump the graph head.
* [help, h](#help): Shows a list of commands or help for one command.

### Command

#### Global Options

> mixin kernel glbal options.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| node    | string  | Optional, Default="127.0.0.1:8239" | the node RPC endpoint |
| dir     | string  | Optional  | the data directory                      |
| time    | boolean | Optional, Default=false |  print the runtime        |
| help    | boolean | Optional, Default=false |  show help                |
| version | boolean | Optional, Default=false |  print the version        |

#### kernel

> Start the Mixin Kernel daemon.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| dir     | string  | Required  | the data directory                      |
| port    | integer | Required, Default=7239  | the peer port to listen   |
| log     | integer | Required, Default=2  | the log level                |
| limiter | integer | Required, Default=0  | limit the log count for the same content, 0 means no limit |
| filter  | string  | Optional  | the RE2 regex pattern to filter log     |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | mixin kernel running log |

*Example*

``` bash
mixin kernel -dir /data/mixin -port 7239
```

*See also*

* [Mixin Kernel Node Operations](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-node-operations.md)

#### setuptestnet

> Setup the test nodes and genesis.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean    | Optional, Default=false  | show help             |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | testnet details                         |

*Example*

``` bash
mixin setuptestnet
```

*See also*

* [Mixin Kernel Node Operations](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-node-operations.md)

#### createaddress

> Create a new Mixin address.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| public  | boolean | Optional, Default=false | whether mark all my transactions public |
| view    | string  | Optional  | the private view key HEX instead of a random one |
| spend   | string  | Optional  | the private spend key HEX instead of a random one |
| help    | boolean    | Optional, Default=false  | show help              |

*Result---`string` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | string    | Required  | mixin address with private view key and private spend key |

*Example*

``` bash
mixin createaddress
address:        XINAjW8w3f3FPz2BKFcJgRGG5Ut4A8njaMRxmGVroWojSYwHsAf5gYFzTB92FarMUz7TN6XjyHnJuGDYf6JttsVR9V9uHBNZ
view key:       8b61a5d273132292e56c9b58f98f6d7516e8bdc436ec40ca1699e2f76b2dd200
spend key:      c8c256b6e5722dca41839cf6714dd1ea724b31b390ac54ab5ff790e70401000c
```

#### decodeaddress

> Decode an address as public view key and public spend key.

*Parameter*

| Name    | Type    | Presence  | Description                            |
| :-----: |:-------:| :-----    | :------------------------------------  |
| address | string  | Required  | the Mixin Kernel address               |
| help    | boolean | Optional, Default=false  | show help               |

*Result---`string` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | string    | Required  | mixin address with public view key, public spend key, spend derive private and spend derive public |

*Example*

``` bash
mixin decodeaddress \
--address XINAjW8w3f3FPz2BKFcJgRGG5Ut4A8njaMRxmGVroWojSYwHsAf5gYFzTB92FarMUz7TN6XjyHnJuGDYf6JttsVR9V9uHBNZ
public view key:        83823fc961dff3fa66db1468ded2d0d8724aef8533df188439e55cc9500360c3
public spend key:       4a64c21bd6f912ea87949dfee65d4a075d5875eb0e83f82edd7d1fb517f88625
spend derive private:   73b14767b05931a683d3a69624670558629d358bd6958a3f6f643ced95e35a00
spend derive public:    462d454a3601655c249c0cbfeb32fa2e899455d9805a1873c84bb532650a872d
```

#### decryptghostkey

> Decrypt a ghost key with the private view key.

*Parameter*

| Name    | Type    | Presence  | Description                            |
| :-----: |:-------:| :-----    | :------------------------------------  |
| view    | string  | Required  | the private view key                   |
| key     | string  | Required  | the ghost key                          |
| mask    | string  | Required  | the ghost mask                         |
| index   | integer | Required, Default=0 | the output index             |
| help    | boolean | Optional, Default=false  | show help               |

*Result---`string` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | string    | Required  | decrypted ghost key |

*Example*

``` bash
mixin decryptghostkey --view VIEW --key KEY --mask MASK
```

#### updateheadreference

> Update the cache round external reference, never use it unless agree by other nodes.

*Parameter*

| Name     | Type    | Presence  | Description                          |
| :------: |:-------:| :-----    | :---------------------------------   |
| id       | string  | Required  | self node ID                         |
| round    | integer | Required, Default=0 | self cache round NUMBER    |
| external | string  | Required  | the external reference HEX           |
| help     | boolean | Optional, Default=false  | show help             |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | `null` when the command was successfull or with an error |

*Example*

``` bash
mixin --dir /data/mixin updateheadreference \
--id cbba7a5e7bae3b0cef3d6dcba7948fa03facda3be401d67aa1a38aecb1f443a0 \
--round 21163 \
--external 65d45bf2cbf005977dfd7c666a6bbc35b56e28f473560a0d6ee83578142e25cb
```

#### removegraphentries

> Remove data entries by prefix from the graph data storage.

*Parameter*

| Name    | Type    | Presence  | Description                            |
| :-----: |:-------:| :-----    | :------------------------------------  |
| prefix  | string  | Required  | the entry prefix                       |
| help    | boolean | Optional, Default=false  | show help               |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | `null` when the command was successfull or with an error |

*Example*

``` bash
mixin --dir /data/mixin removegraphentries --prefix PREFIX
```

#### validategraphentries

> Validate transaction hash integration.

*Parameter*

| Name    | Type    | Presence  | Description                           |
| :-----: |:-------:| :-----    | :------------------------------------ |
| help    | boolean | Optional, Default=false  | show help              |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | `null` when the command was successfull or with an error |

*Example*

``` bash
mixin --dir /data/mixin validategraphentries
```

#### signrawtransaction

> Sign a JSON encoded transaction.

*Parameter*

| Name    | Type    | Presence  | Description                                 |
| :-----: |:-------:| :-----    | :-----------------------------------------  |
| raw     | string  | Required  | the JSON encoded raw transaction            |
| key     | string  | Required  | the private key to sign the raw transaction |
| seed    | string  | Required  | the mask seed to hide the recipient public key |
| help    | boolean | Optional, Default=false  | show help                    |

*Result---`string` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | string  | Required  | signed transaction |

*Example*

``` bash
mixin -n 127.0.0.1:8239 signrawtransaction \
--key VIEWSPEND \
--raw \
'{
  "version": 1,
  "asset": "b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8",
  "inputs": [
    {
      "hash": "4db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6",
      "index": 0
    }
  ],
  "outputs": [
    {
      "type": 0,
      "amount": "1",
      "script": "fffe01",
      "keys": ["4a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562"],
      "mask": "2b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309"
    }
  ],
  "extra": ""
}'
86a756657273696f6e01a54173736574c420b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8a6496e707574739185a448617368c4204db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739185a45479706500a6416d6f756e74d60005f5e100a44b65797391c4204a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562a6536372697074c403fffe01a44d61736bc4202b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309a54578747261c400aa5369676e6174757265739191c4409f5a5e063532ba010005d8c1f6d35d3905a24a6a12d15e02b2717386efdbe2b1127e44e1b545860b21f76ef05591e08cb35738d2a66a067c2eb81e591e1e7f01
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### sendrawtransaction

> Broadcast a hex encoded signed raw transaction.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the hex encoded signed raw transaction  |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | transaction hash |

*Example*

``` bash
mixin -n 127.0.0.1:8239 sendrawtransaction \
--raw 86a756657273696f6e01a54173736574c420b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8a6496e707574739185a448617368c4204db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739185a45479706500a6416d6f756e74d60005f5e100a44b65797391c4204a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562a6536372697074c403fffe01a44d61736bc4202b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309a54578747261c400aa5369676e6174757265739191c4409f5a5e063532ba010005d8c1f6d35d3905a24a6a12d15e02b2717386efdbe2b1127e44e1b545860b21f76ef05591e08cb35738d2a66a067c2eb81e591e1e7f01
{
  "hash": "c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35"
}
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### decoderawtransaction

> Decode a raw transaction as JSON.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the JSON encoded raw transaction        |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | transaction details with json           |

*Example*

``` bash
mixin -n 127.0.0.1:8239 decoderawtransaction \
--raw 86a756657273696f6e01a54173736574c420b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8a6496e707574739185a448617368c4204db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739185a45479706500a6416d6f756e74d60005f5e100a44b65797391c4204a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562a6536372697074c403fffe01a44d61736bc4202b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309a54578747261c400aa5369676e6174757265739191c4409f5a5e063532ba010005d8c1f6d35d3905a24a6a12d15e02b2717386efdbe2b1127e44e1b545860b21f76ef05591e08cb35738d2a66a067c2eb81e591e1e7f01
{
  "asset": "b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8",
  "extra": "",
  "hash": "c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35",
  "inputs": [
    {
      "hash": "4db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6",
      "index": 0
    }
  ],
  "outputs": [
    {
      "amount": "1.00000000",
      "keys": [
        "4a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562"
      ],
      "mask": "2b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309",
      "script": "fffe01",
      "type": 0
    }
  ],
  "signatures": [
    [
      "9f5a5e063532ba010005d8c1f6d35d3905a24a6a12d15e02b2717386efdbe2b1127e44e1b545860b21f76ef05591e08cb35738d2a66a067c2eb81e591e1e7f01"
    ]
  ],
  "version": 1
}
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### buildnodecanceltransaction

> Build the transaction to cancel a pledging node.

*Parameter*

| Name     | Type    | Presence  | Description                              |
| :------: |:-------:| :-----    | :--------------------------------------  |
| view     | string  | Required  | the private view key which signs the pledging transaction |
| spend    | string  | Required  | the private spend key which signs the pledging transaction |
| receiver | string  | Required  | the address to receive the refund         |
| pledge   | string  | Required  | the hex of raw pledge transaction         |
| source   | string  | Required  | the hex of raw pledging input transaction |
| help     | boolean | Optional, Default=false  | show help                  |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | `null` when the command was successfull or with an error |

*Example*

``` bash
mixin -n 127.0.0.1:8239 \
--view VIEW \
--spend SPEND \
--receiver XINFP2byiRvSbWbxH6k4TvbkDiwLLEzg6wLLGGy6ypajFQafcbxQpsqLWq8YhZwuHGAHURxW5Ap69iN5a81e1NNpmq8K4cgR
--pledge PLEDGEHASH \
--source SOURCEHASH
```

#### decodenodepledgetransaction

> Decode the extra info of a pledge transaction.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the raw pledge transaction              |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | `null` when the command was successfull or with an error |

*Example*

``` bash
mixin -n 127.0.0.1:8239 --raw RAW
```

#### getroundlink

> Get the latest link between two nodes.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| from    | string  | Required  | the reference head                      |
| to      | string  | Required  | the reference tail                      |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | Latest link details between two nodes   |

*Example*

``` bash
mixin -n 127.0.0.1:8239 --from HEAD --to TAIL
```

#### getroundbynumber

> Get a specific round.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| id      | string  | Required  | the round node id                       |
| number  | integer | Required, Default=0 | the round number              |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | round details with json                 |

*Example*

``` bash
mixin -n 127.0.0.1:8239 getroundbynumber \
--id 4a625fb4abd5b2bea6f1a1135af0a3ff20ab8b6928e1690cabd7775d406b78e3 \
--number 1
{
  "end": 1559572201613551600,
  "hash": "6af6ba2f8980ed6b6b02de0a53c3a74b0c6cc0e8d9738d16d27edf6a3d69f8f7",
  "node": "4a625fb4abd5b2bea6f1a1135af0a3ff20ab8b6928e1690cabd7775d406b78e3",
  "number": 1,
  "references": {
    "external": "d6d42d8c77fa0f33f5ad03f284eb241b9a28fb5ba30bb2430c9c053d1eca3cf3",
    "self": "0f1a9af515b015a3078dae60b8cc271345eeea8740eef0e0a1b85fd6441596ec"
  },
  "snapshots": [
    {
      "hash": "1e84283cf889625e4d944c27467ff5da0ad174ec661b5abac0d8af3f54a43ec0",
      "node": "4a625fb4abd5b2bea6f1a1135af0a3ff20ab8b6928e1690cabd7775d406b78e3",
      "references": {
        "external": "d6d42d8c77fa0f33f5ad03f284eb241b9a28fb5ba30bb2430c9c053d1eca3cf3",
        "self": "0f1a9af515b015a3078dae60b8cc271345eeea8740eef0e0a1b85fd6441596ec"
      },
      "round": 1,
      "timestamp": 1559572201613551600,
      "topology": 5900546,
      "transaction": "9568037a5768b594c8dd7c03c9beadd562e158489e821d3d263bc3ff1732d7ab",
      "version": 0
    }
  ],
  "start": 1559572201613551600
}
```

#### getroundbyhash

> Get a specific round.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the round hash                          |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | round details with json                 |

*Example*

``` bash
mixin -n 127.0.0.1:8239 --hash HASH
```

#### listsnapshots

> List finalized snapshots.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| since   | integer | Required, Default=0 | the topological order to begin with |
| count   | integer | Required, Default=10 | the up limit of the returned snapshots |
| sig     | boolean | Optional, Default=false | whether including the signatures |
| tx      | boolean | Optional, Default=false | whether including the transactions |
| help    | boolean | Optional, Default=false | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | finalized snapshots details with json   |

*Example*

``` bash
mixin -n 127.0.0.1:8239 listsnapshots --since 0 --count 1 --sig --tx
[
  {
    "hash": "75eabab3b5e3fe0a811bc2969f32716cc58bac7260b112380be45a23fc839939",
    "node": "a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50",
    "references": null,
    "round": 0,
    "signatures": null,
    "timestamp": 1551312000000000000,
    "topology": 0,
    "transaction": {
      "asset": "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
      "extra": "065866c10424d5cfa7ca95eddad69d261ddc7c31a107f28773880bd9cb5bf611c70a3825ca359993324db9e169acb832e9ca75ec17b2b2e1f5b10ebd40eb9dca",
      "hash": "f3a94f83f0a579d1a1b87f713d934df44e9b888216938667e7b2817aba71ef93",
      "inputs": [
        {
          "genesis": "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
        }
      ],
      "outputs": [
        {
          "amount": "10000.00000000",
          "keys": [
            "1d2dced65983ef59ea096d75a27a276308f1ae717c66f1884125adedfda3ae3d",
            "ec7d399503241bf26975719df8152feb599afb85c8cf3cc4175761421fb4c2ca",
            "a5ada6adecdc3bbb8aeb128ba8ddc3f6cb3022406de5576f3d15a38e926f0b96",
            "d1913a811ea696961a0d253359f9590efd77519d6005a6326a47435589e3c909",
            "3796347874919f62625a8db893d254b4292248f84504e7a5c766994c6251aea9",
            "e566095e3fb7949ec2fef418c2a097bc1609ac5adde2401974d7d449ae31190b",
            "2486621dc6c86300a60f2a46a910771f267dba698609aa686aa76d630e58e727",
            "71b3238bb152e5c63386af6bfd27bcfdd677436bb8e70520535fac2087bc5452",
            "9cd1704f830d035f7917e0a7eaf79b873abb715b00f0a2713205d2660f4b533b",
            "967407727188086d3ac67811603e073743945c372103323898a15004da0503d8",
            "4011b7a390e3c514c9da9341fbe461e3398e1538b9647ecafe1a95a74cebdefd",
            "52dae8ec6e0abaab28902f7427163de375e618aaf012d5a5c4ef4629d0b32d1d",
            "7a51cc274ea7dbc39bd81737b952aeea2f3bfaba55313b9a239bdd7e1f8f792e",
            "682ecb376c5af616b20c653fadc59e5c3992ee4ad6ef10b1f4cbe429b2f8e9fb",
            "0359fd509abff274bf7f8eca839ea17ec33c455478c4d1088936f8ff58a71705"
          ],
          "mask": "1502ba20afb0fa88b64ff9fbd8f12615da0fcd57f2132a3af712fee103d5ddeb",
          "script": "fffe0b",
          "type": 164
        }
      ],
      "version": 0
    },
    "version": 0
  }
]
```

*See also*

* [Mixin Kernel Snapshots](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-snapshots.md)

#### getsnapshot

> Get the snapshot by hash.

*Parameter*

| Name    | Type    | Presence  | Description                            |
| :-----: |:-------:| :-----    | :------------------------------------  |
| hash    | string  | Required  | the snapshot hash                      |
| help    | boolean | Optional, Default=false  | show help               |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | finalized snapshots details with json   |

*Example*

``` bash
mixin -n 127.0.0.1:8239 getsnapshot \
--hash 75eabab3b5e3fe0a811bc2969f32716cc58bac7260b112380be45a23fc839939
{
  "hash": "75eabab3b5e3fe0a811bc2969f32716cc58bac7260b112380be45a23fc839939",
  "node": "a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50",
  "references": null,
  "round": 0,
  "signatures": null,
  "timestamp": 1551312000000000000,
  "topology": 0,
  "transaction": {
    "asset": "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
    "extra": "065866c10424d5cfa7ca95eddad69d261ddc7c31a107f28773880bd9cb5bf611c70a3825ca359993324db9e169acb832e9ca75ec17b2b2e1f5b10ebd40eb9dca",
    "hash": "f3a94f83f0a579d1a1b87f713d934df44e9b888216938667e7b2817aba71ef93",
    "inputs": [
      {
        "genesis": "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
      }
    ],
    "outputs": [
      {
        "amount": "10000.00000000",
        "keys": [
          "1d2dced65983ef59ea096d75a27a276308f1ae717c66f1884125adedfda3ae3d",
          "ec7d399503241bf26975719df8152feb599afb85c8cf3cc4175761421fb4c2ca",
          "a5ada6adecdc3bbb8aeb128ba8ddc3f6cb3022406de5576f3d15a38e926f0b96",
          "d1913a811ea696961a0d253359f9590efd77519d6005a6326a47435589e3c909",
          "3796347874919f62625a8db893d254b4292248f84504e7a5c766994c6251aea9",
          "e566095e3fb7949ec2fef418c2a097bc1609ac5adde2401974d7d449ae31190b",
          "2486621dc6c86300a60f2a46a910771f267dba698609aa686aa76d630e58e727",
          "71b3238bb152e5c63386af6bfd27bcfdd677436bb8e70520535fac2087bc5452",
          "9cd1704f830d035f7917e0a7eaf79b873abb715b00f0a2713205d2660f4b533b",
          "967407727188086d3ac67811603e073743945c372103323898a15004da0503d8",
          "4011b7a390e3c514c9da9341fbe461e3398e1538b9647ecafe1a95a74cebdefd",
          "52dae8ec6e0abaab28902f7427163de375e618aaf012d5a5c4ef4629d0b32d1d",
          "7a51cc274ea7dbc39bd81737b952aeea2f3bfaba55313b9a239bdd7e1f8f792e",
          "682ecb376c5af616b20c653fadc59e5c3992ee4ad6ef10b1f4cbe429b2f8e9fb",
          "0359fd509abff274bf7f8eca839ea17ec33c455478c4d1088936f8ff58a71705"
        ],
        "mask": "1502ba20afb0fa88b64ff9fbd8f12615da0fcd57f2132a3af712fee103d5ddeb",
        "script": "fffe0b",
        "type": 164
      }
    ],
    "version": 0
  },
  "version": 0
}
```

*See also*

* [Mixin Kernel Snapshots](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-snapshots.md)

#### gettransaction

> Get the finalized transaction by hash.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | finalized transaction hash details with json |

*Example*

``` bash
mixin -n 127.0.0.1:8239 gettransaction \
--hash c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35
{
  "asset": "b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8",
  "extra": "",
  "hash": "c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35",
  "hex": "86a756657273696f6e01a54173736574c420b9f49cf777dc4d03bc54cd1367eebca319f8603ea1ce18910d09e2c540c630d8a6496e707574739185a448617368c4204db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739185a45479706500a6416d6f756e74d60005f5e100a44b65797391c4204a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562a6536372697074c403fffe01a44d61736bc4202b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309a54578747261c400aa5369676e6174757265739191c4409f5a5e063532ba010005d8c1f6d35d3905a24a6a12d15e02b2717386efdbe2b1127e44e1b545860b21f76ef05591e08cb35738d2a66a067c2eb81e591e1e7f01",
  "inputs": [
    {
      "hash": "4db8bf0626a61e5026b570e9dd19c05528ae5d50d64973bfe250c1e2da1c79c6",
      "index": 0
    }
  ],
  "outputs": [
    {
      "amount": "1.00000000",
      "keys": [
        "4a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562"
      ],
      "mask": "2b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309",
      "script": "fffe01",
      "type": 0
    }
  ],
  "snapshot": "9a4b79aee55b538b6da98c167a9135cbb5c31446ca880e29ac2c0e470a38ec5c",
  "version": 1
}
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### getcachetransaction

> Get the transaction in cache by hash.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | transaction in cache hash details with json |

*Example*

``` bash
mixin -n 127.0.0.1:8239 getcachetransaction --hash HASH
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### getutxo

> Get the UTXO by hash and index.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| index   | integer | Required, Default=0 | the output index              |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | UTXO hash details with json             |

*Example*

``` bash
mixin -n 127.0.0.1:8239 getutxo \
--hash c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35 \
--index 0
{
  "amount": "1.00000000",
  "hash": "c647a2ae5973550a91525ad683c346791a144649577c022d28634f1cb02b4b35",
  "index": 0,
  "keys": [
    "4a2bd5869e6bec65a33e831ca46815ed277ddb5e63536f9e429ebbc6f64ee562"
  ],
  "mask": "2b51d09441893afc59bd440c3aab1fe746435b030dee4155c6bba9b7ff67e309",
  "script": "fffe01",
  "type": 0
}
```

#### listmintdistributions

> List mint distributions.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| since   | integer | Required, Default=0  | the mint batch to begin with |
| count   | integer | Required, Default=10 | the up limit of the returned distributions |
| tx      | boolean | Optional, Default=false | whether including the transactions |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | mint distributions details with json    |

*Example*

``` bash
mixin -n 127.0.0.1:8239 listmintdistributions \
--since 0 \
--count 1 \
--tx
[
  {
    "amount": "1726.02739638",
    "batch": 14,
    "group": "KERNELNODE",
    "transaction": {
      "asset": "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
      "extra": "",
      "hash": "20001842d6eff5129c11f7c053bf1209f0267bf223f1681c9cb9d19fc773a692",
      "inputs": [
        {
          "mint": {
            "amount": "1726.02739638",
            "batch": 14,
            "group": "KERNELNODE"
          }
        }
      ],
      "outputs": [
        {
          "amount": "115.06849309",
          "keys": [
            "5cd87b6b5a25f67445197261e1ebb5d68be598cd63b0a57eef6897f82cde5c0a"
          ],
          "mask": "f287afceabccc3d48b52de04d0edd43b446275041b024a3b5c9517894c06f9ab",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "3347abcbbc2d94b14c05f02a74c1dd13d8348ff65925b6404e5f5130411318aa"
          ],
          "mask": "a97b97f871e8d780d9dc358bbada732d167c124466fb6faf959a941e52069afc",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "899df857ef5d95a364179f76410d07be3500c86628d6e06a257f71eb4035701b"
          ],
          "mask": "7f981c6f6d25f8e71e5b7b4f3e355d244a94a9950292142729c54ac82b2d7b67",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "58fbc1fde3455980569693911e4c0e0e5b4c5d42e4a2614f482a26d8a3b2fe1e"
          ],
          "mask": "7615a41f4701c13430b13abd55873ea06302dba86bffec0249cd67708c4fb2df",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "aabd4523ee745cf03e601477d7609bf8d2840708e46a1c9726c5e570795429ee"
          ],
          "mask": "2ddc77fe0dfab107b4620ec903754fd40aa59c222931da90333a5fbf5f4e3dac",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "566326946a68d11c12d74fa4adfc42834bb8984c2c6174757c4ec20bee323b39"
          ],
          "mask": "dc5cb50ccabe9251212b8a7067375aef4a0e83822117b9a7bd853a86f2cdab43",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "c06e9f1a7a301a7934af5679f4772d405f808de2793b7324d065abe1b329be53"
          ],
          "mask": "815e83bad45043fd28e9e2267357385922a6d7c59a48e1326655882e6679f182",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "f9b1be4502cef1374721538452b6122e1806e18b5507ac034cb1083e5bba34ed"
          ],
          "mask": "43fed0726a7b399709def250823bcd460ee1de3c9f3d995d8b259571a2d1ed86",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "0f686ca4525192f9d9d014b0f014d3a897c6ea4a6a3ecac1bebd574961c55e75"
          ],
          "mask": "8e815203457d3fef9244d51b839093c29ebd0ee0e29dd7558e56ab6c1950fb32",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "18faaa1441a064c47d4a31c231990632c8c074ab379571b46d8fc675e991ad42"
          ],
          "mask": "9d8b8e3c2e221ec505283dd7e266774cc7ab3c0501593825c9788ca8adb689a9",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "c25e17e7fc30947d9f0d482b079357b333e45e76cb9d788250f8141e43c1ee27"
          ],
          "mask": "0d1c0f5db5ea025531182ceca3a804c3224eb5a8141714d136e6c28464431272",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "eac842a3d78a95484d57968fe4aca201db59aaaea97e1bd61de62f86b56ba3c2"
          ],
          "mask": "6736872b536d970d410f9633294823c112d58aa904eba90746007098d2bdf19f",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "f3919a3ee45735c88255e1d13e6c61189fc065387c17a344d57a54dc0b13635c"
          ],
          "mask": "314108551c39156912d2826f9973705bb7ebb2393f63f3596b9bdf1ec3748ba9",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "84d2c08f07e7d57019c9dbf44a45c2f7cd2f2caba387470ab6b2d5d846f8e704"
          ],
          "mask": "2c93c7dfb881aaa1b966b5e73340a81398760a483e622baedbf13102fc30e2f5",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "115.06849309",
          "keys": [
            "9b61763ab495b0cdebff497e29f3f4a0566bc7435a872caa364d417eb67a75da"
          ],
          "mask": "198965050adf1a212b5347bc299802603902586a226a7be74f0f2d776cdcb30f",
          "script": "fffe01",
          "type": 0
        },
        {
          "amount": "0.00000003",
          "keys": [
            "d5b968a266ce9293c6692b0b65c8af4f9270be06081f5027837fe54e1fe33c28"
          ],
          "mask": "9724fa3f438894f381683942ecea361dc2ce4d407f1f0fa0448122c811e218a1",
          "script": "fffe40",
          "type": 0
        }
      ],
      "version": 1
    }
  }
]
```

#### listallnodes

> List all nodes ever existed.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | all existed nodes with json             |

*Example*

``` bash
mixin -n 127.0.0.1:8239 listallnodes
```

#### getinfo

> Get info from the node.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | node information with json              |

*Example*

``` bash
mixin -n 127.0.0.1:8239 getinfo
```

#### dumpgraphhead

> Dump the graph head.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`json` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | json    | Required  | graph head details with json            |

*Example*

``` bash
mixin -n 127.0.0.1:8239 dumpgraphhead
```

#### help

> Shows a list of commands or help for one command.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result---`null` on success*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| result  | null    | Required  | print all command which mixin kernel support |

*Example*

``` bash
mixin --help
```