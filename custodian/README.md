# Mixin Custodian

The custodian connects to a private Mixin Kernel node to get the latest custodian list, we call it _M_, all the custodians in _M_ will do a DKG.

Because the kernel nodes change on a daily basis, the custodian list may change to _Mc_. The custodians will not do a DKG evolution unless, _Nd_ the number of differences between _Mc_ and _M_, satisfies _Nd > M/7_.

After a new DKG, the custodians transfer assets to _Mc_, and _Mc_ becomes the new _M_.

And all custodians pass messages to each other through a secure end-to-end encrypted Mixin Messenger chat group.

## Evolution

All kernel nodes should have the consensus that when the custodian evolution threshold reached, they should wait for the new custodian key generation, until then, no nodes could be removed or pledged.

The custodian key should be bind to network-id and the current nodes transactions queue, after the generation, there should be a custodian key evolution transaction snapshot in the kernel.

The evolution transaction may have multiple outputs typed as CustodianEvolution, each output has public key, signature, algorithm and curve. Thus enable a single evolution transaction to update different kind of DKG keys. The signature algorithm can be Schnorr or BLS, and curve may be secp256k1, edwards25519, or anything.

## Deposit

A custodian deposit transaction submitted to the Kernel will be credited to the custodian DKG address, and all custodian nodes should check this transaction on Bitcoin or Ethereum blockchains respectively, through a proxy node.

## Withdrawal

A custodian withdrawal transaction submitted to the Kernel triggers the custodian to verify and submit the exact withdrawal transaction on Bitcoin or Ethereum.

The custodian withdrawal transaction can only be submitted by the domain, then the custodian should ensure this domain has enough balance, and the balance not below some threshold.

Let's assume the kernel has a total balance of 100BTC in the UTXOs, and the domain can't withdraw unless the remaining is more than 70BTC.

Whenever a UTXO is spent on the withdrawal submit transaction, the total balance is dropped, so the threshold.

## Bitcoin

Use a taproot address as custodian, and transfer coins to new address whenever a DKG evolution happens. This also applies to all Bitcoin similar blockchains without smart contracts.

## Ethereum

Use a contract as custodian which is owned by the DKG address, and change the owner to new address whenever a DKG evolution happens. This also applies to all smart contracts enabled blockchains.

## NOTICE

custodian <---> private proxy <---> kernel and mm

0. Never allow any incoming traffic.
1. Only allow outbound traffic to the proxy.
2. Never deploy this on public servers.
3. Never connect this to public Mixin Kernel nodes.
4. May use the Mixin Messenger API as a trust source to verify the custodian list.
