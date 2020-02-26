# Mixin Kernel Snapshots

Snapshot is the core of all Mixin Kernel stuffs, the Mixin Kernel BFT consensus operates on snapshots.

The current Kernel snapshot version is `0x01`, which is marshaled as a special msgpack format [https://github.com/MixinNetwork/msgpack](https://github.com/MixinNetwork/msgpack) before the snapshot hash calculation.

All snapshot related API exposed externally have and should always have a consistent JSON representation as below.

```json
{
  "hash": "41cc8afecdc4b83f647eb2f27465f5342587742a9de337182f638e8faf81b811",
  "node": "b1ff822e0fc8e1510c0f5eeeb18d3cdc7513bc2142bc936efb2649f2178a6b0c",
  "references": {
    "external": "44494e87df6b2591ab98be05e672d4dc287caa6806879371edbaf26204df1ad2",
    "self": "ad84328addc25c009b3e88d06465a8867a70f49abf82b8352e90e0c3cafe6c6c"
  },
  "round": 367849,
  "timestamp": 1566691718496937854,
  "topology": 10000000,
  "transaction": "f9cb8e3c60d5c1911d7c502396f33011423c1263d632e6708786b52718c7963d",
  "version": 1
}
```

- **hash**: HEX representation of a 32 bytes hash, which is the unique snapshot identifier.

- **node**: HEX representation of a 32 bytes hash, which is the node id which leads this snapshot.

- **references**: the previous round hashes of the leading node and another node conforms to the consensus.

- **round**: a uint64 round number of this snapshot, round is similar to the block of Bitcoin.

- **timestamp**: a uint64 nanosecond since Unix epoch, which is provided by the leading node and agreed upon consensus.

- **topology**: a uint64 number as the snapshot order in all snapshots, this value is the only node provided value, **NOT** included in the hash and not agreed by consensus.

- **transaction**: HEX representation of a 32 bytes hash, which is the transaction hash included by this snapshot.

- **version**: a uint8 number to hint the current snapshot format.
