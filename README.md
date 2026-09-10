# Mixin Kernel

Mixin Kernel is a distributed ledger for digital assets. It combines a UTXO state model, parallel per-node chains, a cross-referenced directed acyclic graph (DAG), and collective signatures for Byzantine-fault-tolerant consensus.

Accepted nodes propose snapshots concurrently. Post-genesis snapshot certification requires a supermajority under the protocol's membership rules. A snapshot can contain up to 255 eligible transactions; each transaction retains its own payload hash, authorization rules, and finalization record.

The main protocol objects fit together as follows:

```text
signed transactions
        ↓
transaction cache
        ↓
snapshot validation + collective signing
        ↓
short rounds on each node chain
        ↓
cross-referenced BFT-DAG + durable UTXO state
```

Asset deposits require custodian authorization. Checking external reserves and executing external-chain withdrawals depend on separate custody services.

For the data model, consensus, networking, storage, and recovery paths, read the [technical paper](doc/mixin-kernel-technical-paper.md).

## Build

Install the Go version declared in [go.mod](go.mod), then build with `make`. The Makefile embeds the Git commit identifier in the build version; a binary built with plain `go build` refuses to start while the version contains `BUILD_VERSION`. Run `make` from a clean checkout: it restores `config/reader.go` from Git before and after compiling.

```bash
git clone https://github.com/MixinNetwork/mixin.git
cd mixin
make
./mixin --version
```

The resulting `mixin` binary runs a Kernel node and provides tools for addresses, transactions, graph inspection, and RPC access. Run `./mixin --help` or `./mixin <command> --help` for the complete command reference.

## Run a local network

`setuptestnet` generates configuration for a seven-node test network under `/tmp/mixin-6861` through `/tmp/mixin-6867`. It prints the generated genesis document, network identifier, and test custodian credentials. Those credentials authorize deposits on that test network; store them if the network will be reused.

```bash
./mixin setuptestnet
```

Start all seven nodes in separate terminals or process-supervisor entries:

```bash
./mixin kernel --dir /tmp/mixin-6861
./mixin kernel --dir /tmp/mixin-6862
# Continue with /tmp/mixin-6863 through /tmp/mixin-6867.
```

The generated nodes use P2P UDP ports `5851`–`5857` and RPC TCP ports `6861`–`6867`. Verify a node from another terminal:

```bash
./mixin --node http://127.0.0.1:6861 getinfo

curl -sS http://127.0.0.1:6861 \
  -H 'Content-Type: application/json' \
  --data '{"id":"example","method":"getinfo","params":[]}'
```

Each invocation of `setuptestnet` overwrites the configuration and genesis files in those directories but leaves database contents in place. Use clean directories for each independent local network.

## Run a node

A node data directory contains `genesis.json`, `config.toml`, and the databases created at runtime. Start from the configuration shipped in this repository:

```bash
mkdir -p "$HOME/mixin"
cp config/genesis.json "$HOME/mixin/genesis.json"
cp config/config.example.toml "$HOME/mixin/config.toml"
```

Before starting the daemon, edit `config.toml`:

- Replace `node.signer-key` with the node signer's private spend key. Never use the example key on a real node.
- Choose unused P2P, RPC, and optional profiling ports.
- Keep `p2p.relayer = false` on a consensus signer. A public relay may set it to `true` when it is intentionally reachable from the network.
- Review the seed list and expose the P2P UDP port if the node must accept direct peer connections.
- RPC and profiling listen on all interfaces at their configured ports. Restrict access at the host or network boundary; set `dev.port = 0` to disable profiling.

Then start the node:

```bash
./mixin kernel --dir "$HOME/mixin"
```

With the example configuration, RPC is reachable locally at `http://127.0.0.1:6860`. CLI commands use that endpoint by default. Override it with the global `--node` flag or the `MIXIN_KERNEL_RPC` environment variable.

Joining the consensus set also requires a 13,439 XIN pledge and the automated acceptance sequence. Acceptance must satisfy the seven-day pledge-to-accept timestamp limit. A finalized pledge has no normal cancellation or refund path; an expired pending pledge also prevents ordinary node removal. A delayed, admissible acceptance certificate with an eligible timestamp can still resolve that state. See [Kernel node operations](doc/mixin-kernel-node-operations.md) before committing a pledge.

## Addresses

A Mixin address begins with `XIN` and contains public view and spend keys. Generate an address with:

```bash
./mixin createaddress
```

Share the address to receive assets. Keep both private keys secret and backed up: the private view key discovers received outputs, while spending also requires the private spend key.

```bash
./mixin decodeaddress --address XIN_ADDRESS
```

`createaddress --public` deterministically derives the view key from the public spend key. Kernel uses this form for public node signer and payee identities; use it only when public observability is intentional.

## Transactions

Transactions use version 5 deterministic binary encoding. An ordinary transfer consumes earlier UTXOs of one asset, creates one or more threshold-script outputs, and is signed over the unsigned payload hash. Output amounts must sum exactly to input amounts; the builder requires an explicit change output when needed. The following workflow builds and signs a one-input transfer by reading the source UTXO's public key material from a node:

```bash
RAW=$(./mixin --node http://127.0.0.1:6860 buildrawtransaction \
  --asset ASSET_ID \
  --inputs SOURCE_TRANSACTION_HASH:0 \
  --outputs XIN_RECIPIENT_ADDRESS:1.00000000 \
  --view PRIVATE_VIEW_KEY \
  --spend PRIVATE_SPEND_KEY)

./mixin decoderawtransaction --raw "$RAW"
./mixin --node http://127.0.0.1:6860 sendrawtransaction --raw "$RAW"
```

Command-line arguments may be visible to other local users through process inspection or shell history. Production wallets should protect private keys and use the transaction packages directly or an appropriately isolated signing process.

See [Kernel transactions](doc/mixin-kernel-transactions.md) for the current schema, input and output forms, limits, signing model, and finalization lifecycle.

## RPC and command groups

The RPC server accepts JSON calls over HTTP `POST /`; `GET /` returns the same node summary as `getinfo`. A successful response contains `data`, while a rejected call contains `error`. Normal method responses echo a nonempty string `id` supplied by the caller.

A successful `sendrawtransaction` response alone does not establish finalization. `gettransaction` includes a `snapshot` field when the queried node records the transaction as finalized. RPC results reflect the selected node's view; applications releasing external funds need an appropriate finality-verification and node-trust policy.

Common CLI groups include:

| Purpose | Commands |
| --- | --- |
| Node and network | `kernel`, `setuptestnet`, `getinfo`, `listpeers`, `listrelayers` |
| Addresses and keys | `createaddress`, `decodeaddress`, `decryptghostkey`, `decodesignature` |
| Transactions | `buildrawtransaction`, `signrawtransaction`, `sendrawtransaction`, `decoderawtransaction` |
| Ledger queries | `gettransaction`, `getcachetransaction`, `getutxo`, `getkey`, `getasset` |
| Snapshots and rounds | `listsnapshots`, `getsnapshot`, `getroundbynumber`, `getroundbyhash`, `getroundlink` |
| Protocol state | `listallnodes`, `listmintworks`, `listmintdistributions`, `listcustodianupdates` |
| Local maintenance | `dumpgraphhead`, `validategraphentries`, `removegraphentries`, `updateheadreference` |

The maintenance commands can alter or inspect local graph storage. Do not use mutation commands without understanding their implementation and coordinating with the relevant node operators.

See [Remote procedure calls](doc/remote-procedure-calls.md) for every RPC method, parameter order, response envelope, and representative object schema.

## Documentation

| Document | Subject |
| --- | --- |
| [Technical paper](doc/mixin-kernel-technical-paper.md) | End-to-end architecture and BFT-DAG consensus |
| [Transactions](doc/mixin-kernel-transactions.md) | Version 5 transaction model, authorization, and validation |
| [Snapshots](doc/mixin-kernel-snapshots.md) | Version 2 snapshots, batching, finality, rounds, and topology |
| [Node operations](doc/mixin-kernel-node-operations.md) | Configuration, pledge, acceptance, stake restrictions, and removal |
| [RPC reference](doc/remote-procedure-calls.md) | HTTP protocol, methods, parameters, and result objects |
| [Storage](STORAGE.md) | Object storage transactions and retrieval |
| [Inscription](INSCRIPTION.md) | Inscription data conventions |

## License

Mixin Kernel is released under the [GNU General Public License v3.0](LICENSE).
