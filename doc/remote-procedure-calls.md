# Mixin Kernel Remote Procedure Calls

> A free and lightning fast peer-to-peer transactional network for digital assets.

Mixin Kernel RPCs accept multiple subcommand and interactive with the network.

### Quick Reference

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

### Command

#### Global Options

mixin kernel glbal options.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| node    | string  | Optional, Default="127.0.0.1:8239" | the node RPC endpoint |
| dir     | string  | Optional  | the data directory                      |
| time    | boolean | Optional, Default=false |  print the runtime        |
| help    | boolean | Optional, Default=false |  show help                |
| version | boolean | Optional, Default=false |  print the version        |

#### signrawtransaction

Sign a JSON encoded transaction.

*Parameter*

| Name    | Type    | Presence  | Description                                 |
| :-----: |:-------:| :-----    | :-----------------------------------------  |
| raw     | string  | Required  | the JSON encoded raw transaction            |
| key     | string  | Required  | the private key to sign the raw transaction |
| seed    | string  | Required  | the mask seed to hide the recipient public key |
| help    | boolean | Optional, Default=false  | show help                    |

*Result*

``` bash
{
   "raw", (string) signed transaction raw
}
```

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

Broadcast a hex encoded signed raw transaction.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the hex encoded signed raw transaction  |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
    "hash": "hash", (string) broadcasted transaction.
}
```

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

Decode a raw transaction as JSON.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the JSON encoded raw transaction        |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
  "asset": "assets", (string) HEX representation of a 32 bytes hash, which is a unique asset identifier, e.g. BTC or XIN.
  "extra": "extra", (string) HEX representation of at most 256 bytes data.
  "hash": "hash", (string) HEX representation of a 32 bytes hash, which is the unique transaction identifier.
  "inputs": [
    {
      "hash": "input hash", (string) input transction hash
      "index": index (integer) input transaction index
    }
  ], (array) an array of input objects, which may be the outputs of previous transactions, Kernel mint reward, Domain deposit or Genesis.
  "outputs": [
    {
      "amount": "amount", (string) a string decimal, always rounded to 8 decimal places.
      "keys": ["keys"], (array) array of HEX representation of 32 bytes key, which are the owner of this output and called ghost keys.
      "mask": "mask", (string) HEX representation of 32 bytes key, which is used to parse the ghost keys.
      "script": "script", (string) HEX representation of {0xff, 0xfe, T}, while 0 <= T <= 0x40, where T is the required number of signatures from keys to spend this output.
      "type": type (integer) a uint8 number to constraint when and how this output can be spent as an input, usually 0 which means it can be spent once the script fulfilled.
    }
  ], (array) an array of output objects, which can be used as the inputs of future transactions.
  "signatures": [
    [
      "signatures" (string) hash signatures
    ]
  ],
  "version": version [integer] a uint8 number to hint the current transaction format.
}
```

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

Build the transaction to cancel a pledging node.

*Parameter*

| Name     | Type    | Presence  | Description                              |
| :------: |:-------:| :-----    | :--------------------------------------  |
| view     | string  | Required  | the private view key which signs the pledging transaction |
| spend    | string  | Required  | the private spend key which signs the pledging transaction |
| receiver | string  | Required  | the address to receive the refund         |
| pledge   | string  | Required  | the hex of raw pledge transaction         |
| source   | string  | Required  | the hex of raw pledging input transaction |
| help     | boolean | Optional, Default=false  | show help                  |

*Result*

See also [signrawtransaction](#signrawtransaction).

*Example*

``` bash
mixin -n 127.0.0.1:8239 \
--view VIEW \
--spend SPEND \
--receiver XINFP2byiRvSbWbxH6k4TvbkDiwLLEzg6wLLGGy6ypajFQafcbxQpsqLWq8YhZwuHGAHURxW5Ap69iN5a81e1NNpmq8K4cgR
--pledge PLEDGEHASH \
--source SOURCEHASH
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### decodenodepledgetransaction

Decode the extra info of a pledge transaction.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| raw     | string  | Required  | the raw pledge transaction              |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

See also [signrawtransaction](#signrawtransaction).

*Example*

``` bash
mixin -n 127.0.0.1:8239 --raw RAW
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### getroundlink

Get the latest link between two nodes.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| from    | string  | Required  | the reference head                      |
| to      | string  | Required  | the reference tail                      |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

See also [getroundbynumber](#getroundbynumber).

*Example*

``` bash
mixin -n 127.0.0.1:8239 --from HEAD --to TAIL
```

#### getroundbynumber

Get a specific round.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| id      | string  | Required  | the round node id                       |
| number  | integer | Required, Default=0 | the round number              |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
  "end": end,
  "hash": "hash", (string) HEX representation of a 32 bytes hash, which is the unique snapshot identifier.
  "node": "node", (string) HEX representation of a 32 bytes hash, which is the node id which leads this snapshot.
  "number": number,
  "references": {
    "external": "external",
    "self": "self"
  }, (array) the previous round hashes of the leading node and another node conforms to the consensus.
  "snapshots": [
    {
      "hash": "hash",
      "node": "node",
      "references": {
        "external": "external",
        "self": "self"
      },
      "round": round, (integer) a uint64 round number of this snapshot, round is similar to the block of Bitcoin.
      "timestamp": timestamp, (timestamp) a uint64 nanosecond since Unix epoch, which is provided by the leading node and agreed upon consensus.
      "topology": topology, (integer) a uint64 number as the snapshot order in all snapshots, this value is the only node provided value, NOT included in the hash and not agreed by consensus.
      "transaction": "transaction", (string) HEX representation of a 32 bytes hash, which is the transaction hash included by this snapshot.
      "version": version (integer) HEX representation of a 32 bytes hash, which is the transaction hash included by this snapshot.
    }
  ],
  "start": start
}
```

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

Get a specific round.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the round hash                          |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

See also [getroundbynumber](#getroundbynumber).

*Example*

``` bash
mixin -n 127.0.0.1:8239 --hash HASH
```

#### listsnapshots

List finalized snapshots.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| since   | integer | Required, Default=0 | the topological order to begin with |
| count   | integer | Required, Default=10 | the up limit of the returned snapshots |
| sig     | boolean | Optional, Default=false | whether including the signatures |
| tx      | boolean | Optional, Default=false | whether including the transactions |
| help    | boolean | Optional, Default=false | show help                |

*Result*

``` bash
[
  {
    "hash": "hash",
    "node": "node",
    "references": references,
    "round": round,
    "signatures": signatures,
    "timestamp": timestamp,
    "topology": topology,
    "transaction": {
      "asset": "asset",
      "extra": "extra",
      "hash": "hash",
      "inputs": [
        {
          "genesis": "genesis"
        }
      ],
      "outputs": [
        {
          "amount": "amount",
          "keys": [
            "keys"
          ],
          "mask": "mask",
          "script": "script",
          "type": type
        }
      ],
      "version": version
    },
    "version": version
  }
]
```

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

*Result*

See also [getroundbynumber](#getroundbynumber).

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

Get the finalized transaction by hash.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
  "asset": "asset",
  "extra": "extra",
  "hash": "hash",
  "hex": "hex",
  "inputs": [
    {
      "hash": "hash",
      "index": index
    }
  ],
  "outputs": [
    {
      "amount": "amount",
      "keys": [
        "keys"
      ],
      "mask": "mask",
      "script": "script",
      "type": type
    }
  ],
  "snapshot": "snapshot",
  "version": version
}
```

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

Get the transaction in cache by hash.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

See also [signrawtransaction](#signrawtransaction).

*Example*

``` bash
mixin -n 127.0.0.1:8239 getcachetransaction --hash HASH
```

*See also*

* [Mixin Kernel Transactions](https://github.com/MixinNetwork/mixin/blob/master/doc/mixin-kernel-transactions.md)

#### getutxo

Get the UTXO by hash and index.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| hash    | string  | Required  | the transaction hash                    |
| index   | integer | Required, Default=0 | the output index              |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
  "amount": "amount",
  "hash": "hash",
  "index": index,
  "keys": [
    "keys"
  ],
  "mask": "mask",
  "script": "script",
  "type": type
}
```

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

List mint distributions.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| since   | integer | Required, Default=0  | the mint batch to begin with |
| count   | integer | Required, Default=10 | the up limit of the returned distributions |
| tx      | boolean | Optional, Default=false | whether including the transactions |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
[
  {
    "amount": "amount",
    "batch": batch,
    "group": "group",
    "transaction": {
      "asset": "asset",
      "extra": "extra",
      "hash": "hash",
      "inputs": [
        {
          "mint": {
            "amount": "amount",
            "batch": batch,
            "group": "group"
          }
        }
      ],
      "outputs": [
        {
          "amount": "amount",
          "keys": [
            "keys"
          ],
          "mask": "mask",
          "script": "script",
          "type": type
        }
      ],
      "version": version
    }
  }
]
```

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
        }
        ...
      ],
      "version": 1
    }
  }
]
```

#### listallnodes

List all nodes ever existed.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
[
  {
    "id": "id", (string) node id
    "payee": "payee", (string) payee address of node
    "signer": "signer", (string) signer address of node
    "state": "state", (string) node state
    "timestamp": timestamp, (timestamp) node timestamp
    "transaction": "transaction" (string) transaction hash
  }
]
```

*Example*

``` bash
mixin -n 127.0.0.1:8239 listallnodes
[
  {
    "id": "f3fcf842446bcf00f3787fd809a02fb4528c57121481904c41d8c025c861a477",
    "payee": "XINYDpVHXHxkFRPbP9LZak5p7FZs3mWTeKvrAzo4g9uziTW99t7LrU7me66Xhm6oXGTbYczQLvznk3hxgNSfNBaZveAmEeRM",
    "signer": "XINJ7LcWaCqPt9zrQFjPz2kQEy4BywpUBrBFQvTLD22siC6VH1MWEk72ftR1HbeSYrTn1VvX1HkR4EyG262JewpHbyDj83kS",
    "state": "ACCEPTED",
    "timestamp": 1558283107344677000,
    "transaction": "2e1f3558ebf4f5d4de110edeae316bcff40f7cf487a3deaefa35c125109b182e"
  }
]
```

#### getinfo

Get info from the node.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
{
  "epoch": "epoch",
  "graph": {
    "cache": {
      "id": {
        "node": "node",
        "references": {
          "external": "external",
          "self": "self"
        },
        "round": round,
        "snapshots": [
          {
            "hash": "hash",
            "node": "node",
            "references": {
              "external": "external",
              "self": "self"
            },
            "round": round,
            "signature": "signature",
            "timestamp": timestamp,
            "transaction": "transaction",
            "version": version
          }
        ],
        "timestamp": timestamp
      }
    },
    "consensus": [
      {
        "node": "node",
        "payee": "payee",
        "signer": "signer",
        "state": "state",
        "timestamp": timestamp,
        "transaction": "transaction"
      }
    ],
    "final": {
      "id": {
        "end": end,
        "hash": "hash",
        "node": "node",
        "round": round,
        "start": start
      }
    },
    "topology": topology
  },
  "mint": {
    "batch": batch,
    "pool": "pool"
  },
  "network": "network",
  "node": "node",
  "queue": {
    "caches": cache,
    "finals": finals
  },
  "timestamp": "timestamp",
  "uptime": "uptime",
  "version": "version"
}
```

*Example*

``` bash
mixin -n 127.0.0.1:8239 getinfo
{
  "epoch": "2019-02-28T00:00:00Z",
  "graph": {
    "cache": {
      "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3": {
        "node": "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3",
        "references": {
          "external": "9a2692555cfb310c589c8c92a99caddad3e14bc19736cf1d08ee12c960b9ecc4",
          "self": "0ffe0a13d8297176af2aef7e8f227d0583162e1e4b92680cfdcd012e9358169e"
        },
        "round": 13472,
        "snapshots": [
          {
            "hash": "a9c757b6b7125e8ae5280d9e26be6a3908e8fcd51f4b7ec3c1f36f717a0d514a",
            "node": "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3",
            "references": {
              "external": "9a2692555cfb310c589c8c92a99caddad3e14bc19736cf1d08ee12c960b9ecc4",
              "self": "0ffe0a13d8297176af2aef7e8f227d0583162e1e4b92680cfdcd012e9358169e"
            },
            "round": 13472,
            "signature": "f1168ee14873710e6f57b6eb7b3d7f415816f2d9c2bab135c201c51b132708372138204ec97c3a42fab5568015c56ca8345f3d7b7cdf12d526457e55b049ca000000000001b977c7",
            "timestamp": 1585062893115572000,
            "transaction": "4471d5c14d0d2a354f0fdce45114f5b2401aea428a7ad6da3c4fdf8bfbf7c6e7",
            "version": 1
          }
        ],
        "timestamp": 1585062886145882000
      }
    },
    "consensus": [
      {
        "node": "cbba7a5e7bae3b0cef3d6dcba7948fa03facda3be401d67aa1a38aecb1f443a0",
        "payee": "XINCcpcWJbJRiqEoUV7pWrmAdN1AZq3wyYTxa62JojvM4UqpuQnoVX7DZ6BgJEb61pSUS4ZyZNuEbAGL5azNyZNCbwdgqcVY",
        "signer": "XIN3ntCzd1FqjSxrYM1f9abN3wY5DcydkDviEVgZL3paV7oYEeKnwzbMLwoRVANwyiu7w9mRrPf2eTpPaLRgQow9rSr3hzWH",
        "state": "ACCEPTED",
        "timestamp": 1579450099118731000,
        "transaction": "ebbbf69e9e74e4070ef0685f8d9b4d7bc443922ac93445bc9bda1567984bdda8"
      }
    ],
    "final": {
      "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3": {
        "end": 1585062883145882000,
        "hash": "0ffe0a13d8297176af2aef7e8f227d0583162e1e4b92680cfdcd012e9358169e",
        "node": "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3",
        "round": 13471,
        "start": 1585062883145882000
      }
    },
    "topology": 15395311
  },
  "mint": {
    "batch": 390,
    "pool": "452226.02739800"
  },
  "network": "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997",
  "node": "9986a9bc3f44d45647d5db245eb562793c83930866b0757bdbb73f17e692d9c7",
  "queue": {
    "caches": 0,
    "finals": 0
  },
  "timestamp": "2020-03-24T15:15:21.092871504Z",
  "uptime": "30h9m43.831051032s",
  "version": "v0.7.27-a68e4d2049d2ea9f95353b56bfcb60e307fdaebd"
}
```

#### dumpgraphhead

Dump the graph head.

*Parameter*

| Name    | Type    | Presence  | Description                             |
| :-----: |:-------:| :-----    | :------------------------------------   |
| help    | boolean | Optional, Default=false  | show help                |

*Result*

``` bash
[
  {
    "hash": "hash", (string) transaction hash
    "node": "node", (string) node id
    "round": round (integer) round number
  }
]
```

*Example*

``` bash
mixin -n 127.0.0.1:8239 dumpgraphhead
[
  {
    "hash": "ead887df0ae2e2221dd5841efb16ac1d0b5bbdf797abd29894619b410c111dd5",
    "node": "017ebfb57ed9aace3d2ed9d559b7a6bf16a8745113872f80cf74ed618a40d3d3",
    "round": 13479
  }
]
```