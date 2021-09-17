# Mixin Custodian

The custodian connects to a private Mixin Kernel node to get the latest custodian list, we call it _M_, all the custodians in _M_ will do a DKG.

Because the kernel nodes change on a daily basis, the custodian list may change to _Mc_. The custodians will not do a new DKG unless, _Nd_ the number of differences between _Mc_ and _M_, statisfies _Nd > M/7_.

After a new DKG, the custodians transfer assets to _Mc_, and _Mc_ becomes the new _M_.

And all custodians pass messages to each other through a secure end-to-end encrypted Mixin Messenger chat group.

## NOTICE

1. Never deploy this on public servers.
2. Never connect this to public Mixin Kernel nodes.
