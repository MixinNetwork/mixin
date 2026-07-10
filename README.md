# Mixin Kernel

Mixin Kernel is a fast distributed ledger for digital assets. It combines a UTXO state model, parallel per-node chains, a cross-referenced directed acyclic graph (DAG), and Byzantine-fault-tolerant collective signatures. The Trusted Execution Environment design is not integrated into this repository.

Kernel does not wait for a single global block producer. Accepted nodes propose snapshots concurrently, and the active consensus set certifies each post-genesis snapshot with a supermajority signature. A snapshot can commit up to 255 eligible transactions, amortizing the consensus exchange while preserving an independent hash, authorization, validation result, and finalization record for every transaction.

The main protocol objects fit together as follows:

```text
signed transactions
        ↓
validated transaction cache
        ↓
batched snapshots + collective signatures
        ↓
short rounds on each node chain
        ↓
cross-referenced BFT-DAG + durable UTXO state
```

For a complete explanation of the data model, consensus, networking, storage, and recovery paths, read [Mixin Kernel: A Fast BFT-DAG Distributed Ledger](doc/mixin-kernel-technical-paper.md).

## Build

Install the Go version declared in [go.mod](go.mod), then build with `make`. The Makefile injects the repository revision into the binary; a plain `go build` intentionally leaves an invalid build version.

```bash
git clone https://github.com/MixinNetwork/mixin.git
cd mixin
make
./mixin --version
```

The resulting `mixin` binary runs a Kernel node and provides tools for addresses, transactions, graph inspection, and RPC access. Run `./mixin --help` or `./mixin <command> --help` for the complete command reference.

## Run a local network

`setuptestnet` creates a fresh seven-node network under `/tmp/mixin-6861` through `/tmp/mixin-6867`. It prints the generated genesis document, network identifier, and test custodian credentials. Those credentials control deposits on that test network; store them if the network will be reused.

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

Running `setuptestnet` again replaces the configuration and genesis files in those directories but does not remove existing database contents. Use clean directories when creating a new local network.

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
- Treat RPC and the profiling endpoint as administrative interfaces; restrict them at the host or network boundary.

Then start the node:

```bash
./mixin kernel --dir "$HOME/mixin"
```

The example configuration serves RPC at `http://127.0.0.1:6860`. CLI commands use that endpoint by default. Override it with the global `--node` flag or the `MIXIN_KERNEL_RPC` environment variable.

Joining the consensus set also requires an on-ledger pledge and the automated acceptance sequence. See [Kernel node operations](doc/mixin-kernel-node-operations.md) before configuring a signer.

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

Transactions use version 5 deterministic binary encoding. An ordinary transfer consumes earlier UTXOs of one asset, creates one or more threshold-script outputs, and is signed over the unsigned payload hash. The following workflow builds and signs a one-input transfer by reading the source UTXO from a node:

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

The RPC server accepts JSON calls over HTTP `POST /`; `GET /` returns the same node summary as `getinfo`. A successful response contains `data`, while a rejected call contains `error`. The caller-provided `id` is copied into the response.

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
| [Node operations](doc/mixin-kernel-node-operations.md) | Configuration, pledge, acceptance, cancellation, and removal |
| [RPC reference](doc/remote-procedure-calls.md) | HTTP protocol, methods, parameters, and result objects |
| [Storage](STORAGE.md) | Object storage transactions and retrieval |
| [Inscription](INSCRIPTION.md) | Inscription data conventions |

## License

Mixin Kernel is released under the [GNU General Public License v3.0](LICENSE).
