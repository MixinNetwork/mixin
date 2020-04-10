# Mixin Kernel Node Operations

This article describes the basic operations to run a Mixin Kernel Full Node, which validates all transactions and records snapshots in the ledger.

To read this article, you need to have a good understanding of the transaction data format. You can find details at [Mixin Kernel Transactions](./mixin-kernel-transactions.md).

## Boot Node

To boot a fresh Mixin Kernel Full Node, you need to generate your keys, send the pledge transaction and run the kernel daemon.

1. Get the latest release from https://github.com/MixinNetwork/mixin/releases and extract files `mixin`, `genesis.json`, `nodes.json` and `config.example.toml`.

2. Add `mixin` to your `$PATH`, then create a directory `~/mixin` with `genesis.json` and `nodes.json` in it.

3. Generate your own node signer and payee keys by executing `mixin createaddress --public` twice.

4. Store your signer and payee spend key securely and they can't be recovered if you lost them.

5. Rename `config.example.toml` to `config.toml` and put it in `~/mixin`. Edit `~/mixin/config.toml` with your own `signer-key` and `listener`.

6. Send the pledge transaction to any other running Kernel Node, if it fails due to pending node operations, wait and try again.

7. If your pledge transaction succeed, you can run the daemon `mixin kernel -d ~/mixin`.

## Kernel Concepts

There are 5 Kernel Node operations, `pledge`, `cancel`, `accept`, `resign` and `remove`.

Every successful Kernel Node operation will prevent further Kernel Node operations for at least 12 hours.

A Kernel Year is 365 natural days, and all nodes will be removed one by one at the beginning of a new Kernel Year.

The Kernel Pledge Amount is 10000 XIN for the first Kernel Year, and will change every Kernel Year.

## Pledge Transaction

This transaction will pledge your node, and the node will be accepted to the Kernel by the automatic accept transaction.

- **inputs**: any amount of inputs from any signers, with the exact total Kernel Pledge Amount and all should be the type `0x00`.

- **outputs**: one single output with type `0xa3`, the exact Kernel Pledge Amount, zero keys, empty script and empty mask.

- **extra**: 64 bytes extra which is the concatenation of signer public spend key and payee public spend key.

This transaction can be sent out at any time before 14 days of the next Kernel Year and after 24 hours of a new Kernel Year.

This transaction will block any further `pledge`, `cancel`, or `resign` transactions for at least 24 hours.

## Cancel Transaction

If you regret your `pledge` transaction before it gets accepted to the Kernel, you can send a cancel transaction to cancel the pledge operation.

- **inputs**: the single pledge transaction output as the only input, and signed by the exact keys of the first pledge input.

- **outputs**: two outputs while the first is type `0xaa`, amount 1/100 of the input, zero keys, empty script and empty mask.; the second is type `0x00`, amount the remaining of the input, script `0xfffe01`, keys and mask should be derived from the inputs signer.

- **extra**: 96 bytes, the first 64 bytes is the same as the pledge transaction extra, and the last 32 bytes is the private view key of the inputs signer.

This transaction must be sent out after at least 24 hours of the pledge transaction, no more than 168 hours after the pledge transaction and from 13:00 UTC to 19:00 UTC.

This transaction will block any further `pledge`, `cancel`, or `resign`  transactions for at least 24 hours.

## Accept Transaction

After the pledge transaction, you can start your kernel daemon, then the daemon will send out an accept transaction automatically.

- **inputs**: the single pledge transaction output as the only input.

- **outputs**: one single output with type `0xa4`, the exact pledge transaction amount, zero keys, empty script and empty mask.

- **extra**: 64 bytes same as the pledge transaction extra.

This transaction must get snapshot by the exact fresh node as its first snapshot.

This transaction must be sent out after at least 12 hours of the pledge transaction, no more than 168 hours after the pledge transaction and from 13:00 UTC to 19:00 UTC.

This transaction will block any further `pledge`, `cancel`, `resign` or `remove` transactions for at least 12 hours.

## Resign Transaction

You can send a resign transaction to remove your node after it gets accepted to the Kernel.

- **inputs**: one single input with type `0x00`, amount 1/100 of the pledge transaction and signed by the node payee key.

- **outputs**: one single output with type `0xa5`, amount 1/100 of the pledge transaction, zero keys, empty script and empty mask.

- **extra**: 64 bytes same as the pledge transaction extra.

This transaction can be sent out at any time before 14 days of the next Kernel Year and after 24 hours of a new Kernel Year.

This transaction will block any further `pledge`, `cancel`, or `resign` transactions for at least 24 hours.

## Remove Transaction

When a new Kernel Year starts or a resign transaction gets snapshot, the Kernel will send out a remove transaction automatically.

- **inputs**: the single accept transaction output as the only input.

- **outputs**: one single output with type `0xa6`, the exact accept transaction amount, script `0xfffe01`, keys and mask should be derived from the payee.

- **extra**: 64 bytes same as the accept transaction extra.

This transaction must not get snapshot by the node to be removed.

This transaction must be sent out after at least 12 hours of the resign transaction, and from 13:00 UTC to 19:00 UTC.

This transaction will block any further `pledge`, `cancel`, `resign` or `remove` transactions for at least 12 hours.
