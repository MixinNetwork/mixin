# Mixin Custodian

The custodian connects to a private Mixin Kernel node to get the latest custodian list, we call it _M_, all the custodians in _M_ will do a DKG.

Because the kernel nodes change on a daily basis, the custodian list may change to _Mc_. The custodians will not do a new DKG unless, _Nd_ the number of differences between _Mc_ and _M_, satisfies _Nd > M/7_.

After a new DKG, the custodians transfer assets to _Mc_, and _Mc_ becomes the new _M_.

And all custodians pass messages to each other through a secure end-to-end encrypted Mixin Messenger chat group.

## Bitcoin

Use a segwit address as custodian, and transfer coins to new address whenever a new DKG happens. This also applies to all Bitcoin similar blockchains without smart contracts.

## Ethereum

Use a contract as custodian which is owned by the DKG address, and change the owner to new address whenever a new DKG happens. This also applies to all smart contracts enabled blockchains.

## NOTICE

custodian <---> private proxy <---> kernel and mm

0. Never allow any incoming traffic.
1. only allow outbound traffic to the proxy.
2. Never deploy this on public servers.
3. Never connect this to public Mixin Kernel nodes.
