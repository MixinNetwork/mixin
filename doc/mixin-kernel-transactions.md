# Mixin Kernel Transactions

Transactions are the state-transition layer of Mixin Kernel. A transaction consumes existing outputs or a protocol-defined special input, creates new outputs of the same asset, and carries the authorization needed to perform that transition. Consensus does not change a transaction's identity: it finalizes the transaction by committing its hash in a collectively signed [snapshot](./mixin-kernel-snapshots.md).

For the relationship among transactions, snapshots, rounds, node chains, and consensus, see the [technical paper](./mixin-kernel-technical-paper.md).

## Encoding and identity

Kernel accepts transaction version `5` (`0x05`). Transactions use the repository's deterministic binary encoder, beginning with the format marker `77770005`; JSON is an API and tooling representation, not the consensus wire encoding.

The transaction identifier is the BLAKE3 hash of the encoded transaction payload without authorization signatures:

$$
H_{tx} = \mathrm{BLAKE3}\bigl(\mathrm{EncodeUnsigned}(tx)\bigr).
$$

The asset, inputs, outputs, references, and extra data are therefore committed by the identifier. Signature maps or an aggregate signature authorize that fixed identifier but do not become part of it. Different valid signature representations of the same payload retain the same transaction hash.

## JSON representation

The following schematic object shows the form returned by `gettransaction` and `decoderawtransaction`. Placeholder strings stand for full hexadecimal values.

```json
{
  "version": 5,
  "asset": "<32-byte asset identifier>",
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
      "keys": ["<one-time ghost public key>"],
      "mask": "<output mask public key>",
      "script": "fffe01"
    }
  ],
  "references": [],
  "extra": "",
  "hash": "<transaction hash>"
}
```

`gettransaction` also returns the signed binary transaction as `hex` and, once finalized, the containing snapshot hash as `snapshot`. Signatures remain in the binary representation; the normalized JSON object focuses on the state transition.

| Field | Meaning |
| --- | --- |
| `version` | Transaction encoding version, currently `5` |
| `asset` | 32-byte identifier of the one asset moved by the transaction |
| `inputs` | Ordinary UTXO references or a special genesis, deposit, or mint input |
| `outputs` | New amounts and their spending conditions |
| `references` | Finalized transactions committed as dependencies without spending an output |
| `extra` | Application or protocol data encoded as hexadecimal bytes |
| `hash` | BLAKE3 identifier computed from the unsigned binary payload |

Amounts are decimal strings with eight fractional places in normalized output. Consensus arithmetic uses fixed-precision integers, not floating point. Except for type-specific protocol rules, value is conserved exactly:

$$
\sum_i amount(input_i) = \sum_j amount(output_j).
$$

## Input forms

### Ordinary UTXO input

Most transactions spend an earlier output by transaction hash and zero-based output index:

```json
{
  "hash": "<source transaction hash>",
  "index": 0
}
```

The referenced output must exist, have the same asset as the new transaction, remain unspent and unlocked by another candidate, and receive enough valid signatures to satisfy its script.

### Deposit input

A deposit introduces an asset observed on an external domain:

```json
{
  "deposit": {
    "chain": "<external chain identifier>",
    "asset_key": "<asset identifier on that chain>",
    "transaction": "<external transaction identifier>",
    "index": 0,
    "amount": "2.15226159"
  }
}
```

The tuple `(chain, transaction, index)` is locked against replay. Deposit validation also checks the active custodian authorization and the asset mapping derived from `chain` and `asset_key`.

### Mint input

Protocol mint distributions use a batch-numbered input:

```json
{
  "mint": {
    "group": "UNIVERSAL",
    "batch": 200,
    "amount": "123.28767117"
  }
}
```

Mint transactions are generated and validated by the protocol. They are not ordinary wallet-created transfers.

### Genesis input

Genesis allocation transactions contain an encoded `genesis` value. This form is restricted to network initialization and is rejected by normal post-genesis transaction validation.

## Outputs and spending conditions

An ordinary output contains:

- `type`: the rule family governing how the output may later be consumed;
- `amount`: a positive fixed-precision quantity;
- `keys`: recipient-derived one-time public keys, called ghost keys;
- `mask`: the ephemeral public key used by a recipient to recognize and derive its ghost key;
- `script`: the threshold condition for spending the output.

The standard threshold script is three bytes:

```text
ff fe T
```

`T` is a threshold from 0 through 64. For example, `fffe01` requires one valid signature from the output's key list. An output may carry up to 256 keys, but its script threshold cannot exceed 64.

Mixin addresses contain public view and spend keys. A sender combines the recipient keys with fresh output randomness and the output index to create a ghost key. The recipient's private view key recognizes the output; the private spend key is additionally required to derive the signing key. Ghost keys reduce address reuse on the public ledger, but they do not hide asset identifiers or amounts.

Some output types carry protocol data instead of an ordinary script. Current categories include:

| Output type | Value | Purpose |
| --- | ---: | --- |
| Script | `0x00` | Ordinary transfer or storage output |
| Withdrawal submit | `0xa1` | Request an external withdrawal |
| Node pledge | `0xa3` | Lock the node pledge |
| Node accept | `0xa4` | Mark an accepted node pledge |
| Node remove | `0xa6` | Return a removed node's pledge to its payee |
| Withdrawal claim | `0xa9` | Complete withdrawal accounting |
| Node cancel | `0xaa` | Record cancellation and refund a pending pledge |
| Custodian update | `0xb1` | Change custodian protocol state |
| Custodian slash | `0xb2` | Apply a custodian-state penalty |

The transaction class is inferred from special inputs and output types; callers do not submit a separate trusted transaction-type field.

## References and extra data

A reference commits another transaction hash without consuming one of its outputs. Every referenced transaction must already be finalized. A transaction may contain at most 16 references.

General transactions may carry up to 256 bytes in `extra`. A qualifying XIN storage output with script `fffe40` buys additional capacity in 1 KiB steps per `0.0001` XIN, up to the 4 MiB transaction ceiling. See [STORAGE.md](../STORAGE.md) for the object-storage convention built on this mechanism.

## Authorization

Each ordinary input inherits the key list and threshold script of the output it spends. Signatures cover `H_tx`, so modifying any committed field after signing invalidates authorization.

Version 5 supports two authorization representations:

1. **Signature maps.** Each input maps key indexes to Edwards25519 signatures. The validator verifies the selected signatures together with batch verification.
2. **Aggregate signature.** One signature and an ordered signer-index set authorize selected keys across the transaction's inputs.

These mechanisms optimize authorization within one transaction. They are separate from the collective signature on a snapshot, which establishes Byzantine agreement across Kernel nodes.

## Validation rules and limits

Before a transaction can enter a snapshot, a node checks structural, cryptographic, and state-dependent rules. The principal implementation limits are:

| Item | Limit |
| --- | ---: |
| Encoded transaction size | 4 MiB |
| Inputs | 256 |
| Outputs | 256 |
| References | 16 |
| Input output-index value | 1024 |
| Keys on one output | 256 |
| Ordinary `extra` data | 256 bytes |
| Storage-capable `extra` data | Up to 4 MiB, subject to purchased capacity and total size |

Validation establishes that:

1. The encoding version and inferred transaction class are supported.
2. Counts, indexes, scripts, keys, extra data, and encoded size are within bounds.
3. Each ordinary input exists, belongs to the declared asset, and appears only once.
4. Inputs, deposit identifiers, mint batches, and output ghost keys do not conflict with existing locks.
5. The selected keys satisfy every threshold script and all signatures verify over the payload hash.
6. Output amounts are positive, masks and ghost keys are valid, and total input and output values match.
7. Every reference exists and is finalized.
8. Deposit, mint, withdrawal, node, and custodian transactions satisfy their additional protocol rules.

Validation may reserve inputs and ghost keys while a candidate waits for consensus. Spendable outputs and the permanent transaction-to-snapshot record are created only at finalization.

## Snapshot batching and finality

Eligible transactions are grouped by hash into a version 2 snapshot. One snapshot carries between 1 and 255 transaction hashes, sorted canonically before hashing. The consensus set validates the entire candidate and collectively signs the snapshot hash.

| Transaction class | May share a snapshot |
| --- | :---: |
| Ordinary script transfer or storage transaction | Yes |
| Deposit | Yes |
| Withdrawal submit | Yes |
| Withdrawal claim | Yes |
| Mint | No |
| Node pledge, cancel, accept, or remove | No |
| Custodian update or slash | No |

Consensus-sensitive operations remain alone because they can change membership or other protocol state used to validate subsequent work. For a batched snapshot, every included transaction still retains its own hash and UTXO effects; the transactions share only the snapshot envelope and collective-signature exchange.

The lifecycle is:

```text
construct → sign → submit → validate and lock → batch into snapshot
          → collective signature → atomic transaction finalization
```

`sendrawtransaction` returning a hash means the node accepted the transaction into its processing path. Durable finality is demonstrated by `gettransaction` returning a `snapshot` field and by retrieving that finalized snapshot.

## CLI workflow

For an ordinary transfer whose inputs belong to one address, `buildrawtransaction` constructs outputs, reads source UTXO keys from RPC, signs every input, and prints the signed binary transaction as hexadecimal:

```bash
RAW=$(./mixin --node http://127.0.0.1:6860 buildrawtransaction \
  --asset ASSET_ID \
  --inputs FIRST_TRANSACTION_HASH:0,SECOND_TRANSACTION_HASH:1 \
  --outputs XIN_FIRST_RECIPIENT:1.25000000,XIN_SECOND_RECIPIENT:0.75000000 \
  --extra 68656c6c6f \
  --view PRIVATE_VIEW_KEY \
  --spend PRIVATE_SPEND_KEY)

./mixin decoderawtransaction --raw "$RAW"
./mixin --node http://127.0.0.1:6860 sendrawtransaction --raw "$RAW"
```

`signrawtransaction` accepts a version 5 JSON construction object. An output can contain `accounts` so the tool derives fresh `keys` and `mask`, or it can contain precomputed `keys` and `mask`. Each `--key` value is the 32-byte private view key concatenated with the 32-byte private spend key, encoded as 128 hexadecimal characters.

The command-line utilities are convenient for development and recovery. Because private arguments may be exposed through shell history or process inspection, production signing should use protected application code or an isolated signer.

## Querying transactions

```bash
./mixin --node http://127.0.0.1:6860 gettransaction --hash TRANSACTION_HASH
./mixin --node http://127.0.0.1:6860 getcachetransaction --hash TRANSACTION_HASH
./mixin --node http://127.0.0.1:6860 getutxo --hash TRANSACTION_HASH --index 0
```

`gettransaction` reads the durable transaction store and includes `snapshot` once the transaction is final. `getcachetransaction` reads the earlier, unfinalized TTL cache. `getutxo` returns the current output and any lock held by a competing candidate. The full HTTP method definitions are in the [RPC reference](./remote-procedure-calls.md).
