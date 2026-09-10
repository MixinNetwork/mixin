# Mixin Kernel Remote Procedure Calls

Mixin Kernel exposes ledger queries and transaction submission through a small HTTP/JSON interface. The same `mixin` binary includes CLI wrappers for every RPC method, plus local address, transaction-construction, decoding, and maintenance tools.

This interface uses a project-specific request and error envelope rather than JSON-RPC 2.0. It does not require or interpret a `jsonrpc` version field.

## Endpoint and configuration

Enable RPC in the node's `config.toml`:

```toml
[rpc]
port = 6860
runtime = false
object-server = false
```

The daemon starts RPC when `port` is greater than zero and listens on `:<port>` on all available interfaces. The CLI defaults to `http://127.0.0.1:6860`; override it with the global `--node` option or `MIXIN_KERNEL_RPC`:

```bash
./mixin --node http://127.0.0.1:6860 getinfo

export MIXIN_KERNEL_RPC=http://127.0.0.1:6860
./mixin getinfo
```

The built-in server does not provide TLS or RPC authentication. Its CORS handler reflects the request's `Origin` and permits cross-origin requests; the advertised methods do not create additional RPC routes. An `OPTIONS` request with an `Origin` receives HTTP `200` with `{}`. The Go profiling endpoint configured under `[dev]` is a separate service.

## Request and response envelopes

Send one JSON object to `POST /` with a method name and positional parameter array. Method names are case-sensitive. JSON-RPC batch arrays are not supported, and trailing JSON values or non-whitespace data are rejected. The request-body limit is 8 MiB + 64 KiB (8,454,144 bytes). The transaction decoder separately limits the complete signed binary envelope to 4 MiB, and common validation also caps the unsigned payload at 4 MiB. Its hexadecimal representation and JSON framing must fit the HTTP body limit. Other transaction limits still apply.

`id` is an optional string. A nonempty value is echoed after the complete request is decoded; an absent, empty, or `null` ID is omitted from the response. Numeric IDs fail decoding. Unknown object fields are ignored.

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

The envelope shapes below are illustrative. Successful calls return `data` with the method's result:

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

When `rpc.runtime = true`, responses to fully decoded POST calls also contain `runtime`, represented as elapsed dispatch time in seconds in a string. Request-decoding errors and the recovered-panic response omit both `id` and `runtime`; the panic response is `{"error":"server error"}`. `error` is a string, without a numeric code or structured error object. The handler returns HTTP `200` for both result and error envelopes, including bad paths and invalid methods. Clients must inspect `error` as well as transport status.

`GET /` returns the `getinfo` envelope without `id` or `runtime`. Apart from enabled object routes and CORS preflights, other path/method combinations return a `bad request` error. Server timeouts are 5 seconds for request headers, 10 seconds for request reads and response writes, and 120 seconds for idle connections.

## Method reference

Parameters are positional and must appear in the listed order. The tables use these types:

| Type | Representation and bounds |
| --- | --- |
| Hash or key | A 32-byte value encoded as 64 hexadecimal characters, without `0x`; either letter case is accepted and output is lowercase |
| `uint64` | An unsigned base-10 integer from 0 through 18,446,744,073,709,551,615, supplied as a JSON number or decimal string |
| Output index for `getutxo` | An unsigned integer from 0 through 65,535; this parsing limit does not mean that every index exists |
| Boolean | Prefer JSON `true` or `false`; the parser also accepts strings `true`, `True`, `TRUE`, `t`, `T`, `1` and the corresponding `false`, `False`, `FALSE`, `f`, `F`, `0`, or JSON numbers `1` and `0` |
| External transaction ID | An external-chain identifier string; the RPC lookup does not parse it as a Kernel hash |
| Amount in a result | A decimal string with eight fractional digits |

Integer parameters do not accept fractional or exponent notation. The server preserves JSON integer text during decoding; clients must also preserve precision for nanosecond timestamps and other large integers. Ledger timestamps are Unix nanoseconds unless stated otherwise. `getinfo.epoch` and `getinfo.timestamp` are RFC 3339 time strings.

Methods require the parameter count shown, except `getinfo`, `listpeers`, `dumpgraphhead`, and `listcustodianupdates`, which ignore their parameters. Use `[]` for those methods. Wrong counts return `invalid params count`; parsing and validation errors are strings rather than a stable error-code taxonomy.

### Node and graph

| Method | `params` | Result |
| --- | --- | --- |
| `getinfo` | `[]` | Node, consensus, graph, queue, mint, and transport summary |
| `listpeers` | `[]` | Direct peer objects `{id, address, relayer}` |
| `listrelayers` | `[node_id]` | Known relay peers for the requested node |
| `dumpgraphhead` | `[]` | One sync point per tracked node chain |
| `listallnodes` | `[timestamp_ns, include_state_history]` | Membership records at or before the timestamp |

`node_id` is a hash. `timestamp_ns` is a `uint64`, and `include_state_history` is a boolean. `timestamp_ns = 0` makes `listallnodes` use the server's current time. With `include_state_history = false`, the result contains the latest state per signer through that time; `true` retains all state records through that time. Membership states are `PLEDGING`, `ACCEPTED`, and `REMOVED`.

`listpeers` and `listrelayers` return populated results only when the HTTP connection's remote address begins with `127.0.0.1:`. IPv6 loopback and other addresses receive `[]`; forwarded-address headers do not control this check. A local reverse proxy therefore counts as a local caller. `listrelayers` checks the parameter count but ignores a hash-decoding error, using the resulting zero hash for that lookup.

### Transactions, UTXOs, and assets

| Method | `params` | Result |
| --- | --- | --- |
| `sendrawtransaction` | `[signed_transaction_hex]` | `{hash}` for a queued, already cached, or already finalized payload |
| `gettransaction` | `[transaction_hash]` | Durable transaction object with `hex` and, when final, `snapshot` |
| `getcachetransaction` | `[transaction_hash]` | Cached transaction object with `hex`, without a finalization lookup |
| `getdeposittransaction` | `[chain_id, external_transaction_id, output_index]` | Durable transaction found through the deposit tuple's lock record |
| `getwithdrawalclaim` | `[withdrawal_submit_hash]` | Claim transaction associated with a withdrawal submit transaction |
| `getutxo` | `[transaction_hash, output_index]` | Stored output and its optional consuming-transaction lock |
| `getkey` | `[ghost_public_key]` | Transaction hash recorded for the ghost key |
| `getasset` | `[asset_id]` | Asset mapping and ledger-wide balance |

Hash/key parameters have the form described above. `signed_transaction_hex` contains the full binary transaction envelope as hexadecimal without `0x`. The deposit `output_index` is a `uint64`; `getutxo` uses the narrower 16-bit index limit.

For an unfinalized payload, submission accepts script transfers, deposits, withdrawal submissions, withdrawal claims, and node pledges. Custodian updates are also accepted outside the configured mainnet network. Other types are produced through their protocol paths. An already finalized payload returns its hash before this type filter.

For a payload absent from the cache, `sendrawtransaction` runs common validation and then caches and schedules the envelope. For an already cached payload it reuses the first retained envelope and its prior cache entry without fresh common validation of the submitted authorization. Transaction hashes exclude authorization signatures, so the returned hash does not identify which signature envelope was retained. Common validation can reserve ghost keys, and later scheduling or persistence failure does not provide a general rollback of those reservations.

`sendrawtransaction` success is neither a finality receipt nor a guarantee of eventual inclusion. `getcachetransaction` reads the TTL-backed cache without revalidating authorization or checking finality. `gettransaction` reads durable storage, which includes candidates persisted before finalization as well as finalized transactions. A nonempty `snapshot` field reports the node's first local finalization record; its absence means that this lookup has no such record. Cache presence, durable presence, and finality are separate observations.

`gettransaction`, `getcachetransaction`, `getdeposittransaction`, `getwithdrawalclaim`, `getutxo`, `getasset`, and `getsnapshot` return `data: null` when their requested record is absent. A deposit lock can exist without a retrievable body, so a null deposit result alone does not establish that the tuple is free. A withdrawal claim lookup reports a ledger claim record; it does not independently confirm payment on the external chain.

For independent finality checking, retrieve the reported snapshot, recompute its payload hash, check transaction inclusion, and verify its certificate against the applicable historical signer set and graph rules. The RPC helpers decode responses; they do not perform that verification for the caller.

### Snapshots and rounds

| Method | `params` | Result |
| --- | --- | --- |
| `getsnapshot` | `[snapshot_hash]` | Snapshot with collective signature and expanded transactions |
| `listsnapshots` | `[topology_offset, count, include_signature, include_transactions]` | Snapshots from the inclusive local topology cursor |
| `getroundbynumber` | `[node_id, round_number]` | One round and all of its snapshots |
| `getroundbyhash` | `[round_hash]` | One round and all of its snapshots |
| `getroundlink` | `[from_node_id, to_node_id]` | `{link}` containing the latest stored link position |

All hashes are 32-byte hex values; `topology_offset`, `count`, and `round_number` are `uint64` values, and both inclusion flags are booleans. `listsnapshots` permits `count` from 0 through 500 with either expansion setting; zero returns `[]`. Results are ordered by ascending local topology from the inclusive offset. Continue with the last returned `topology + 1` to avoid repeating that entry.

Topological order is local to the queried node; it is a pagination cursor, not a globally agreed block height. When `include_transactions` is false, snapshots contain transaction hashes; when true, each hash is replaced by its normalized transaction object. `include_signature` controls only the JSON `signature` field in `listsnapshots`; the full encoded snapshot remains in `hex`. `getsnapshot` includes both expanded transactions and the signature field, which can be `null` for genesis.

`getroundbyhash` also accepts a node ID as the key for that node's active head round. `getroundbynumber` returns the same head representation when the requested number is current. Missing rounds return errors rather than `data: null`; an unknown node in `getroundbynumber` can produce the generic `server error`. `getroundlink` returns `{"link":0}` when there is no stored link, so zero alone does not distinguish absence from a round-zero position.

### Mint and custodian state

| Method | `params` | Result |
| --- | --- | --- |
| `listmintworks` | `[batch]` | Map from accepted node ID to `[led_snapshots, signed_snapshots]` for the mint day |
| `listmintdistributions` | `[batch_offset, count, include_transactions]` | Mint distribution objects starting at the batch offset |
| `listcustodianupdates` | `[]` | Complete custodian update history |

`batch`, `batch_offset`, and `count` are `uint64` values; `include_transactions` is a boolean. A mint batch is a day offset from the network epoch. `listmintworks` selects accepted nodes at `epoch + batch × 24 hours` and returns their recorded work for that day; it does not return mint payout amounts.

`listmintdistributions` permits `count` from 0 through 500 and includes finalized distributions only, in ascending batch order from the inclusive offset. Zero returns `[]`; continue with the last returned `batch + 1`. Its `transaction` field is a hash unless expansion is enabled, in which case it contains the normalized transaction object. Custodian history items expose the aggregate custodian address, transaction, and timestamp, not the complete participant configuration.

## Result objects

The JSON examples in this section illustrate field names and types, not observed node responses or valid cryptographic fixtures. Strings in angle brackets are placeholders. Optional fields and null values are described alongside each shape.

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
    "spt": 0,
    "tps": 0
  },
  "queue": {
    "finals": 0,
    "caches": 0,
    "state": {}
  },
  "metric": {
    "transport": {}
  }
}
```

`timestamp` is the node's graph-progress time, not the response time. `consensus` identifies the latest consensus-history snapshot selected by the node. The summary combines live memory and storage reads; it is not one atomic ledger view.

| Field | Contents |
| --- | --- |
| `graph.consensus[]` | `{node, signer, payee, state, timestamp, transaction, aggregator, works}` for accepted/pledging nodes at graph time; optional `spaces` is `[batch, round]` |
| `graph.consensus[].aggregator` | Work aggregation round offset as an integer |
| `graph.consensus[].works` | `[led_snapshots, signed_snapshots]` for the graph timestamp's day |
| `graph.cache[node_id]` | `{node, round, timestamp, snapshots, references}` for the active cache round; removed nodes are filtered out |
| `graph.cache[node_id].snapshots[]` | `{version, node, references, round, timestamp, hash, transactions, signature}`; transaction entries are hashes, with no RPC `hex`, `topology`, or `witness` fields |
| `graph.final[node_id]` | `{node, round, start, end, hash}` for the preceding final round; removed nodes are filtered out |
| `queue.state[node_id]` | `[cache_actions, final_actions]` for accepted chains; `queue.caches` and `queue.finals` are their totals |
| `metric.transport` | Optional `sent` and `received` objects of message counters; `{}` when transport metrics are disabled |

`sps` and `tps` are locally sampled snapshot and deduplicated transaction rates, updated every 60 seconds. `spt` is the average number of snapshot appearances per distinct transaction in the sampling interval. These values are not network-wide throughput guarantees. Transport counter keys include `ping`, `authentication`, `graph`, `snapshot-confirm`, `transaction-request`, `transaction`, `snapshot-announcement`, `snapshot-commitment`, `transaciton-challenge` (this exact spelling), `snapshot-response`, `snapshot-finalization`, `commitments`, `full-challenge`, `transaction-bundle`, `finalized-transaction-bundle`, and `relay`.

`dumpgraphhead` returns an array of sync points sorted by node ID. Each item has this form:

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
  "references": null,
  "extra": "",
  "hash": "<transaction hash>",
  "hex": "<signed transaction encoding>",
  "snapshot": "<finalizing snapshot hash>"
}
```

`hex` is added by transaction lookup methods but is absent from transactions expanded inside a snapshot or mint distribution. `snapshot` is present only when a lookup resolves a finalization record. `references` is an array of transaction-hash strings or `null` when its underlying list is nil. `extra` is hex, including `""` for empty bytes.

The normalized object omits transaction authorization signatures. Use `hex` for the complete envelope. Each rendered input selects one of these forms, in priority order: nonzero source hash, genesis bytes, deposit, or mint. This projection is not a lossless representation of every field in a decoded input.

```json
{"hash":"<transaction hash>","index":0}
```

```json
{"genesis":"<hex-encoded genesis bytes>"}
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

`transactions` is an array of hashes or expanded transaction objects, depending on the method and expansion flag. `references` is `null` for round zero and otherwise contains the two round hashes. When included, `signature` is a 144-character hex string: the 64-byte collective signature followed by the 64-bit signer mask as 16 hex digits. Genesis has a null signature. A certificate must be verified against the snapshot payload and applicable signer set; the presence of this field alone is not verification.

`hex` is the complete stored snapshot encoding, including its collective signature and local topological order, even when the JSON signature field is omitted. The serving node produces `witness.signature`, a 128-character hex signature over `BLAKE3(hex-decoded snapshot bytes)`, using its private spend key. Verification requires a trusted binding to that node's public spend key. The returned `witness.timestamp` is generated for the response but is **not included in the signed bytes**. It does not provide authenticated freshness. This witness attests to the serving node's snapshot encoding; it is not a quorum certificate or proof of a global topological order.

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

Round snapshots contain transaction hashes and omit the JSON `signature` field, while retaining `hex`, `topology`, and `witness`. `references` can be `null` for the initial round. For a completed round, `hash` commits the canonical snapshot sequence and `start`/`end` are its minimum/maximum snapshot timestamps. For the active head, `hash` is the node ID used as a storage lookup key, and both `start` and `end` are the stored head timestamp; they are not a completed-round hash or duration. See the snapshot guide for the round-hash construction.

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
  "lock": "<reserving or consuming transaction hash>"
}
```

`lock` identifies the transaction that reserves or consumes the output; it is omitted when no lock hash is stored. The owner may be pending or finalized, and a retained reservation does not guarantee that its body is still cached or progressing. `getutxo` reports stored outputs and locks, not a promise that an output is currently spendable. It also omits specialized output metadata such as withdrawal fields; the full transaction is available through `gettransaction`.

For a valid key lookup without a storage error, `getkey` returns an object. `transaction` is `null` when the key is not reserved or recorded, and otherwise is a transaction-hash string:

```json
{"transaction":null}
```

A ghost-key record is not proof that its transaction finalized or that the corresponding output remains unspent.

`getasset` returns:

```json
{
  "id": "<asset identifier>",
  "chain": "<external chain identifier>",
  "asset_key": "<external asset key>",
  "balance": "<ledger-wide amount>"
}
```

The asset balance tracks ledger issuance less withdrawal submissions. It is not an independent measurement of external-chain reserves.

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

The CLI unwraps the server's `data` field and prints its JSON. A `data: null` result becomes an empty output line; errors are returned to the command and printed by the CLI. The shared Go client uses a 20-second HTTP timeout and treats a non-200 response or a non-null `error` field as an error. It does not submit an `id` or expose the response `runtime`. Global options precede the command:

```bash
./mixin --node http://127.0.0.1:6860 getinfo
```

| RPC method | CLI command and flags |
| --- | --- |
| `getinfo` | `getinfo` |
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

For list commands, `--since` defaults to 0, `--count` to 10, and boolean flags to false. `listallnodes --threshold` and numeric lookup indexes/round numbers default to 0. The Go `rpc.GetSnapshot` and `rpc.GetTransaction` helpers decode the returned `hex`; they do not establish the serving node's signer identity, validate a witness, or verify a quorum certificate.

`createaddress`, `decodeaddress`, `decoderawtransaction`, and `decodesignature` are local utilities, not RPC methods. Transaction builders and signers also run locally, although they may query RPC for source UTXO data. See the [README](../README.md) and [transaction guide](./mixin-kernel-transactions.md) for those workflows.

## Object server

When `rpc.object-server = true`, an XIN transaction present in the node's durable transaction store exposes its `extra` bytes through a content-oriented HTTP endpoint:

```text
GET /objects/<transaction-hash>
```

The response body is the raw `extra` value, without a `data` envelope. Empty and UTF-8 content is served as `text/plain; charset=utf-8`; non-UTF-8 content is `application/octet-stream`. Content is served as `application/json; charset=utf-8` only when the first byte is `{` and the complete value decodes as a JSON object. Arrays and objects with leading whitespace take the ordinary text/binary path.

For an object recognized by that rule, one top-level field can be retrieved with:

```text
GET /objects/<transaction-hash>/<field>
```

The field value is converted to a string using Go's default formatting, rather than JSON serialization. Missing or null fields return the literal text `<nil>`; nested objects are not returned as JSON subdocuments. Only the first field path segment is used, and additional segments are ignored. A field suffix on non-object content leaves the full `extra` response unchanged.

If that string is a `data:` URI with exactly one comma, the server uses its media type and optional charset. A final `;base64` marker enables standard-base64 decoding; other data is used without percent-decoding. Malformed base64 is not reported as an RPC validation error. Object responses include `Cache-Control: max-age=31536000, public`, `Content-Security-Policy: sandbox`, and `X-Content-Type-Options: nosniff`.

The object handler does not enforce GET-only access; GET is the retrieval convention shown here. It serves only XIN transactions in durable storage and does not require a finalization record. One-year HTTP caching therefore does not establish ledger finality. Clients requiring finality must check the transaction's `snapshot` and the corresponding certificate. Invalid, missing, or non-XIN objects return a JSON `error` envelope with HTTP `200`; disabled object routes return a bad-request envelope. Object responses do not carry a request ID or runtime. Storage transaction construction is documented in [STORAGE.md](../STORAGE.md).

The request dispatcher and response projections are defined in [`rpc/internal/server`](../rpc/internal/server); pagination and lookup behavior also depend on the storage methods it calls. CLI flags are defined in [`main.go`](../main.go), with calls in [`command.go`](../command.go) and HTTP handling in [`rpc/client.go`](../rpc/client.go).
