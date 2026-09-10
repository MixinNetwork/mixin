# Monero range-proof fixtures

`monero_range_proofs.json` contains static Bulletproofs+ fixtures from Monero
`v0.18.5.1`, commit
[`4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5`](https://github.com/monero-project/monero/tree/4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5).
The reference implementation generates the proofs and verifies each expected
result. The Go tests use the checked-in fixtures without a C++ build or network
access.

| Case | Amounts | Expected verification |
| --- | --- | --- |
| `valid_zero` | `0` | Accept |
| `valid_max` | `2^64 - 1` | Accept |
| `invalid_8` | `2^64` | Reject |
| `invalid_31` | `2^248` | Reject |
| `invalid_8_padded` | `7`, `2^64`, `2^64 - 1` | Reject |
| `invalid_31_maximum` | `1` through `15`, followed by `2^248` | Reject |

The two single-amount negative cases use the amounts from Monero's
[`invalid_8` and `invalid_31` unit tests](https://github.com/monero-project/monero/blob/4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5/tests/unit_tests/bulletproofs_plus.cpp).
The three-amount aggregate requires padding to four amounts; the sixteen-amount
aggregate reaches the protocol limit.

Generation uses the unmodified scalar-input `rct::bulletproof_plus_PROVE`
overload in
[`bulletproofs_plus.cc`](https://github.com/monero-project/monero/blob/4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5/src/ringct/bulletproofs_plus.cc).
The full-width values are canonical scalars even when they exceed the supported
64-bit amount range. Blinding scalars are the public test values `17 + i`, where
`i` is the zero-based commitment index. Proof randomness comes from the reference
implementation's default generator, so regeneration produces different proof
bytes. The reference verifier checks every proof with
`rct::bulletproof_plus_VERIFY` before serialization.

All binary fields are hexadecimal. `values` and `blindings` contain 32-byte
little-endian scalars. `commitments` contain ordinary compressed points
`C = blind*G + value*H`, computed as `8 * proof.V[i]` and checked against the
full scalar amount. `proof` is the result of Monero's `serialization::dump_binary`
for `BulletproofPlus`: `A`, `A1`, `B`, `r1`, `s1`, `d1`, then the varint length
and elements of `L`, followed by the varint length and elements of `R`.
Commitments are external to this proof body.

The main-module tests check canonical encodings, reconstruct commitments using
the entire scalar amount, and require successful structural and transcript
processing before testing the final verification equation. Both the main and
P2Pool interoperability tests cover individual proofs, single-item batches,
and mixed batches with an invalid proof first, middle, or last. Valid boundary
proofs provide positive controls.
