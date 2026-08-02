# Bulletproofs+

This package is a native Go implementation of the Monero Bulletproofs+
construction. It is pinned to Monero `v0.18.5.1` tree
`4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5`, specifically
`src/ringct/bulletproofs_plus.cc` and `.h`. See `LICENSE.monero` for the
retained upstream notice and terms.

Protocol parameters:

- Edwards25519 with canonical point and scalar encodings;
- commitments `C = blind*G + value*H` using Monero's amount generator;
- 64-bit values;
- 1 to 16 commitments per aggregated proof;
- legacy Keccak-256 generator derivation and Fiat-Shamir transcript; and
- Monero's multiply-by-`1/8` proof-point wire convention.

Batch verification draws its random weights from `crypto/rand` only;
deterministic readers are confined to internal tests.

The prover uses constant-time multiscalar multiplication for witness-dependent
operations. Verification uses variable-time multiscalar multiplication because
all verifier inputs are public. Standalone proof parsing is strict and bounded.

## Tests

The main-module tests port Monero's boundary, aggregation, rejection-sampling,
and complete torsion-mutation matrix and add canonical decoding, truncation,
mutation, wrong-commitment, batch, and fuzz-seed coverage:

```sh
go test ./crypto/bulletproofs
```

Monero does not publish fixed serialized Bulletproofs+ fixtures in its
`tests/unit_tests/bulletproofs_plus.cpp`; those tests generate proofs at run
time. The main test pins SHA-256 known-answer digests for deterministic proofs
at every aggregate size. The nested `interop` module pins P2Pool consensus
`v5.0.10` and runs the same 1-through-16 aggregation matrix against an
independent API. It requires byte-for-byte equality for deterministic proofs
and verifies in both directions:

```sh
cd crypto/bulletproofs/interop
go test ./...
```

## Security status

This code has not been independently audited. Passing Monero-derived and
cross-implementation tests is not a substitute for a cryptographic audit.
Do not activate it in consensus until the implementation, proof parser, batch
verifier, commitment-balance equation, and transaction integration have been
reviewed together.
