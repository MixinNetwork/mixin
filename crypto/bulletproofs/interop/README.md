# P2Pool interoperability test

This nested module pins
`git.gammaspectra.live/P2Pool/consensus/v5@v5.0.10` (commit
`63f673c8425628b586e9ac1c58cb4df6d9965218`). It reproduces P2Pool's
1-through-16 commitment aggregate-proof test and additionally requires:

- identical ordinary commitment encodings;
- byte-for-byte identical proofs for identical witnesses and randomness;
- P2Pool verification of proofs produced by Mixin; and
- Mixin verification of proofs produced by P2Pool.

Run it separately from the repository's main module:

```sh
cd crypto/bulletproofs/interop
go test -count=1 ./...
```
