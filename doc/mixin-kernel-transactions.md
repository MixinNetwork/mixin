# Mixin Kernel Transactions

Transactions are the state-transition layer of Mixin Kernel. A transaction consumes existing outputs or a protocol-defined special input, creates new outputs of the same asset, and carries the authorization needed to perform that transition. Consensus does not change a transaction's identity: it finalizes the transaction by committing its hash in a collectively signed [snapshot](./mixin-kernel-snapshots.md).

For the relationship among transactions, snapshots, rounds, node chains, and consensus, see the [technical paper](./mixin-kernel-technical-paper.md).

## Encoding and identity

Kernel accepts transaction version `5` (`0x05`). Transactions use the repository's deterministic binary encoder, beginning with the format marker `77770005`. Decoding requires the received bytes to match the canonical re-encoding. JSON is an API and tooling representation, not the consensus wire encoding.

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

`gettransaction` also returns the signed binary transaction as `hex`. Once finalized locally, its `snapshot` field identifies the first snapshot recorded as finalizing the transaction. The RPC's normalized JSON omits authorization signatures; they remain in `hex`. The CLI's `decoderawtransaction` additionally exposes `signatures` or `aggregated` authorization data.

| Field | Meaning |
| --- | --- |
| `version` | Transaction encoding version, currently `5` |
| `asset` | 32-byte identifier of the one asset moved by the transaction |
| `inputs` | Ordinary UTXO references or a special genesis, deposit, or mint input |
| `outputs` | New amounts and their spending conditions |
| `references` | Finalized transactions committed as dependencies without spending an output |
| `extra` | Application or protocol data encoded as hexadecimal bytes |
| `hash` | BLAKE3 identifier computed from the unsigned binary payload |

Amounts are decimal strings with eight fractional places in normalized output. Consensus arithmetic uses fixed-precision integers, not floating point. Every post-genesis transaction passing the normal validator conserves value exactly:

$$
\sum_i amount(input_i) = \sum_j amount(output_j).
$$

Ordinary input amounts come from stored UTXOs. Deposit and mint inputs carry the amount that their outputs must reproduce. Genesis allocations use a separate initialization path.

## Input forms

### Ordinary UTXO input

Most transactions spend an earlier output by transaction hash and zero-based output index:

```json
{
  "hash": "<source transaction hash>",
  "index": 0
}
```

The referenced output must exist, have the same asset as the new transaction, and permit the proposed spending transaction type. Ordinary script transfers can consume script outputs and node-remove outputs, with signatures satisfying their scripts. Normal admission rejects an input locked by any different transaction. During finalized-snapshot replay, persistence can replace an unfinalized input-lock owner but cannot replace a finalized owner.

### Deposit input

A deposit records a custodian-authorized amount from an external domain:

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

The tuple `(chain, transaction, index)` is locked against replay. A deposit has exactly one input and one script output, and requires a signature over the transaction hash from the custodian selected at validation time. Validation checks the external asset metadata, its consistency with any existing mapping for the declared asset, and the applicable asset-capacity bounds. Kernel relies on the custodian's authorization for the external deposit; this validator does not independently observe external settlement or reserves.

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

Mint transactions use the XIN asset and the `UNIVERSAL` group, with one mint input and script outputs. The common validator checks the batch against stored mint distributions; applicable kernel snapshot rules rebuild the scheduled distribution. Mint transactions are generated by the protocol rather than submitted as ordinary wallet transfers.

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

`T` is a threshold from 0 through 64. For example, `fffe01` requires one valid signature from the output's key list. An output may carry up to 256 keys, but its script threshold cannot exceed 64. Validation permits a threshold greater than the number of keys, making the output unspendable. The ordinary-input verification path still requires verifiable signature entries, so `T = 0` is not a general signature-free spending mode.

Ghost keys must be valid, unique across every output in a transaction, and available for reservation by its payload hash. Script outputs require a valid nonzero mask. Withdrawal-submit, withdrawal-claim, node-pledge, and node-accept outputs instead require no keys, an empty script, and a zero mask.

Mixin addresses contain public view and spend keys. A sender combines the recipient keys with fresh output randomness and the output index to create a ghost key. The recipient's private view key recognizes the output; the private spend key is additionally required to derive the signing key. Ghost keys reduce address reuse on the public ledger, but they do not hide asset identifiers or amounts.

Some output types carry protocol data instead of an ordinary script. Current categories include:

| Output type | Value | Purpose |
| --- | ---: | --- |
| Script | `0x00` | Ordinary transfer or storage output |
| Withdrawal submit | `0xa1` | Request an external withdrawal |
| Node pledge | `0xa3` | Lock the node pledge |
| Node accept | `0xa4` | Mark an accepted node pledge |
| Node remove | `0xa6` | Return a removed node's pledge to its payee |
| Withdrawal claim | `0xa9` | Record a claim against a finalized withdrawal submission |
| Custodian update | `0xb1` | Change custodian protocol state |
| Custodian slash | `0xb2` | Recognized type; validation returns an unimplemented error |

The transaction class is inferred from special inputs and output types; callers do not submit a separate trusted transaction-type field.

Pledge outputs can only be consumed by node-accept transactions; accepted pledges can only be consumed by node-remove transactions. There is no general wallet spend or timed refund for those outputs. Acceptance requires a snapshot timestamp between 12 hours and 7 days after the pledge. A pending pledge has no automatic expiry transition and prevents normal node removal while it remains pending. See [node operations](./mixin-kernel-node-operations.md) for the membership lifecycle and removal conditions.

## Withdrawals and asset accounting

A withdrawal-submit transaction spends script outputs. Its first output carries the external `address` and `tag` in a `withdrawal` object; any remaining outputs are script change. Finalization subtracts the submitted amount from the asset's recorded total. The submission output has no ordinary spending or refund path, and Kernel does not execute the external payout.

A withdrawal claim is a separate XIN transaction spending script outputs. Its first output is a claim amount of at least `0.0001` XIN, any remaining outputs are script change, and its sole reference identifies a finalized withdrawal submission. Validation rejects a submission that already has a recorded claim. The first 64 bytes of `extra` carry a custodian signature over `BLAKE3(submission hash || remaining extra bytes)`, using the custodian selected at validation time.

During finalized replay (`fork = true`) at a snapshot timestamp before 2026-09-01 00:00:00 UTC, the claim validator permits that custodian-signature check to fail after the preceding claim checks succeed. Normal admission requires the signature at every timestamp, and finalized replay requires it at or after that boundary.

Claim finalization records the association with the submission; it does not spend the submission output or reduce the withdrawn asset's total again. The claim output has no ordinary spending path. Deposits, mint distributions, and genesis allocations increase recorded asset totals. Ordinary transfers, storage outputs, and claim fees do not change those totals. These totals describe Kernel accounting, not independently verified external backing or the sum of outputs that remain spendable.

## References and extra data

A reference commits another transaction hash without consuming one of its outputs. Every referenced transaction must already be finalized. A transaction may contain at most 16 references.

Mint and node-remove transactions use a consensus-state reference followed by a nonzero anchor transaction hash for deterministic ghost-key derivation. Their builders select the anchor from the proposer's latest finalized round. Validators rebuild from the recorded anchor and require finalized references, but do not enforce its age or membership in that round. Reference-seeded derivation applies on non-mainnet networks and on mainnet for snapshot timestamps at or after 2026-09-01 00:00:00 UTC; earlier mainnet timestamps use unanchored derivation and one consensus-state reference.

General transactions may carry up to 256 bytes in `extra`. For a XIN script transaction, a storage output with exactly one key and script `fffe40` buys capacity in 1 KiB steps per `0.0001` XIN. The validator selects the largest qualifying output regardless of its position; it does not add together several outputs' payments. Capacity is the number of complete price steps multiplied by 1 KiB, capped at 4 MiB and subject to the total transaction-size limits. The storage output itself is unspendable because its threshold is 64 and it has one key.

A qualifying custodian-update output allows up to 4 MiB of `extra` under its own protocol rules. Transaction encoding and signatures also consume space, so usable extra data must fit below the 4 MiB envelope limit. See [STORAGE.md](../STORAGE.md) for the object-storage retrieval convention built on transaction extra data.

## Authorization

An input spending a script or node-remove output inherits its key list and threshold script. Signatures cover `H_tx`, so modifying any committed field after signing invalidates authorization. Other protocol outputs use the transaction-type rules described above.

Version 5 supports two authorization representations:

1. **Signature maps.** Each input maps key indexes to Edwards25519 signatures. The validator verifies the selected signatures together with batch verification.
2. **Aggregate signature.** One signature and a strictly increasing signer-index set authorize selected keys across the concatenated key lists of the transaction's inputs.

These mechanisms optimize authorization within one transaction. They are separate from the collective signature on a snapshot, which establishes Byzantine agreement across Kernel nodes.

## Validation rules and limits

The normal path for admitting a new transaction body checks structural, cryptographic, and state-dependent rules. The principal implementation limits are:

| Item | Limit |
| --- | ---: |
| Decoded signed envelope | 4 MiB |
| Unsigned encoded payload | 4 MiB |
| Inputs | 256 |
| Outputs | 256 |
| References | 16 |
| Input output-index value | 1024 |
| Keys on one output | 256 |
| Ordinary `extra` data | 256 bytes |
| Storage-capable `extra` data | Up to 4 MiB, subject to purchased capacity and total size |

Full validation checks that:

1. The encoding version and inferred transaction class are supported.
2. Counts, indexes, scripts, keys, extra data, and encoded size are within bounds.
3. Each ordinary input exists, belongs to the declared asset, and appears only once.
4. Ordinary inputs satisfy the applicable lock rules; deposit and mint inputs satisfy their type-specific replay rules.
5. Selected ordinary-input keys satisfy their scripts and signatures verify over the payload hash; protocol inputs satisfy their own authorization rules.
6. Output amounts are positive, ghost keys are valid and unique, script and mask fields match the output type, and total input and output values match.
7. Every reference exists and is finalized.
8. Deposit, mint, withdrawal, node, and custodian transactions satisfy their additional protocol rules.

Only after these checks, including type-specific validation, does common validation reserve output ghost keys. It does not reserve ordinary inputs, deposit identifiers, or mint batches by itself.

## Admission, cache, and candidate locks

RPC admission permits script, deposit, withdrawal-submit, withdrawal-claim, and node-pledge transactions, plus custodian updates on non-mainnet networks. Mint, node-accept, and node-remove transactions enter through protocol processing.

A new, uncached submission of a permitted type runs common validation, including ghost-key reservation, before entering the TTL cache. A submission whose hash is already finalized returns that hash. A payload-hash cache hit requeues the first retained authorization envelope without validating the newly submitted signatures; it does not replace the cached envelope. Cache entries can also arrive from peers before common validation. The queue worker validates the retained envelope before routing it for a snapshot.

Snapshot transaction validation treats cache bodies and durable candidate bodies differently. A body obtained from the cache undergoes common validation at the snapshot timestamp and applicable kernel checks, with individual ghost-key writes deferred. The node then commits the pending bodies, ghost keys, and ordinary-input/deposit/mint locks together through `LockAndPersistTransactions`. A body already in durable candidate storage reuses its prior common validation and undergoes applicable kernel checks; common time-dependent checks are not repeated at each snapshot timestamp.

If a candidate batch exceeds Badger's transaction limit, the kernel falls back to per-transaction revalidation and bounded commits. Admission, cache insertion, candidate persistence, and snapshot finalization are separate operations. A later failure does not roll back a completed earlier reservation or a previously committed fallback transaction.

Candidate locks survive restarts and have no general timeout or unlock operation. Requeueing restores queue records and leaves locks in place; retrying the same payload can reuse its own locks. Finalized replay can replace certain input locks held by an unfinalized transaction and prune that transaction's durable body, but does not release all its other reservations. Ghost keys remain bound to the payload hash that first reserved them, including on the replay path.

## Snapshot batching and finality

Eligible transactions are grouped by hash into a version 2 snapshot. One snapshot carries between 1 and 255 unique transaction hashes, sorted canonically before hashing. Validators apply snapshot rules and the transaction-admission checks described above, then collectively sign the snapshot hash.

| Transaction class | May share a snapshot |
| --- | :---: |
| Ordinary script transfer or storage transaction | Yes |
| Deposit | Yes |
| Withdrawal submit | Yes |
| Withdrawal claim | Yes |
| Mint | No |
| Node pledge, accept, or remove | No |
| Custodian update | No |
| Custodian slash | Rejected as unimplemented |

Mint, node, and custodian-update operations remain alone because they affect protocol state used to validate subsequent work. For a batched snapshot, every included transaction retains its own hash and output effects. Newly admitted transactions cannot consume one another's pending outputs: inputs must already exist in finalized storage.

The lifecycle of a new ordinary submission is:

```text
construct → sign → submit and reserve ghost keys → propose snapshot
          → validate and persist candidate bodies and input locks
          → collective signature → commit snapshot and transaction effects
```

At snapshot commit, one database transaction writes the snapshot, each newly finalized transaction's marker and outputs, asset-accounting and protocol-state effects, and local topology/work records. Reprocessing an already finalized transaction does not apply its output or accounting effects again. Candidate persistence alone creates no spendable outputs or finalization marker.

`sendrawtransaction` returning a hash acknowledges the node's processing path or an existing transaction; it is not a finality receipt. A trusted node's `gettransaction` response with a `snapshot` field reports local durable finalization. That snapshot can be retrieved for its transaction hashes and collective certificate; independently establishing finality also requires verifying the certificate against the applicable consensus membership.

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

`signrawtransaction --raw` accepts a version 5 JSON construction object with `asset`, `inputs`, `outputs`, and hexadecimal `extra`. Inputs can provide `keys` and `mask` for local signing or let the tool fetch them from RPC. An output can contain `accounts` so the tool derives `keys` and `mask`, or precomputed `keys` with a nonzero `mask`. Both construction commands generate a random seed by default and accept a 64-byte hexadecimal `--seed` for deterministic derivation.

Each `--key` value is the 32-byte private view key concatenated with the 32-byte private spend key, encoded as 128 hexadecimal characters. The signing command rebuilds the transaction and its signatures. Its construction schema supports ordinary and deposit inputs, but does not copy existing authorization, transaction references, mint/genesis inputs, or withdrawal metadata from arbitrary transaction JSON.

The command-line utilities are convenient for development and recovery. Because private arguments may be exposed through shell history or process inspection, production signing should use protected application code or an isolated signer.

## Querying transactions

```bash
./mixin --node http://127.0.0.1:6860 gettransaction --hash TRANSACTION_HASH
./mixin --node http://127.0.0.1:6860 getcachetransaction --hash TRANSACTION_HASH
./mixin --node http://127.0.0.1:6860 getutxo --hash TRANSACTION_HASH --index 0
```

`gettransaction` reads durable candidate and finalized bodies, including `snapshot` when the queried node has finalized the transaction. `getcachetransaction` reads the transient TTL cache; an entry's presence does not establish validation or finality, and it may remain cached after finalization. `getutxo` returns a stored output and its optional `lock` owner. Spent output records remain stored, so a nonzero lock may identify either a pending candidate or a finalized spender. The full HTTP method definitions are in the [RPC reference](./remote-procedure-calls.md).
