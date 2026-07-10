# Mixin Kernel Remote Procedure Calls

Mixin Kernel exposes ledger queries and transaction submission through a small HTTP/JSON interface. The same `mixin` binary includes CLI wrappers for every RPC method, plus local address, transaction-construction, decoding, and maintenance tools.

This interface is JSON-based but is not JSON-RPC 2.0: requests do not contain a `jsonrpc` field, and protocol errors are returned in a project-specific envelope.

## Endpoint and configuration

Enable RPC in the node's `config.toml`:

```toml
[rpc]
port = 6860
runtime = false
object-server = false
```

The server listens on the configured TCP port. The CLI defaults to `http://127.0.0.1:6860`; override it with the global `--node` option or `MIXIN_KERNEL_RPC`:

```bash
./mixin --node http://127.0.0.1:6860 getinfo

export MIXIN_KERNEL_RPC=http://127.0.0.1:6860
./mixin getinfo
```

The built-in server does not provide TLS or an authentication layer. It also permits cross-origin browser requests. Use host firewall rules or a trusted reverse proxy when the service is reachable beyond the local machine. The Go profiling endpoint configured under `[dev]` is a separate service and should also remain private.

## Request and response envelopes

Send a JSON object to `POST /` with a method name and positional parameter array. `id` is an optional string copied into the response.

```json
{
  "id": "request-1",
  "method": "getinfo",
  "params": []
}
```

Example:

```bash
curl -sS http://127.0.0.1:6860 \
  -H 'Content-Type: application/json' \
  --data '{"id":"request-1","method":"getinfo","params":[]}'
```

A successful call returns:

```json
{
  "id": "request-1",
  "data": {}
}
```

A rejected call returns:

```json
{
  "id": "request-1",
  "error": "invalid params count"
}
```

When `rpc.runtime = true`, either envelope also contains `runtime`, represented as elapsed seconds in a string. Application-level errors normally still use HTTP status `200`, so clients must inspect `error`. A missing well-formed object may appear as `data: null`, while malformed identifiers and invalid parameters return `error`.

`GET /` is a convenience endpoint that returns the same `data` object as `getinfo`, without a request ID.

## Method reference

Parameters are positional and must appear in the listed order. Hashes and keys are lowercase or uppercase hexadecimal strings accepted by the corresponding decoder; amounts in results are fixed-precision decimal strings. Timestamps used by ledger objects are Unix nanoseconds unless stated otherwise.

### Node and graph

| Method | `params` | Result |
| --- | --- | --- |
| `getinfo` | `[]` | Node, consensus, graph, queue, mint, and transport summary |
| `listpeers` | `[]` | Direct peer objects `{id, address, relayer}` |
| `listrelayers` | `[node_id]` | Known relay peers for the requested node |
| `dumpgraphhead` | `[]` | One sync point per tracked node chain |
| `listallnodes` | `[timestamp_ns, include_state_history]` | Membership records at or before the timestamp |

`timestamp_ns = 0` makes `listallnodes` use the server's current time. With `include_state_history = false`, the result contains the latest state per signer; `true` returns the complete state sequence. Membership states are `PLEDGING`, `ACCEPTED`, `CANCELLED`, and `REMOVED`.

For privacy of network topology, `listpeers` and `listrelayers` return populated results only when the HTTP connection's remote address is IPv4 loopback (`127.0.0.1`). Other callers receive an empty array.

### Transactions, UTXOs, and assets

| Method | `params` | Result |
| --- | --- | --- |
| `sendrawtransaction` | `[signed_transaction_hex]` | `{hash}` after the node accepts the transaction into its processing path |
| `gettransaction` | `[transaction_hash]` | Durable transaction object with `hex` and, when final, `snapshot` |
| `getcachetransaction` | `[transaction_hash]` | Unfinalized cache transaction object with `hex` |
| `getdeposittransaction` | `[chain_id, external_transaction_id, output_index]` | Transaction associated with an external deposit tuple |
| `getwithdrawalclaim` | `[withdrawal_submit_hash]` | Claim transaction associated with a withdrawal submit transaction |
| `getutxo` | `[transaction_hash, output_index]` | Current UTXO and its optional candidate lock |
| `getkey` | `[ghost_public_key]` | Transaction currently reserving or owning the ghost key |
| `getasset` | `[asset_id]` | Asset mapping and ledger-wide balance |

`sendrawtransaction` returning a hash is not a separate finality receipt. Confirm finality by waiting for `gettransaction` to include a `snapshot` value, then retrieve that snapshot and verify its collective signature as appropriate for the client.

### Snapshots and rounds

| Method | `params` | Result |
| --- | --- | --- |
| `getsnapshot` | `[snapshot_hash]` | Snapshot with collective signature and expanded transactions |
| `listsnapshots` | `[topology_offset, count, include_signature, include_transactions]` | Snapshots from the inclusive local topology cursor |
| `getroundbynumber` | `[node_id, round_number]` | One round and all of its snapshots |
| `getroundbyhash` | `[round_hash]` | One round and all of its snapshots |
| `getroundlink` | `[from_node_id, to_node_id]` | `{link}` containing the latest stored link position |

Topological order is local to the queried node; it is a pagination cursor, not a globally agreed block height. When `include_transactions` is false, snapshots contain transaction hashes. When true, each hash is replaced by its normalized transaction object, and `count` may not exceed 500. `include_signature` controls the snapshot `signature` field in `listsnapshots`; `getsnapshot` always includes it.

### Mint and custodian state

| Method | `params` | Result |
| --- | --- | --- |
| `listmintworks` | `[batch]` | Map from accepted node ID to `[led_snapshots, signed_snapshots]` for the mint day |
| `listmintdistributions` | `[batch_offset, count, include_transactions]` | Mint distribution objects starting at the batch offset |
| `listcustodianupdates` | `[]` | Complete custodian update history |

`listmintdistributions` permits at most 500 results. Its `transaction` field is a hash unless `include_transactions` is true, in which case it contains the normalized transaction object.

## Result objects

### Node information

`getinfo` returns a live, node-local summary:

```json
{
  "network": "<network identifier>",
  "node": "<local node identifier>",
  "version": "<build version>",
  "uptime": "12h34m56s",
  "epoch": "<RFC 3339 time>",
  "timestamp": "<RFC 3339 graph time>",
  "consensus": "<latest consensus snapshot hash>",
  "mint": {
    "pool": "<remaining mint pool>",
    "batch": 0,
    "pledge": "13439.00000000"
  },
  "graph": {
    "consensus": [],
    "cache": {},
    "final": {},
    "topology": 0,
    "sps": 0,
    "tps": 0
  },
  "queue": {
    "finals": 0,
    "caches": 0,
    "state": "<queue state>"
  },
  "metric": {
    "transport": {}
  }
}
```

`graph.consensus` contains the currently relevant accepted or pledging nodes and their signer, payee, state, work, and aggregation progress. `graph.cache` and `graph.final` are keyed by node ID. `sps` and `tps` are local rolling snapshot and distinct-transaction rates, not network-wide guarantees.

`dumpgraphhead` returns sync points in this form:

```json
{
  "node": "<node identifier>",
  "round": 12345,
  "hash": "<final round hash>",
  "pool": {
    "index": 0,
    "count": 0
  }
}
```

### Transaction

The normalized transaction object is:

```json
{
  "version": 5,
  "asset": "<asset identifier>",
  "inputs": [
    {
      "hash": "<source transaction hash>",
      "index": 0
    }
  ],
  "outputs": [
    {
      "type": 0,
      "amount": "1.00000000",
      "keys": ["<ghost public key>"],
      "script": "fffe01",
      "mask": "<output mask>"
    }
  ],
  "references": [],
  "extra": "",
  "hash": "<transaction hash>",
  "hex": "<signed transaction encoding>",
  "snapshot": "<finalizing snapshot hash>"
}
```

`hex` is added by transaction lookup methods but is absent from transactions expanded inside a snapshot or mint distribution. `snapshot` is present only when a lookup resolves a finalization record.

An input has exactly one of these forms:

```json
{"hash":"<transaction hash>","index":0}
```

```json
{"genesis":"<encoded genesis input>"}
```

```json
{
  "deposit": {
    "chain": "<chain identifier>",
    "asset_key": "<external asset key>",
    "transaction": "<external transaction identifier>",
    "index": 0,
    "amount": "1.00000000"
  }
}
```

```json
{
  "mint": {
    "group": "UNIVERSAL",
    "batch": 123,
    "amount": "1.00000000"
  }
}
```

Output `keys`, `script`, and `mask` are present when applicable. Withdrawal outputs additionally contain:

```json
{
  "withdrawal": {
    "address": "<external destination>",
    "tag": "<optional destination tag>"
  }
}
```

See [Mixin Kernel Transactions](./mixin-kernel-transactions.md) for field semantics, limits, and authorization.

### Snapshot

```json
{
  "version": 2,
  "node": "<proposing node identifier>",
  "references": {
    "self": "<self round hash>",
    "external": "<external round hash>"
  },
  "round": 12345,
  "timestamp": 1760000000000000000,
  "transactions": ["<transaction hash>"],
  "hash": "<snapshot hash>",
  "hex": "<encoded signed snapshot with topology>",
  "topology": 987654,
  "signature": "<collective signature and signer mask>",
  "witness": {
    "signature": "<serving-node witness signature>",
    "timestamp": 1760000001000000000
  }
}
```

`transactions` is an array of hashes or expanded transaction objects, depending on the method and expansion flag. `signature` establishes consensus finality. `witness` is a fresh signature by the serving node over the exact stored encoding, including its local topology metadata; it is not the consensus certificate.

See [Mixin Kernel Snapshots](./mixin-kernel-snapshots.md) for batching, rounds, references, and topology semantics.

### Round

```json
{
  "node": "<node identifier>",
  "hash": "<round hash>",
  "start": 1760000000000000000,
  "end": 1760000002000000000,
  "number": 12345,
  "references": {
    "self": "<preceding self round hash>",
    "external": "<external round hash>"
  },
  "snapshots": []
}
```

Round snapshots contain transaction hashes and omit the collective `signature` field in these responses. The hash commits the canonical snapshot sequence; see the snapshot guide for the exact construction.

### UTXO, key, and asset

`getutxo` returns the source and spending condition, with optional fields omitted when empty:

```json
{
  "type": 0,
  "hash": "<transaction hash>",
  "index": 0,
  "amount": "1.00000000",
  "keys": ["<ghost public key>"],
  "script": "fffe01",
  "mask": "<output mask>",
  "lock": "<candidate transaction hash>"
}
```

`lock` means a candidate transaction currently reserves the output; it is omitted when there is no lock.

`getkey` always returns an object. `transaction` is `null` when the key is not reserved or recorded, and otherwise is a transaction-hash string:

```json
{"transaction":null}
```

`getasset` returns:

```json
{
  "id": "<asset identifier>",
  "chain": "<external chain identifier>",
  "asset_key": "<external asset key>",
  "balance": "<ledger-wide amount>"
}
```

### Membership, peers, mint, and custodian history

A `listallnodes` item is:

```json
{
  "id": "<network-scoped node identifier>",
  "signer": "<XIN signer address>",
  "payee": "<XIN payee address>",
  "transaction": "<transaction establishing this state>",
  "timestamp": 1760000000000000000,
  "state": "ACCEPTED"
}
```

A peer item is:

```json
{
  "id": "<node identifier>",
  "address": "<host:port>",
  "relayer": false
}
```

A mint distribution item is:

```json
{
  "group": "UNIVERSAL",
  "batch": 123,
  "amount": "1.00000000",
  "transaction": "<hash or expanded transaction>"
}
```

A custodian history item is:

```json
{
  "custodian": "<XIN custodian address>",
  "transaction": "<custodian update transaction hash>",
  "timestamp": 1760000000000000000
}
```

## CLI mappings

The CLI unwraps the server's `data` field and prints it as JSON. Global options must precede the command:

```bash
./mixin --node http://127.0.0.1:6860 getinfo
```

| RPC method | CLI command and flags |
| --- | --- |
| `sendrawtransaction` | `sendrawtransaction --raw HEX` |
| `gettransaction` | `gettransaction --hash HASH` |
| `getcachetransaction` | `getcachetransaction --hash HASH` |
| `getdeposittransaction` | `getdeposittransaction --chain HASH --hash EXTERNAL_ID --index N` |
| `getwithdrawalclaim` | `getwithdrawalclaim --hash SUBMIT_HASH` |
| `getutxo` | `getutxo --hash HASH --index N` |
| `getkey` | `getkey --key GHOST_KEY` |
| `getasset` | `getasset --id ASSET_ID` |
| `getsnapshot` | `getsnapshot --hash HASH` |
| `listsnapshots` | `listsnapshots --since TOPOLOGY --count N [--sig] [--tx]` |
| `getroundbynumber` | `getroundbynumber --id NODE_ID --number N` |
| `getroundbyhash` | `getroundbyhash --hash HASH` |
| `getroundlink` | `getroundlink --from NODE_ID --to NODE_ID` |
| `listmintworks` | `listmintworks --since BATCH` |
| `listmintdistributions` | `listmintdistributions --since BATCH --count N [--tx]` |
| `listallnodes` | `listallnodes --threshold TIMESTAMP_NS [--state]` |
| `listcustodianupdates` | `listcustodianupdates` |
| `listpeers` | `listpeers` |
| `listrelayers` | `listrelayers --id NODE_ID` |
| `dumpgraphhead` | `dumpgraphhead` |

`createaddress`, `decodeaddress`, `decoderawtransaction`, and `decodesignature` are local utilities, not RPC methods. Transaction builders and signers also run locally, although they may query RPC for source UTXO data. See the [README](../README.md) and [transaction guide](./mixin-kernel-transactions.md) for those workflows.

## Object server

When `rpc.object-server = true`, an XIN transaction present in the node's durable transaction store exposes its `extra` bytes through a content-oriented HTTP endpoint:

```text
GET /objects/<transaction-hash>
```

The response body is the raw `extra` value. UTF-8 data is served as text, JSON objects as JSON, and other data as `application/octet-stream`.

If `extra` is a top-level JSON object, one field can be retrieved with:

```text
GET /objects/<transaction-hash>/<field>
```

If that field is a `data:` URI, the server decodes base64 when requested and uses its media type and charset. Object responses include one-year public caching, a sandbox content-security policy, and `X-Content-Type-Options: nosniff`.

The object endpoint serves only transactions whose asset is XIN, but it does not itself require a finalization record. A client that needs permanence must confirm that `gettransaction` returns a `snapshot` field. Invalid, missing, or non-XIN objects return the normal JSON `error` envelope. Storage transaction construction is documented in [STORAGE.md](../STORAGE.md).
