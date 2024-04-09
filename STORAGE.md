# Mixin Object Storage

Mixin Kernel offers a permanent and decentralized object storage solution in the distributed ledger.

## Store

An object storage transaction is a normal Mixin Kernel transaction that supports a maximum 4MB arbitrary data in the transaction extra. The transaction must comply with the following rules:

1. The transaction asset must be XIN.
2. The first output must be any 64/1 script keys, i.e. the output value is slashed.
3. The first output value must be no lower than `0.0001*(len(extra)/1024+1)`.

Then the data will be permanently stored in the decentralized Mixin Network.

## Retrieve

To retrieve the stored object, just use the `gettransaction` RPC call to any Mixin Kernel nodes. However we provide a simpler public HTTP API.

```
GET https://kernel.mixin.dev/objects/TX-HASH
```

This will respond the extra data of the transaction `TX-HASH`. For now the `Content-Type` could be `application/json`, `text/plain` and `application/octet-stream`.

If the extra is a JSON dictionary, it's possible to query each field at the first level:

```
GET https://kernel.mixin.dev/objectx/TX-HASH/FIELD
```

If the field data is a valid data URI scheme, then the parsed media type value, e.g. `image/webp`, will be used to set the HTTP response `Content-Type` header.

The host `kernel.mixin.dev` used in the sample could be replaced with any Mixin Kernel node RPC host.
