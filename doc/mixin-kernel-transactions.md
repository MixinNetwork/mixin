# Mixin Kernel Transactions

The current Kernel transaction version is `0x01`, which is marshaled as a special msgpack format [https://github.com/MixinNetwork/msgpack](https://github.com/MixinNetwork/msgpack) before the transaction hash calculation.

All transaction related API exposed externally have and should always have a consistent JSON representation of inputs and outputs, a neat UTXO model.

```json
{
  "asset": "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
  "extra": "f7619c5618a05c0f86bcfbcea0ec69c1",
  "hash": "f9cb8e3c60d5c1911d7c502396f33011423c1263d632e6708786b52718c7963d",
  "inputs": [
    {
      "hash": "1a9d515d32adcbb13e34f3d5fd597f8d6750a1e80e32e2955aa50dcd907a1e44",
      "index": 1
    }
  ],
  "outputs": [
    {
      "amount": "0.00169230",
      "keys": [
        "1685d250a1b1ecb10a2debdae4237f002c3169a4578266614b4dd8c2151399ec"
      ],
      "mask": "2f5b06608bb9829d381a89dc5fc9890699e5beaf32ca48d604ff3149cc887602",
      "script": "fffe01",
      "type": 0
    },
    {
      "amount": "0.06599083",
      "keys": [
        "c259678709da0b3ac3a5915554fd4d34752befb84431acc94813f4d105bfb09c"
      ],
      "mask": "8b09414eda7bd8b5b71d371ff3cb77c5016d9af510c2c17e15b56b06a8a82579",
      "script": "fffe01",
      "type": 0
    }
  ],
  "version": 1
}
```

- **asset**: HEX representation of a 32 bytes hash, which is a unique asset identifier, e.g. BTC or XIN.

- **extra**: HEX representation of at most 256 bytes data.

- **hash**: HEX representation of a 32 bytes hash, which is the unique transaction identifier.

- **inputs**: an array of input objects, which may be the outputs of previous transactions, Kernel mint reward, Domain deposit or Genesis.

- **outputs**: an array of output objects, which can be used as the inputs of future transactions.

- **version**: a uint8 number to hint the current transaction format.

The genesis input is only used when the network boot from genesis.json, only those genesis Kernel Nodes accept transactions have this kind of inputs.

```json
{
  "genesis":"6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
}
```

The domain deposit comes when some assets flow into the Kernel from Domain, e.g. when someone deposit ETH from outside of Mixin Kernel.

```json
{
  "deposit": {
    "amount": "2.15226159",
    "asset": "0x0000000000000000000000000000000000000000",
    "chain": "8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27",
    "index": 0,
    "transaction": "0x07f073bd2c056be7833c270215c162bff6774673a318d0e842dc27aac686a3ec"
  }
}
```

The mint reward input usually appears daily, it shows the current mint batch and total reward amount for all Kernel nodes.

```json
{
  "mint": {
    "amount": "123.28767117",
    "batch": 200,
    "group": "KERNELNODE"
  }
}
```

The general input source is the outputs of earlier transactions, the standard UTXO model with the earlier transaction hash and output index.

```json
{
  "hash": "1a9d515d32adcbb13e34f3d5fd597f8d6750a1e80e32e2955aa50dcd907a1e44",
  "index": 1
}
```

The output target is always masked and can be spent according to the type and script constraints.

- **amount**: a string decimal, always rounded to 8 decimal places.

- **keys**: array of HEX representation of 32 bytes key, which are the owner of this output and called ghost keys.

- **mask**: HEX representation of 32 bytes key, which is used to parse the ghost keys.

- **script**: HEX representation of `{0xff, 0xfe, T}`, while `0 <= T <= 0x40`, where T is the required number of signatures from keys to spend this output.

- **type**: a uint8 number to constraint when and how this output can be spent as an input, usually 0 which means it can be spent once the script fulfilled.
